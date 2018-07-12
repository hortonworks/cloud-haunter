package aws

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

func TestProviderInit(t *testing.T) {
	provider := awsProvider{}

	provider.init(func() ([]string, error) {
		return []string{"region1", "region2"}, nil
	})

	assert.Equal(t, 2, len(provider.ec2Clients))
}

func TestGetRunningInstances(t *testing.T) {
	ec2Clients := map[string]ec2Client{"region": mockEc2Client{}}

	instances, _ := getRunningInstances(ec2Clients)

	assert.Equal(t, 1, len(instances))
}

func TestGetAccesses(t *testing.T) {
	accesses, _ := getAccesses(mockIamClient{})

	assert.Equal(t, 1, len(accesses))
}

func TestGetRegions(t *testing.T) {
	regions, _ := getRegions(mockEc2Client{})

	assert.Equal(t, 1, len(regions))
}

func TestNewInstanceWithName(t *testing.T) {
	ec2Instance := newTestInstance()
	ec2Instance.Tags = []*ec2.Tag{&ec2.Tag{Key: &(&types.S{S: "Name"}).S, Value: &(&types.S{S: "name"}).S}}

	instance := newInstance(ec2Instance)

	assert.Equal(t, "name", instance.Name)
}

func TestNewInstanceMissingName(t *testing.T) {
	instance := newInstance(newTestInstance())

	assert.Equal(t, "ID", instance.Name)
}
func TestGetTags(t *testing.T) {
	assert.Equal(t, types.Tags{"k": "v"}, getTags([]*ec2.Tag{&ec2.Tag{Key: &(&types.S{S: "k"}).S, Value: &(&types.S{S: "v"}).S}}))
}

func TestGetRegionFromAvailabilityZoneOk(t *testing.T) {
	assert.Equal(t, "availabiltyzon", getRegionFromAvailabilityZone(&(&types.S{S: "availabiltyzone"}).S))
}

func TestGetRegionFromAvailabilityZoneNil(t *testing.T) {
	assert.Equal(t, "", getRegionFromAvailabilityZone(nil))
}

func TestGetRegionFromAvailabilityZoneLessThanOne(t *testing.T) {
	assert.Equal(t, "", getRegionFromAvailabilityZone(&(&types.S{S: "s"}).S))
}

type mockEc2Client struct {
}

func (t mockEc2Client) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			&ec2.Reservation{
				Instances: []*ec2.Instance{
					newTestInstance(),
				},
			},
		},
	}, nil
}

func (t mockEc2Client) DescribeRegions(*ec2.DescribeRegionsInput) (*ec2.DescribeRegionsOutput, error) {
	return &ec2.DescribeRegionsOutput{
		Regions: []*ec2.Region{
			&ec2.Region{
				RegionName: &(&types.S{S: "region"}).S,
			},
		},
	}, nil
}

type mockIamClient struct {
}

func (t mockIamClient) ListUsers(*iam.ListUsersInput) (*iam.ListUsersOutput, error) {
	return &iam.ListUsersOutput{
		Users: []*iam.User{
			&iam.User{
				UserName: &(&types.S{S: "user"}).S,
			},
		},
	}, nil
}

func (t mockIamClient) ListAccessKeys(*iam.ListAccessKeysInput) (*iam.ListAccessKeysOutput, error) {
	now := time.Now()
	return &iam.ListAccessKeysOutput{
		AccessKeyMetadata: []*iam.AccessKeyMetadata{
			&iam.AccessKeyMetadata{
				AccessKeyId: &(&types.S{S: "ACCESSKEY_1"}).S,
				UserName:    &(&types.S{S: "user"}).S,
				CreateDate:  &now,
				Status:      &(&types.S{S: "Active"}).S,
			},
			&iam.AccessKeyMetadata{
				AccessKeyId: &(&types.S{S: "ACCESSKEY_2"}).S,
				UserName:    &(&types.S{S: "user"}).S,
				CreateDate:  &now,
				Status:      &(&types.S{S: "NotActive"}).S,
			},
		},
	}, nil
}

func newTestInstance() *ec2.Instance {
	now := time.Now()
	return &ec2.Instance{
		InstanceId: &(&types.S{S: "ID"}).S,
		LaunchTime: &now,
		Placement:  &ec2.Placement{},
	}
}
