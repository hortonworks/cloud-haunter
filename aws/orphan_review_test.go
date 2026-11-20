package aws

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	elb "github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fail only the unrelated orphan's reads, not the healthy native group's reads.
// Target-group ARNs retain the LB identity so health failures are selective too.
type orphanReviewELBFake struct {
	orphanELBFake
	failedARN                         string
	tagError, groupError, healthError error
}

func (f orphanReviewELBFake) DescribeTags(in *elb.DescribeTagsInput) (*elb.DescribeTagsOutput, error) {
	for _, arn := range in.ResourceArns {
		if aws.StringValue(arn) == f.failedARN && f.tagError != nil {
			f.operationChannel <- "DescribeTags"
			return nil, f.tagError
		}
	}
	return f.orphanELBFake.DescribeTags(in)
}

func (f orphanReviewELBFake) DescribeTargetGroups(in *elb.DescribeTargetGroupsInput) (*elb.DescribeTargetGroupsOutput, error) {
	f.operationChannel <- "DescribeTargetGroups"
	arn := aws.StringValue(in.LoadBalancerArn)
	if arn == f.failedARN && f.groupError != nil {
		return nil, f.groupError
	}
	return &elb.DescribeTargetGroupsOutput{TargetGroups: []*elb.TargetGroup{{TargetGroupArn: aws.String("tg:" + arn)}}}, nil
}

func (f orphanReviewELBFake) DescribeTargetHealth(in *elb.DescribeTargetHealthInput) (*elb.DescribeTargetHealthOutput, error) {
	if aws.StringValue(in.TargetGroupArn) == "tg:"+f.failedARN && f.healthError != nil {
		f.operationChannel <- "DescribeTargetHealth"
		return nil, f.healthError
	}
	return f.orphanELBFake.DescribeTargetHealth(in)
}

func orphanReviewReadOnlyOperations(t *testing.T, ch chan string) []string {
	t.Helper()
	ops := closeAndCollectOperations(ch)
	for _, op := range ops {
		for _, prefix := range []string{"Terminate", "WaitUntil", "Modify", "Delete", "Release", "Detach"} {
			assert.False(t, strings.HasPrefix(op, prefix), "unexpected mutation: %s", op)
		}
	}
	return ops
}

func orphanReviewOperationCount(ops []string, operation string) int {
	count := 0
	for _, op := range ops {
		if op == operation {
			count++
		}
	}
	return count
}

func TestOrphanReviewDiscoveryReadFailuresPreserveHealthyNative(t *testing.T) {
	readErr := errors.New("unrelated orphan read denied")
	for _, where := range []string{"target_groups", "target_health", "endpoint_services", "tags", "all_tags"} {
		t.Run(where, func(t *testing.T) {
			f := newOrphanFixture()
			const healthyGroup = "healthy-native"
			i := f.fullInstance(t, 16)
			i.Tags[0].Value = aws.String(healthyGroup)
			f.ec2.instances = []*ec2.Instance{i}
			healthyLB := *f.elb.lbs[0]
			healthyLB.LoadBalancerArn = aws.String("healthy-lb")
			healthyLB.LoadBalancerName = aws.String("healthy-lb")
			f.elb.lbs = append(f.elb.lbs, &healthyLB)
			f.elb.tags["healthy-lb"] = types.Tags{f.cfg.ResourceGroupingLabel: healthyGroup, f.cfg.OwnerLabel: "cloudbreak-qe"}
			client := orphanReviewELBFake{orphanELBFake: f.elb, failedARN: "lb-1"}
			switch where {
			case "target_groups":
				client.groupError = readErr
			case "target_health":
				client.healthError = readErr
			case "endpoint_services":
				f.ec2.endpointsErr = readErr
			case "tags":
				client.tagError = readErr
			case "all_tags":
				client.tagsErr = readErr
			}
			stacks, err := getNativeStacks(f.cfg, types.AWS,
				map[string]ec2Client{orphanTestRegion: f.ec2},
				map[string]elbClient{orphanTestRegion: client},
				map[string]cloudWatchClient{orphanTestRegion: mockCwClient{operationChannel: f.ch}}, nil,
				map[string]cfClient{orphanTestRegion: f.cf})
			if where == "tags" || where == "all_tags" {
				require.ErrorIs(t, err, readErr)
				require.Empty(t, stacks, "tag failures cannot emit incomplete Native LB metadata")
				assertNoMutations(t, f.ch)
				return
			}
			require.NoError(t, err)
			require.Len(t, stacks, 1, "failed orphan-only checks must skip candidates, not the healthy native stack")
			assert.Equal(t, healthyGroup, stacks[0].ID)
			assert.Equal(t, TYPE_NATIVE, stacks[0].Metadata[METADATA_TYPE])
			assert.Equal(t, types.Running, stacks[0].State)
			assert.Equal(t, "ID", stacks[0].Metadata[METADATA_INSTANCES])
			assert.Equal(t, "vol-2", stacks[0].Metadata[METADATA_VOLUMES])
			assert.Equal(t, "healthy-lb", stacks[0].Metadata[METADATA_LOAD_BALANCERS])
			assertNoMutations(t, f.ch)
		})
	}
}

