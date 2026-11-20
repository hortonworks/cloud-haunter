package aws

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	elb "github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/filter"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const orphanTestRegion = "us-west-2"
const orphanTestGroup = "qe-orphan-environment"

type orphanEC2Fake struct {
	mockEc2Client
	instances    []*ec2.Instance
	instancesErr error
	addressesErr error
	groupsErr    error
	endpoints    []*ec2.ServiceConfiguration
	endpointsErr error
	pages        bool
}

func (f orphanEC2Fake) DescribeAddresses(in *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error) {
	if f.addressesErr != nil {
		return nil, f.addressesErr
	}
	output, err := f.mockEc2Client.DescribeAddresses(in)
	for _, address := range output.Addresses {
		address.PublicIp = aws.String("192.0.2.1") // Complete the legacy fixture for orphan warnings.
	}
	return output, err
}

func (f orphanEC2Fake) DescribeSecurityGroups(in *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error) {
	if f.groupsErr != nil {
		return nil, f.groupsErr
	}
	return f.mockEc2Client.DescribeSecurityGroups(in)
}

func (f orphanEC2Fake) DescribeInstances(in *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	f.operationChannel <- "DescribeInstances"
	if f.instancesErr != nil {
		return nil, f.instancesErr
	}
	if f.pages && in.NextToken == nil {
		return &ec2.DescribeInstancesOutput{NextToken: aws.String("next")}, nil
	}
	return &ec2.DescribeInstancesOutput{Reservations: []*ec2.Reservation{{Instances: f.instances}}}, nil
}
func (f orphanEC2Fake) DescribeVpcEndpointServiceConfigurations(in *ec2.DescribeVpcEndpointServiceConfigurationsInput) (*ec2.DescribeVpcEndpointServiceConfigurationsOutput, error) {
	f.operationChannel <- "DescribeEndpointServices"
	if f.endpointsErr != nil {
		return nil, f.endpointsErr
	}
	if f.pages && in.NextToken == nil {
		return &ec2.DescribeVpcEndpointServiceConfigurationsOutput{NextToken: aws.String("next")}, nil
	}
	return &ec2.DescribeVpcEndpointServiceConfigurationsOutput{ServiceConfigurations: f.endpoints}, nil
}

type orphanELBFake struct {
	mockElbClient
	lbs                                    []*elb.LoadBalancer
	tags                                   map[string]types.Tags
	targets                                []*elb.TargetHealthDescription
	tagsErr, groupsErr, healthErr, listErr error
	pages                                  bool
}

func (f orphanELBFake) DescribeLoadBalancers(in *elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error) {
	f.operationChannel <- "DescribeLoadBalancers"
	if f.listErr != nil {
		return nil, f.listErr
	}
	if len(in.LoadBalancerArns) > 0 {
		for _, lb := range f.lbs {
			if aws.StringValue(lb.LoadBalancerArn) == aws.StringValue(in.LoadBalancerArns[0]) {
				return &elb.DescribeLoadBalancersOutput{LoadBalancers: []*elb.LoadBalancer{lb}}, nil
			}
		}
		return nil, awserr.New("LoadBalancerNotFound", "already gone", nil)
	}
	if f.pages && in.Marker == nil {
		return &elb.DescribeLoadBalancersOutput{NextMarker: aws.String("next")}, nil
	}
	return &elb.DescribeLoadBalancersOutput{LoadBalancers: f.lbs}, nil
}
func (f orphanELBFake) DescribeTags(in *elb.DescribeTagsInput) (*elb.DescribeTagsOutput, error) {
	f.operationChannel <- "DescribeTags"
	if f.tagsErr != nil {
		return nil, f.tagsErr
	}
	output := &elb.DescribeTagsOutput{}
	for _, arn := range in.ResourceArns {
		tags := []*elb.Tag{}
		for k, v := range f.tags[aws.StringValue(arn)] {
			tags = append(tags, &elb.Tag{Key: aws.String(k), Value: aws.String(v)})
		}
		output.TagDescriptions = append(output.TagDescriptions, &elb.TagDescription{ResourceArn: arn, Tags: tags})
	}
	return output, nil
}
func (f orphanELBFake) DescribeTargetGroups(in *elb.DescribeTargetGroupsInput) (*elb.DescribeTargetGroupsOutput, error) {
	f.operationChannel <- "DescribeTargetGroups"
	if f.groupsErr != nil {
		return nil, f.groupsErr
	}
	if f.pages && in.Marker == nil {
		return &elb.DescribeTargetGroupsOutput{NextMarker: aws.String("next")}, nil
	}
	return &elb.DescribeTargetGroupsOutput{TargetGroups: []*elb.TargetGroup{{TargetGroupArn: aws.String("tg")}}}, nil
}
func (f orphanELBFake) DescribeTargetHealth(*elb.DescribeTargetHealthInput) (*elb.DescribeTargetHealthOutput, error) {
	f.operationChannel <- "DescribeTargetHealth"
	if f.healthErr != nil {
		return nil, f.healthErr
	}
	return &elb.DescribeTargetHealthOutput{TargetHealthDescriptions: f.targets}, nil
}

