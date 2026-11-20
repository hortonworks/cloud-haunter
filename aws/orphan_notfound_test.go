package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrphanDisappearanceIsSafeSkip(t *testing.T) {
	for _, where := range []string{"tags", "target_groups", "target_health"} {
		for _, action := range []bool{false, true} {
			t.Run(where+map[bool]string{false: "/selection", true: "/action"}[action], func(t *testing.T) {
				f := newOrphanFixture()
				missing := awserr.New("LoadBalancerNotFound", "deleted concurrently", nil)
				switch where {
				case "tags":
					f.elb.tagsErr = missing
				case "target_groups":
					f.elb.groupsErr = missing
				case "target_health":
					f.elb.healthErr = awserr.New("TargetGroupNotFound", "deleted concurrently", nil)
				}
				if action {
					assert.Empty(t, f.delete())
				} else {
					stacks, err := f.discover(nil)
					assert.NoError(t, err)
					assert.Empty(t, stacks)
				}
				assertNoMutations(t, f.ch)
			})
		}
	}
}

func TestOrphanActionRefreshesSelectionOwnershipSnapshot(t *testing.T) {
	f := newOrphanFixture()
	stacks, err := f.discover(nil)
	require.NoError(t, err)
	require.Len(t, stacks, 1)
	f.stack = stacks[0]
	f.cf.stacks = []*cloudformation.Stack{{StackId: aws.String("imported-after-selection")}}
	f.cf.resources = []*cloudformation.StackResourceSummary{{PhysicalResourceId: aws.String("lb-2")}}
	assert.Empty(t, f.delete())
	ops := orphanReviewReadOnlyOperations(t, f.ch)
	assert.Contains(t, ops, "DescribeStacks")
	assert.Contains(t, ops, "ListStackResources")
}

func TestOrphanCFPhysicalReferenceMatchesARNNotName(t *testing.T) {
	f := newOrphanFixture()
	f.elb.lbs[1].LoadBalancerName = aws.String("shared-name")
	f.cf.stacks = []*cloudformation.Stack{{StackId: aws.String("unrelated")}}
	f.cf.resources = []*cloudformation.StackResourceSummary{{PhysicalResourceId: aws.String("shared-name")}}
	inventory := &orphanLBInventory{region: orphanTestRegion, cfClient: f.cf, ec2Client: f.ec2}
	checked, err := assembleOrphanLoadBalancerStack(f.cfg, orphanTestRegion, orphanTestGroup, f.elb.lbs, f.elb.tags, nil, nil, f.ec2, f.elb, inventory)
	require.NoError(t, err)
	require.NotNil(t, checked)
	assert.Equal(t, TYPE_NATIVE_LB, checked.Type)
	assert.Equal(t, []string{"lb-1", "lb-2"}, checked.LoadBalancers)
	assertNoMutations(t, f.ch)
}

// The group may have disappeared before tags are read; a second live native
// group must still be returned, while no unverified orphan is emitted.
func TestMissingOrphanTagReadPreservesLiveNative(t *testing.T) {
	f := newOrphanFixture()
	i := f.fullInstance(t, 16)
	i.Tags[0].Value = aws.String("live-native")
	f.ec2.instances = append(f.ec2.instances, i)
	live := *f.elb.lbs[0]
	live.LoadBalancerArn = aws.String("live-lb")
	live.LoadBalancerName = aws.String("live-lb")
	f.elb.lbs = append(f.elb.lbs, &live)
	f.elb.tags["live-lb"] = types.Tags{f.cfg.ResourceGroupingLabel: "live-native", f.cfg.OwnerLabel: "cloudbreak-qe"}
	client := orphanReviewELBFake{orphanELBFake: f.elb, failedARN: "lb-1", tagError: awserr.New("LoadBalancerNotFound", "already gone", nil)}
	// The remaining lb-2 can be classified on its own, with fresh action checks;
	// the vanished ARN is never attached to a new orphan group.
	stacks, err := getNativeStacks(f.cfg, types.AWS, map[string]ec2Client{orphanTestRegion: f.ec2}, map[string]elbClient{orphanTestRegion: client}, map[string]cloudWatchClient{orphanTestRegion: mockCwClient{operationChannel: f.ch}}, nil, map[string]cfClient{orphanTestRegion: f.cf})
	require.NoError(t, err)
	foundLive := false
	for _, s := range stacks {
		if s.ID == "live-native" {
			foundLive = true
			assert.Equal(t, TYPE_NATIVE, s.Metadata[METADATA_TYPE])
			assert.Equal(t, "live-lb", s.Metadata[METADATA_LOAD_BALANCERS])
		}
		if s.Metadata[METADATA_TYPE] == TYPE_NATIVE_LB {
			assert.Equal(t, "lb-2", s.Metadata[METADATA_LOAD_BALANCERS])
		}
	}
	assert.True(t, foundLive)
	assertNoMutations(t, f.ch)
}