func TestNativeTagReadFailuresNeverEmitPartialCleanup(t *testing.T) {
	readErr := errors.New("LB tags throttled")
	for _, state := range []int64{16, 48} {
		for _, kind := range []string{"network", "application", "ownerless"} {
			t.Run(fmt.Sprintf("state=%d/%s", state, kind), func(t *testing.T) {
				f := newOrphanFixture()
				f.ec2.instances = []*ec2.Instance{f.fullInstance(t, state)}
				if kind == "ownerless" {
					delete(f.elb.tags["lb-1"], f.cfg.OwnerLabel)
					delete(f.elb.tags["lb-2"], f.cfg.OwnerLabel)
				} else {
					f.elb.lbs[0].Type = aws.String(kind)
					f.elb.lbs[1].Type = aws.String(kind)
				}
				client := orphanReviewELBFake{orphanELBFake: f.elb, failedARN: "lb-1", tagError: readErr}
				stacks, err := getNativeStacks(f.cfg, types.AWS,
					map[string]ec2Client{orphanTestRegion: f.ec2},
					map[string]elbClient{orphanTestRegion: client},
					map[string]cloudWatchClient{orphanTestRegion: mockCwClient{operationChannel: f.ch}}, nil,
					map[string]cfClient{orphanTestRegion: f.cf})
				assert.ErrorIs(t, err, readErr)
				assert.Empty(t, stacks, "unknown LB ownership cannot yield a Native stack containing only the readable LB")
				assertNoMutations(t, f.ch)
			})
		}
	}
}

func TestOrphanReviewRejectedTerminatedGroupsKeepLegacyLoadBalancers(t *testing.T) {
	cases := []struct {
		name   string
		modify func(*orphanFixture)
	}{
		{"ALB", func(f *orphanFixture) { f.elb.lbs[0].Type = aws.String("application") }},
		{"ownerless", func(f *orphanFixture) { delete(f.elb.tags["lb-1"], f.cfg.OwnerLabel) }},
		{"registered_targets", func(f *orphanFixture) {
			f.elb.targets = []*elb.TargetHealthDescription{{Target: &elb.TargetDescription{Id: aws.String("i-live")}}}
		}},
		{"target_read_denied", func(f *orphanFixture) { f.elb.groupsErr = errors.New("target groups denied") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newOrphanFixture()
			f.ec2.instances = []*ec2.Instance{f.fullInstance(t, 48)}
			tc.modify(f)
			stacks, err := f.discover(nil)
			require.NoError(t, err)
			require.Len(t, stacks, 1)
			assert.Equal(t, TYPE_NATIVE, stacks[0].Metadata[METADATA_TYPE])
			assert.Equal(t, types.Terminated, stacks[0].State)
			assert.Equal(t, "lb-1,lb-2", stacks[0].Metadata[METADATA_LOAD_BALANCERS])
			assert.Equal(t, "ID", stacks[0].Metadata[METADATA_INSTANCES])
			assert.Equal(t, "vol-2", stacks[0].Metadata[METADATA_VOLUMES])
			assertNoMutations(t, f.ch)
		})
	}
	t.Run("accepted_orphans_remain_separate", func(t *testing.T) {
		f := newOrphanFixture()
		f.ec2.instances = []*ec2.Instance{f.fullInstance(t, 48)}
		stacks, err := f.discover(nil)
		require.NoError(t, err)
		require.Len(t, stacks, 2)
		for _, stack := range stacks {
			switch stack.Metadata[METADATA_TYPE] {
			case TYPE_NATIVE:
				assert.Equal(t, types.Terminated, stack.State)
				assert.Empty(t, stack.Metadata[METADATA_LOAD_BALANCERS])
				assert.Equal(t, "vol-2", stack.Metadata[METADATA_VOLUMES])
			case TYPE_NATIVE_LB:
				assert.Equal(t, types.Unused, stack.State)
				assert.Equal(t, "lb-1,lb-2", stack.Metadata[METADATA_LOAD_BALANCERS])
				for _, key := range []string{METADATA_INSTANCES, METADATA_VOLUMES, METADATA_SECURITY_GROUPS, METADATA_ELASTIC_IPS, METADATA_ALARMS} {
					assert.Empty(t, stack.Metadata[key])
				}
			default:
				t.Errorf("unexpected stack type: %s", stack.Metadata[METADATA_TYPE])
			}
		}
		assertNoMutations(t, f.ch)
	})
}