type orphanCFFake struct {
	mockCfClient
	stacks       []*cloudformation.Stack
	resources    []*cloudformation.StackResourceSummary
	err          error
	resourcesErr error
	pages        bool
}

func (f orphanCFFake) DescribeStacks(in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	f.operationChannel <- "DescribeStacks"
	if f.err != nil {
		return nil, f.err
	}
	if f.pages && in.NextToken == nil {
		return &cloudformation.DescribeStacksOutput{NextToken: aws.String("next")}, nil
	}
	return &cloudformation.DescribeStacksOutput{Stacks: f.stacks}, nil
}

func (f orphanCFFake) ListStackResources(in *cloudformation.ListStackResourcesInput) (*cloudformation.ListStackResourcesOutput, error) {
	f.operationChannel <- "ListStackResources"
	if f.resourcesErr != nil {
		return nil, f.resourcesErr
	}
	if f.pages && in.NextToken == nil {
		return &cloudformation.ListStackResourcesOutput{NextToken: aws.String("next")}, nil
	}
	return &cloudformation.ListStackResourcesOutput{StackResourceSummaries: f.resources}, nil
}

type orphanFixture struct {
	cfg   *config.Config
	ch    chan string
	ec2   orphanEC2Fake
	elb   orphanELBFake
	cf    orphanCFFake
	stack *types.Stack
}

func newOrphanFixture() *orphanFixture {
	ch := make(chan string, 200)
	cfg := testCfg()
	cfg.IgnoreLabel = config.DefaultIgnoreLabel
	cfg.LongRunningPeriod = 6 * time.Hour
	old := time.Now().Add(-48 * time.Hour)
	newer := old.Add(time.Hour)
	lbs := []*elb.LoadBalancer{}
	tags := map[string]types.Tags{}
	for n, arn := range []string{"lb-1", "lb-2"} {
		created := old
		if n == 1 {
			created = newer
		}
		lbs = append(lbs, &elb.LoadBalancer{LoadBalancerArn: aws.String(arn), LoadBalancerName: aws.String(arn), Type: aws.String("network"), CreatedTime: &created, State: &elb.LoadBalancerState{Code: aws.String("active")}})
		tags[arn] = types.Tags{cfg.ResourceGroupingLabel: orphanTestGroup, cfg.OwnerLabel: "cloudbreak-qe", cloudbreakResourceTag: "freeipa-resource"}
	}
	return &orphanFixture{cfg: cfg, ch: ch,
		ec2:   orphanEC2Fake{mockEc2Client: mockEc2Client{operationChannel: ch}},
		elb:   orphanELBFake{mockElbClient: mockElbClient{operationChannel: ch}, lbs: lbs, tags: tags},
		cf:    orphanCFFake{mockCfClient: mockCfClient{operationChannel: ch}},
		stack: &types.Stack{ID: orphanTestGroup, Name: orphanTestGroup, Region: orphanTestRegion, Owner: "cloudbreak-qe", State: types.Unused, Metadata: map[string]string{METADATA_TYPE: TYPE_NATIVE_LB, METADATA_LOAD_BALANCERS: "lb-1,lb-2"}},
	}
}
func (f *orphanFixture) discover(cfStacks []*types.Stack) ([]*types.Stack, error) {
	return getNativeStacks(f.cfg, types.AWS, map[string]ec2Client{orphanTestRegion: f.ec2}, map[string]elbClient{orphanTestRegion: f.elb}, map[string]cloudWatchClient{orphanTestRegion: mockCwClient{operationChannel: f.ch}}, cfStacks, map[string]cfClient{orphanTestRegion: f.cf})
}
func (f *orphanFixture) delete() []error {
	return deleteStacks(f.cfg, map[string]cfClient{orphanTestRegion: f.cf}, nil, map[string]ec2Client{orphanTestRegion: f.ec2}, map[string]elbClient{orphanTestRegion: f.elb}, map[string]cloudWatchClient{orphanTestRegion: mockCwClient{operationChannel: f.ch}}, []*types.Stack{f.stack})
}
func backingInstance(cfg *config.Config, state int64, cfOwned bool, byResource bool) *ec2.Instance {
	tags := []*ec2.Tag{}
	if byResource {
		tags = append(tags, &ec2.Tag{Key: aws.String(cloudbreakResourceTag), Value: aws.String("freeipa-resource")})
	} else {
		tags = append(tags, &ec2.Tag{Key: aws.String(cfg.ResourceGroupingLabel), Value: aws.String(orphanTestGroup)})
	}
	if cfOwned {
		tags = append(tags, &ec2.Tag{Key: aws.String(CF_TAG), Value: aws.String("cf-id")})
	}
	return &ec2.Instance{Tags: tags, State: &ec2.InstanceState{Code: aws.Int64(state)}}
}
func assertNoMutations(t *testing.T, ch chan string) {
	t.Helper()
	for _, op := range closeAndCollectOperations(ch) {
		for _, prefix := range []string{"Terminate", "WaitUntil", "Modify", "Delete", "Release", "Detach"} {
			assert.False(t, strings.HasPrefix(op, prefix), "unexpected mutation: %s", op)
		}
	}
}