func TestOrphanReviewDiscoveryCloudFormationPhysicalReferenceExcludesOrphan(t *testing.T) {
	for _, physicalID := range []string{"lb-2", "orphan-lb-2-name"} {
		t.Run(physicalID, func(t *testing.T) {
			f := newOrphanFixture()
			f.elb.lbs[1].LoadBalancerName = aws.String("orphan-lb-2-name")
			f.cf.stacks = []*cloudformation.Stack{{StackId: aws.String("unrelated-cf-stack"), StackName: aws.String("unrelated-cf-stack"), CreationTime: &NOW, StackStatus: aws.String("CREATE_COMPLETE")}}
			f.cf.resources = []*cloudformation.StackResourceSummary{{PhysicalResourceId: aws.String(physicalID)}}
			cfStacks, err := getCFStacks(f.cfg, types.AWS, map[string]cfClient{orphanTestRegion: f.cf})
			require.NoError(t, err)
			stacks, err := f.discover(cfStacks)
			require.NoError(t, err)
			if physicalID == "lb-2" {
				assert.Empty(t, stacks, "an ELBv2 ARN reference protects the whole orphan group")
			} else {
				assert.Len(t, stacks, 1, "a coincident resource name is not ELBv2 physical ownership; its physical ID is the ARN")
			}
			ops := orphanReviewReadOnlyOperations(t, f.ch)
			assert.Equal(t, 1, orphanReviewOperationCount(ops, "ListStackResources"))
		})
	}
}

func TestOrphanReviewDryRunRevalidatesAndSurfacesReadErrors(t *testing.T) {
	readErr := errors.New("fresh orphan read denied")
	cases := []struct {
		name   string
		modify func(*orphanFixture)
	}{
		{"eligible", nil},
		{"LB_read_denied", func(f *orphanFixture) { f.elb.listErr = readErr }},
		{"tags_denied", func(f *orphanFixture) { f.elb.tagsErr = readErr }},
		{"instances_denied", func(f *orphanFixture) { f.ec2.instancesErr = readErr }},
		{"CF_inventory_denied", func(f *orphanFixture) { f.cf.err = readErr }},
		{"CF_resources_denied", func(f *orphanFixture) { f.cf.resourcesErr = readErr }},
		{"endpoints_denied", func(f *orphanFixture) { f.ec2.endpointsErr = readErr }},
		{"target_groups_denied", func(f *orphanFixture) { f.elb.groupsErr = readErr }},
		{"target_health_denied", func(f *orphanFixture) { f.elb.healthErr = readErr }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newOrphanFixture()
			f.cfg.DryRun = true
			f.cf.stacks = []*cloudformation.Stack{{StackId: aws.String("unrelated-cf-stack")}}
			if tc.modify != nil {
				tc.modify(f)
			}
			errs := f.delete()
			if tc.modify == nil {
				require.Empty(t, errs)
			} else {
				require.ErrorIs(t, errors.Join(errs...), readErr)
			}
			ops := orphanReviewReadOnlyOperations(t, f.ch)
			if tc.modify == nil {
				for _, op := range []string{"DescribeLoadBalancers", "DescribeTags", "DescribeInstances", "DescribeStacks", "ListStackResources", "DescribeEndpointServices", "DescribeTargetGroups", "DescribeTargetHealth"} {
					assert.Contains(t, ops, op, "dry-run must perform fresh read-only validation")
				}
			}
		})
	}
}