func (f *orphanFixture) fullInstance(t *testing.T, state int64) *ec2.Instance {
	t.Helper()
	i := newTestInstance()
	i.State.Code = aws.Int64(state)
	i.Placement.AvailabilityZone = aws.String(orphanTestRegion + "a")
	i.Tags = []*ec2.Tag{
		{Key: aws.String(f.cfg.ResourceGroupingLabel), Value: aws.String(orphanTestGroup)},
		{Key: aws.String(f.cfg.OwnerLabel), Value: aws.String("cloudbreak-qe")},
	}
	return i
}

type failingOrphanCW struct {
	mockCwClient
	err error
}

func (f failingOrphanCW) DescribeAlarms(*cloudwatch.DescribeAlarmsInput) (*cloudwatch.DescribeAlarmsOutput, error) {
	f.operationChannel <- "DescribeAlarms"
	return nil, f.err
}

func TestGetNativeStacksReconcilesOrphanLoadBalancers(t *testing.T) {
	for _, terminated := range []bool{false, true} {
		t.Run(map[bool]string{false: "no_instances", true: "lingering_terminated_record"}[terminated], func(t *testing.T) {
			f := newOrphanFixture()
			if terminated {
				f.ec2.instances = []*ec2.Instance{f.fullInstance(t, 48)}
			}
			stacks, err := f.discover(nil)
			require.NoError(t, err)
			var s *types.Stack
			for _, candidate := range stacks {
				if candidate.Metadata[METADATA_TYPE] == TYPE_NATIVE_LB {
					s = candidate
				} else {
					assert.Equal(t, types.Terminated, candidate.State)
					assert.Empty(t, candidate.Metadata[METADATA_LOAD_BALANCERS])
					assert.Equal(t, "vol-2", candidate.Metadata[METADATA_VOLUMES])
				}
			}
			require.NotNil(t, s)
			if terminated {
				assert.Len(t, stacks, 2) // Preserve legacy non-LB teardown metadata.
			} else {
				assert.Len(t, stacks, 1)
			}
			assert.Equal(t, TYPE_NATIVE_LB, s.Metadata[METADATA_TYPE])
			assert.Equal(t, types.Unused, s.State)
			assert.Equal(t, orphanTestGroup, s.ID)
			assert.Equal(t, "cloudbreak-qe", s.Owner)
			assert.Equal(t, *f.elb.lbs[1].CreatedTime, s.Created)
			assert.Equal(t, "lb-1,lb-2", s.Metadata[METADATA_LOAD_BALANCERS])
			for _, key := range []string{METADATA_INSTANCES, METADATA_VOLUMES, METADATA_SECURITY_GROUPS, METADATA_ELASTIC_IPS, METADATA_ALARMS} {
				assert.Empty(t, s.Metadata[key])
			}
			filtered, err := filter.NewLongRunning(f.cfg).Execute([]types.CloudItem{s})
			require.NoError(t, err)
			assert.Len(t, filtered, 1)
			f.stack = s
			require.Empty(t, f.delete())
			mutations := []string{}
			for _, op := range closeAndCollectOperations(f.ch) {
				if strings.HasPrefix(op, "Modify") || strings.HasPrefix(op, "Delete") || strings.HasPrefix(op, "Terminate") || strings.HasPrefix(op, "Release") {
					mutations = append(mutations, op)
				}
			}
			assert.ElementsMatch(t, []string{"ModifyLoadBalancerAttributes:lb-1", "DeleteLoadBalancer:lb-1", "ModifyLoadBalancerAttributes:lb-2", "DeleteLoadBalancer:lb-2"}, mutations)
		})
	}
}

func TestNativeGroupsPreserveTerminatedRecordBehavior(t *testing.T) {
	t.Run("mixed_group_keeps_original_state_and_age", func(t *testing.T) {
		f := newOrphanFixture()
		running := f.fullInstance(t, 16)
		terminated := f.fullInstance(t, 48)
		old := time.Now().Add(-72 * time.Hour)
		terminated.LaunchTime = &old
		f.ec2.instances = []*ec2.Instance{running, terminated}
		stacks, err := f.discover(nil)
		require.NoError(t, err)
		require.Len(t, stacks, 1)
		assert.Equal(t, TYPE_NATIVE, stacks[0].Metadata[METADATA_TYPE])
		assert.Equal(t, types.Unknown, stacks[0].State)
		assert.Equal(t, old, stacks[0].Created)
		assert.Equal(t, "lb-1,lb-2", stacks[0].Metadata[METADATA_LOAD_BALANCERS])
		selected, err := filter.NewLongRunning(f.cfg).Execute([]types.CloudItem{stacks[0]})
		require.NoError(t, err)
		assert.Empty(t, selected)
		assertNoMutations(t, f.ch)
	})
	t.Run("terminated_without_LBs_keeps_legacy_metadata", func(t *testing.T) {
		f := newOrphanFixture()
		f.elb.lbs = nil
		f.ec2.instances = []*ec2.Instance{f.fullInstance(t, 48)}
		stacks, err := f.discover(nil)
		require.NoError(t, err)
		require.Len(t, stacks, 1)
		assert.Equal(t, TYPE_NATIVE, stacks[0].Metadata[METADATA_TYPE])
		assert.Equal(t, types.Terminated, stacks[0].State)
		assert.Equal(t, "vol-2", stacks[0].Metadata[METADATA_VOLUMES])
		assertNoMutations(t, f.ch)
	})
}