func TestOrphanReviewActionEligibilityChangesWarnAndSkip(t *testing.T) {
	cases := []struct {
		name   string
		modify func(*orphanFixture)
	}{
		{"owner_changed", func(f *orphanFixture) { f.elb.tags["lb-1"][f.cfg.OwnerLabel] = "new-owner" }},
		{"group_changed", func(f *orphanFixture) { f.elb.tags["lb-1"][f.cfg.ResourceGroupingLabel] = "other-group" }},
		{"young_LB", func(f *orphanFixture) { f.elb.lbs[1].CreatedTime = aws.Time(time.Now()) }},
		{"target_added", func(f *orphanFixture) {
			f.elb.targets = []*elb.TargetHealthDescription{{Target: &elb.TargetDescription{Id: aws.String("i-live")}}}
		}},
		{"ignore_added", func(f *orphanFixture) { f.elb.tags["lb-2"][f.cfg.IgnoreLabel] = "true" }},
		{"type_changed", func(f *orphanFixture) { f.elb.lbs[0].Type = aws.String("application") }},
		{"compute_added", func(f *orphanFixture) { f.ec2.instances = []*ec2.Instance{backingInstance(f.cfg, 16, false, false)} }},
		{"endpoint_added", func(f *orphanFixture) {
			f.ec2.endpoints = []*ec2.ServiceConfiguration{{NetworkLoadBalancerArns: aws.StringSlice([]string{"lb-1"})}}
		}},
		{"CF_owner_added", func(f *orphanFixture) {
			f.cf.stacks = []*cloudformation.Stack{{StackId: aws.String("new-cf-stack"), Tags: []*cloudformation.Tag{{Key: aws.String(f.cfg.ResourceGroupingLabel), Value: aws.String(orphanTestGroup)}}}}
		}},
		{"CF_physical_reference_added", func(f *orphanFixture) {
			f.cf.stacks = []*cloudformation.Stack{{StackId: aws.String("unrelated-cf-stack")}}
			f.cf.resources = []*cloudformation.StackResourceSummary{{PhysicalResourceId: aws.String("lb-2")}}
		}},
	}
	for _, dryRun := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/dry_run=%t", tc.name, dryRun), func(t *testing.T) {
				f := newOrphanFixture()
				f.cfg.DryRun = dryRun
				tc.modify(f)
				assert.Empty(t, f.delete(), "ordinary eligibility changes are skips, not action failures")
				assertNoMutations(t, f.ch)
			})
		}
	}
}

func TestOrphanReviewMalformedActionMetadataStillErrors(t *testing.T) {
	cases := []struct {
		name   string
		modify func(*orphanFixture)
	}{
		{"non_LB_metadata", func(f *orphanFixture) { f.stack.Metadata[METADATA_VOLUMES] = "vol-active" }},
		{"missing_grouping_config", func(f *orphanFixture) { f.cfg.ResourceGroupingLabel = "" }},
		{"missing_owner_config", func(f *orphanFixture) { f.cfg.OwnerLabel = "" }},
		{"missing_owner_identity", func(f *orphanFixture) { f.stack.Owner = "" }},
	}
	for _, dryRun := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/dry_run=%t", tc.name, dryRun), func(t *testing.T) {
				f := newOrphanFixture()
				f.cfg.DryRun = dryRun
				tc.modify(f)
				assert.NotEmpty(t, f.delete())
				assertNoMutations(t, f.ch)
			})
		}
	}
}