func TestOrphanLoadBalancerSafeguards(t *testing.T) {
	cases := []struct {
		name   string
		modify func(*orphanFixture)
		cf     []*types.Stack
	}{
		{name: "CF_tag", modify: func(f *orphanFixture) { f.elb.tags["lb-1"][CF_TAG] = "cf" }},
		{name: "CF_group", cf: []*types.Stack{{Region: orphanTestRegion, Tags: types.Tags{config.DefaultResourceGroupingLabel: orphanTestGroup}}}},
		{name: "ALB", modify: func(f *orphanFixture) { f.elb.lbs[0].Type = aws.String("application") }},
		{name: "GWLB", modify: func(f *orphanFixture) { f.elb.lbs[0].Type = aws.String("gateway") }},
		{name: "untagged", modify: func(f *orphanFixture) { delete(f.elb.tags["lb-1"], f.cfg.ResourceGroupingLabel) }},
		{name: "unowned", modify: func(f *orphanFixture) { delete(f.elb.tags["lb-1"], f.cfg.OwnerLabel) }},
		{name: "mixed_owners", modify: func(f *orphanFixture) { f.elb.tags["lb-1"][f.cfg.OwnerLabel] = "other-user" }},
		{name: "provisioning", modify: func(f *orphanFixture) { f.elb.lbs[0].State.Code = aws.String("provisioning") }},
		{name: "missing_created_time", modify: func(f *orphanFixture) { f.elb.lbs[0].CreatedTime = nil }},
		{name: "stopped_compute", modify: func(f *orphanFixture) { f.ec2.instances = []*ec2.Instance{backingInstance(f.cfg, 80, false, false)} }},
		{name: "CF_compute", modify: func(f *orphanFixture) { f.ec2.instances = []*ec2.Instance{backingInstance(f.cfg, 16, true, false)} }},
		{name: "resource_tag_compute", modify: func(f *orphanFixture) { f.ec2.instances = []*ec2.Instance{backingInstance(f.cfg, 16, false, true)} }},
		{name: "shutting_down_compute", modify: func(f *orphanFixture) { f.ec2.instances = []*ec2.Instance{backingInstance(f.cfg, 32, false, false)} }},
		{name: "endpoint_service", modify: func(f *orphanFixture) {
			f.ec2.pages = true
			f.ec2.endpoints = []*ec2.ServiceConfiguration{{NetworkLoadBalancerArns: aws.StringSlice([]string{"lb-2"})}}
		}},
		{name: "draining_target", modify: func(f *orphanFixture) {
			f.elb.pages = true
			f.elb.targets = []*elb.TargetHealthDescription{{Target: &elb.TargetDescription{Id: aws.String("i-live")}, TargetHealth: &elb.TargetHealth{State: aws.String("draining")}}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newOrphanFixture()
			if tc.modify != nil {
				tc.modify(f)
			}
			s, err := assembleOrphanLoadBalancerStack(f.cfg, orphanTestRegion, orphanTestGroup, f.elb.lbs, f.elb.tags, f.ec2.instances, tc.cf, f.ec2, f.elb)
			assert.NoError(t, err)
			assert.Nil(t, s)
			assertNoMutations(t, f.ch)
		})
	}
}

func TestOrphanGroupTagMatching(t *testing.T) {
	const label = "cleanup-approved"
	ignoreLabel := config.DefaultIgnoreLabel
	cases := []struct {
		name     string
		first    types.Tags
		second   types.Tags
		retained bool
		ignored  bool
	}{
		{name: "single_empty_value", first: types.Tags{label: ""}},
		{name: "shared_empty_value", first: types.Tags{label: ""}, second: types.Tags{label: ""}, retained: true},
		{name: "shared_nonempty_value", first: types.Tags{label: "yes"}, second: types.Tags{label: "yes"}, retained: true},
		{name: "different_values", first: types.Tags{label: "yes"}, second: types.Tags{label: "no"}},
		{name: "single_empty_ignore", first: types.Tags{ignoreLabel: ""}, retained: true, ignored: true},
		{name: "different_ignore_values", first: types.Tags{ignoreLabel: ""}, second: types.Tags{ignoreLabel: "true"}, retained: true, ignored: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, order := range []string{"original", "reversed"} {
				t.Run(order, func(t *testing.T) {
					f := newOrphanFixture()
					for arn, tags := range map[string]types.Tags{"lb-1": tc.first, "lb-2": tc.second} {
						for k, v := range tags {
							f.elb.tags[arn][k] = v
						}
					}
					if order == "reversed" {
						f.elb.lbs[0], f.elb.lbs[1] = f.elb.lbs[1], f.elb.lbs[0]
					}
					key := label
					if tc.ignored {
						key = ignoreLabel
					}
					include := &types.FilterInstanceConfig{}
					include.Aws.Labels = []string{key}
					f.cfg.FilterConfig = &types.FilterConfig{IncludeInstance: include}
					stacks, err := f.discover(nil)
					require.NoError(t, err)
					require.Len(t, stacks, 1)
					if tc.retained {
						assert.Contains(t, stacks[0].Tags, key)
					} else {
						assert.NotContains(t, stacks[0].Tags, key)
					}
					selected, err := filter.NewMatch(f.cfg).Execute([]types.CloudItem{stacks[0]})
					require.NoError(t, err)
					if tc.retained && !tc.ignored {
						assert.Len(t, selected, 1)
					} else {
						assert.Empty(t, selected)
					}
					if tc.ignored {
						f.cfg.IgnoreLabelDisabled = true
						selected, err = filter.NewMatch(f.cfg).Execute([]types.CloudItem{stacks[0]})
						require.NoError(t, err)
						assert.Len(t, selected, 1)
					}
					assertNoMutations(t, f.ch)
				})
			}
		})
	}
}

func TestOrphanGroupIgnoreAndYoungMembers(t *testing.T) {
	f := newOrphanFixture()
	f.elb.tags["lb-2"][f.cfg.IgnoreLabel] = "false"
	stacks, err := f.discover(nil)
	require.NoError(t, err)
	require.Len(t, stacks, 1)
	assert.Contains(t, stacks[0].Tags, f.cfg.IgnoreLabel)
	selected, err := filter.NewLongRunning(f.cfg).Execute([]types.CloudItem{stacks[0]})
	require.NoError(t, err)
	assert.Empty(t, selected)
	assertNoMutations(t, f.ch)
	f = newOrphanFixture()
	recent := time.Now()
	f.elb.lbs[1].CreatedTime = &recent
	stacks, err = f.discover(nil)
	require.NoError(t, err)
	require.Len(t, stacks, 1)
	assert.Equal(t, recent, stacks[0].Created)
	selected, err = filter.NewLongRunning(f.cfg).Execute([]types.CloudItem{stacks[0]})
	require.NoError(t, err)
	assert.Empty(t, selected)
	assertNoMutations(t, f.ch)
}

func TestOrphanLoadBalancerActionRevalidates(t *testing.T) {
	apiErr := errors.New("read denied")
	cases := []struct {
		name   string
		modify func(*orphanFixture)
	}{
		{"owner_changed", func(f *orphanFixture) { f.elb.tags["lb-1"][f.cfg.OwnerLabel] = "other-user" }},
		{"group_changed", func(f *orphanFixture) { f.elb.tags["lb-1"][f.cfg.ResourceGroupingLabel] = "other-env" }},
		{"ignore_added", func(f *orphanFixture) { f.elb.tags["lb-2"][f.cfg.IgnoreLabel] = "true" }},
		{"target_added", func(f *orphanFixture) {
			f.elb.pages = true
			f.elb.targets = []*elb.TargetHealthDescription{{Target: &elb.TargetDescription{Id: aws.String("i-live")}}}
		}},
		{"compute_added", func(f *orphanFixture) {
			f.ec2.pages = true
			f.ec2.instances = []*ec2.Instance{backingInstance(f.cfg, 80, false, true)}
		}},
		{"CF_added", func(f *orphanFixture) {
			f.cf.pages = true
			f.cf.stacks = []*cloudformation.Stack{{StackId: aws.String("owned-cf-stack"), Tags: []*cloudformation.Tag{{Key: aws.String(f.cfg.ResourceGroupingLabel), Value: aws.String(orphanTestGroup)}}}}
		}},
		{"CF_physical_reference_without_tags", func(f *orphanFixture) {
			f.cf.pages = true
			f.cf.stacks = []*cloudformation.Stack{{StackId: aws.String("shared-cf-stack")}}
			f.cf.resources = []*cloudformation.StackResourceSummary{{PhysicalResourceId: aws.String("lb-2")}}
		}},
		{"CF_resources_API_error", func(f *orphanFixture) {
			f.cf.stacks = []*cloudformation.Stack{{StackId: aws.String("shared-cf-stack")}}
			f.cf.resourcesErr = apiErr
		}},
		{"endpoint_added", func(f *orphanFixture) {
			f.ec2.pages = true
			f.ec2.endpoints = []*ec2.ServiceConfiguration{{NetworkLoadBalancerArns: aws.StringSlice([]string{"lb-1"})}}
		}},
		{"young_LB", func(f *orphanFixture) { recent := time.Now(); f.elb.lbs[0].CreatedTime = &recent }},
		{"extra_non_LB_metadata", func(f *orphanFixture) { f.stack.Metadata[METADATA_VOLUMES] = "vol-active" }},
		{"tag_API_error", func(f *orphanFixture) { f.elb.tagsErr = apiErr }},
		{"EC2_API_error", func(f *orphanFixture) { f.ec2.instancesErr = apiErr }},
		{"CF_API_error", func(f *orphanFixture) { f.cf.err = apiErr }},
		{"endpoint_API_error", func(f *orphanFixture) { f.ec2.endpointsErr = apiErr }},
		{"target_group_API_error", func(f *orphanFixture) { f.elb.groupsErr = apiErr }},
		{"target_health_API_error", func(f *orphanFixture) { f.elb.healthErr = apiErr }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newOrphanFixture()
			tc.modify(f)
			errs := f.delete()
			if strings.Contains(tc.name, "API_error") || tc.name == "extra_non_LB_metadata" {
				require.NotEmpty(t, errs)
			} else {
				require.Empty(t, errs) // Changed eligibility is a safe skip, not a failed run.
			}
			assertNoMutations(t, f.ch)
		})
	}
	t.Run("dry_run", func(t *testing.T) {
		f := newOrphanFixture()
		f.cfg.DryRun = true
		assert.Empty(t, f.delete())
		assertNoMutations(t, f.ch)
	})
	t.Run("already_deleted", func(t *testing.T) {
		f := newOrphanFixture()
		f.elb.lbs = nil
		assert.Empty(t, f.delete())
		assertNoMutations(t, f.ch)
	})
	t.Run("partially_deleted_group", func(t *testing.T) {
		f := newOrphanFixture()
		f.elb.lbs = f.elb.lbs[1:]
		assert.Empty(t, f.delete())
		ops := closeAndCollectOperations(f.ch)
		assert.Contains(t, ops, "DeleteLoadBalancer:lb-2")
		assert.NotContains(t, ops, "DeleteLoadBalancer:lb-1")
		assert.NotContains(t, ops, "ModifyLoadBalancerAttributes:lb-1")
	})
	t.Run("pagination", func(t *testing.T) {
		f := newOrphanFixture()
		f.ec2.pages = true
		f.elb.pages = true
		f.cf.pages = true
		f.cf.stacks = []*cloudformation.Stack{{StackId: aws.String("unrelated-cf-stack")}}
		assert.Empty(t, f.delete())
		ops := closeAndCollectOperations(f.ch)
		assert.Contains(t, ops, "ListStackResources")
		assert.Contains(t, ops, "DeleteLoadBalancer:lb-1")
		assert.Contains(t, ops, "DeleteLoadBalancer:lb-2")
	})
}