// Old per-group scans stay below this capacity too, so the scaling regression
// fails on call counts rather than deadlocking while recording fake operations.
func orphanReviewScalingFixture() (*orphanFixture, []*types.Stack) {
	f := newOrphanFixture()
	f.ch = make(chan string, 1000)
	f.ec2.operationChannel = f.ch
	f.elb.operationChannel = f.ch
	f.cf.operationChannel = f.ch
	baseLB := *f.elb.lbs[0]
	f.elb.lbs = nil
	f.elb.tags = map[string]types.Tags{}
	var stacks []*types.Stack
	for groupIndex := 0; groupIndex < 5; groupIndex++ {
		group := fmt.Sprintf("orphan-group-%d", groupIndex)
		var arns []string
		for lbIndex := 0; lbIndex < 2; lbIndex++ {
			arn := fmt.Sprintf("lb-%d-%d", groupIndex, lbIndex)
			lb := baseLB
			lb.LoadBalancerArn = aws.String(arn)
			lb.LoadBalancerName = aws.String(arn)
			f.elb.lbs = append(f.elb.lbs, &lb)
			f.elb.tags[arn] = types.Tags{f.cfg.ResourceGroupingLabel: group, f.cfg.OwnerLabel: "cloudbreak-qe"}
			arns = append(arns, arn)
		}
		stacks = append(stacks, &types.Stack{ID: group, Name: group, Region: orphanTestRegion, Owner: "cloudbreak-qe", State: types.Unused,
			Metadata: map[string]string{METADATA_TYPE: TYPE_NATIVE_LB, METADATA_LOAD_BALANCERS: strings.Join(arns, ",")}})
	}
	for n := 0; n < 20; n++ {
		f.cf.stacks = append(f.cf.stacks, &cloudformation.Stack{StackId: aws.String(fmt.Sprintf("unrelated-cf-%d", n))})
	}
	return f, stacks
}

func TestOrphanReviewActionSharesRegionalInventories(t *testing.T) {
	f, stacks := orphanReviewScalingFixture()
	errs := deleteStacks(f.cfg,
		map[string]cfClient{orphanTestRegion: f.cf}, nil,
		map[string]ec2Client{orphanTestRegion: f.ec2},
		map[string]elbClient{orphanTestRegion: f.elb},
		map[string]cloudWatchClient{orphanTestRegion: mockCwClient{operationChannel: f.ch}}, stacks)
	require.Empty(t, errs)
	ops := closeAndCollectOperations(f.ch)
	assert.Equal(t, 1, orphanReviewOperationCount(ops, "DescribeStacks"))
	assert.Equal(t, 20, orphanReviewOperationCount(ops, "ListStackResources"), "CF references must be indexed once, not scanned for each of five groups")
	assert.Equal(t, 1, orphanReviewOperationCount(ops, "DescribeEndpointServices"))
	var mutations, expected []string
	for _, op := range ops {
		for _, prefix := range []string{"Terminate", "WaitUntil", "Modify", "Delete", "Release", "Detach"} {
			if strings.HasPrefix(op, prefix) {
				mutations = append(mutations, op)
				break
			}
		}
	}
	for _, lb := range f.elb.lbs {
		arn := aws.StringValue(lb.LoadBalancerArn)
		expected = append(expected, "ModifyLoadBalancerAttributes:"+arn, "DeleteLoadBalancer:"+arn)
	}
	assert.ElementsMatch(t, expected, mutations, "exactly ten LBs are deleted, with no non-LB mutations")
}

func TestOrphanReviewDiscoverySharesRegionalInventories(t *testing.T) {
	f, _ := orphanReviewScalingFixture()
	// Discovery may materialize CF stacks; unlike action, it needs full fields.
	for _, stack := range f.cf.stacks {
		stack.StackName = stack.StackId
		stack.CreationTime = &NOW
		stack.StackStatus = aws.String("CREATE_COMPLETE")
	}
	cfStacks, err := getCFStacks(f.cfg, types.AWS, map[string]cfClient{orphanTestRegion: f.cf})
	require.NoError(t, err)
	stacks, err := f.discover(cfStacks)
	require.NoError(t, err)
	require.Len(t, stacks, 5)
	for _, stack := range stacks {
		assert.Equal(t, TYPE_NATIVE_LB, stack.Metadata[METADATA_TYPE])
		assert.Len(t, getResourceList(stack.Metadata[METADATA_LOAD_BALANCERS]), 2)
	}
	ops := orphanReviewReadOnlyOperations(t, f.ch)
	assert.Equal(t, 1, orphanReviewOperationCount(ops, "DescribeStacks"))
	assert.Equal(t, 20, orphanReviewOperationCount(ops, "ListStackResources"))
	assert.Equal(t, 1, orphanReviewOperationCount(ops, "DescribeEndpointServices"))
}