func TestCFInventoryErrorsCannotProducePartialStacks(t *testing.T) {
	f := newOrphanFixture()
	failure := errors.New("CF inventory denied")
	bad := f.cf
	bad.err = failure
	f.cf.stacks = []*cloudformation.Stack{{StackId: aws.String("cf-ok"), StackName: aws.String("cf-ok"), CreationTime: &NOW, StackStatus: aws.String("CREATE_COMPLETE")}}
	stacks, err := getCFStacks(f.cfg, types.AWS, map[string]cfClient{orphanTestRegion: f.cf, "eu-central-1": bad})
	assert.ErrorIs(t, err, failure)
	assert.Empty(t, stacks)
	assertNoMutations(t, f.ch)
}

func TestElbTagsHandleMissingValues(t *testing.T) {
	tags := getElbTags([]*elb.Tag{nil, {Key: aws.String(config.DefaultIgnoreLabel)}, {Key: nil, Value: aws.String("bad")}})
	assert.Contains(t, tags, config.DefaultIgnoreLabel)
	assert.Empty(t, tags[config.DefaultIgnoreLabel])
	assert.NotContains(t, tags, "")
}

func TestNativeStackListingFailsClosed(t *testing.T) {
	apiErr := errors.New("cannot complete inventory")
	for _, where := range []string{"list", "tags", "instances", "addresses", "security_groups", "alarms", "endpoint", "groups", "health"} {
		t.Run(where, func(t *testing.T) {
			f := newOrphanFixture()
			switch where {
			case "list":
				f.elb.listErr = apiErr
			case "addresses":
				f.ec2.addressesErr = apiErr
			case "security_groups":
				f.ec2.groupsErr = apiErr
			case "tags":
				f.elb.tagsErr = apiErr
			case "instances":
				f.ec2.instancesErr = apiErr
			case "endpoint":
				f.ec2.endpointsErr = apiErr
			case "groups":
				f.elb.groupsErr = apiErr
			case "health":
				f.elb.healthErr = apiErr
			}
			var stacks []*types.Stack
			var err error
			if where == "alarms" {
				stacks, err = getNativeStacks(f.cfg, types.AWS, map[string]ec2Client{orphanTestRegion: f.ec2}, map[string]elbClient{orphanTestRegion: f.elb}, map[string]cloudWatchClient{orphanTestRegion: failingOrphanCW{mockCwClient: mockCwClient{operationChannel: f.ch}, err: apiErr}}, nil)
			} else {
				stacks, err = f.discover(nil)
			}
			if where == "endpoint" || where == "groups" || where == "health" {
				assert.NoError(t, err) // Optional orphan checks do not abort established cleanup.
			} else {
				assert.ErrorIs(t, err, apiErr)
			}
			assert.Empty(t, stacks)
			assertNoMutations(t, f.ch)
		})
	}
}
