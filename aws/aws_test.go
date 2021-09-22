package aws

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudtrail"
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
	ctClients := map[string]cloudTrailClient{"region": mockCtClient{}}

	instances, _ := getInstances(ec2Clients, ctClients)

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

func TestDeleteVolumes(t *testing.T) {
	region1Chan := make(chan string)
	region2Chan := make(chan string)
	clients := map[string]ec2Client{
		"region-1": mockEc2Client{deregisterVolumesChannel: region1Chan},
		"region-2": mockEc2Client{deregisterVolumesChannel: region2Chan},
	}
	volumes := []*types.Disk{
		{CloudType: types.AWS, ID: "disk-id-1", Region: "region-1"},
		{CloudType: types.AWS, ID: "disk-id-2", Region: "region-2"},
	}

	go func() {
		defer close(region1Chan)
		defer close(region2Chan)

		deleteVolumes(clients, volumes)
	}()

	assert.Equal(t, "disk-id-1", <-region1Chan)
	assert.Equal(t, "disk-id-2", <-region2Chan)
}

func TestDeleteImages(t *testing.T) {
	region1Chan := make(chan string)
	region2Chan := make(chan string)
	clients := map[string]ec2Client{
		"region-1": mockEc2Client{deregisterImagesChannel: region1Chan},
		"region-2": mockEc2Client{deregisterImagesChannel: region2Chan},
	}
	images := []*types.Image{
		{CloudType: types.AWS, ID: "ami-id-1", Region: "region-1"},
		{CloudType: types.AWS, ID: "ami-id-2", Region: "region-2"},
	}

	go func() {
		defer close(region1Chan)
		defer close(region2Chan)

		deleteImages(clients, images)
	}()

	assert.Equal(t, "ami-id-1", <-region1Chan)
	assert.Equal(t, "ami-id-2", <-region2Chan)
}

func TestNewInstanceWithName(t *testing.T) {
	ec2Instance := newTestInstance()
	ec2Instance.Tags = []*ec2.Tag{{Key: &(&types.S{S: "Name"}).S, Value: &(&types.S{S: "name"}).S}}

	instance := newInstance(ec2Instance)

	assert.Equal(t, "name", instance.Name)
}

func TestNewInstanceMissingName(t *testing.T) {
	instance := newInstance(newTestInstance())

	assert.Equal(t, "ID", instance.Name)
}
func TestGetTags(t *testing.T) {
	assert.Equal(t, types.Tags{"k": "v"}, getEc2Tags([]*ec2.Tag{{Key: &(&types.S{S: "k"}).S, Value: &(&types.S{S: "v"}).S}}))
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

func TestGgetNameIDPairs(t *testing.T) {
	instIDNames, instanceIDs := getNameIDPairs([]*types.Instance{{Name: "name", ID: "id"}})

	assert.Equal(t, 1, len(instIDNames))
	assert.Equal(t, 1, len(instanceIDs))
	assert.Equal(t, "name", instIDNames["id"])
	assert.Equal(t, "id", *instanceIDs[0])
}

func TestRemoveInstance(t *testing.T) {
	instanceIDs := removeInstance([]*string{&(&types.S{S: "ID1"}).S, &(&types.S{S: "ID2"}).S}, &(&types.S{S: "ID1"}).S)

	assert.Equal(t, 1, len(instanceIDs))
	assert.Equal(t, "ID2", *instanceIDs[0])
}

func TestNewStackOwner(t *testing.T) {
	ownerTag := cloudformation.Tag{}
	ownerTag.SetKey("owner")
	ownerTag.SetValue("validOwner")
	cfStack := cloudformation.Stack{}
	cfStack.SetStackId("id")
	cfStack.SetStackName("name")
	cfStack.SetTags([]*cloudformation.Tag{&ownerTag})

	stack := newStack(&cfStack, "us-west-1")

	assert.Equal(t, "validOwner", stack.Owner)
}

type mockEc2Client struct {
	deregisterVolumesChannel chan (string)
	deregisterImagesChannel  chan (string)
}

func (t mockEc2Client) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			{
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
			{
				RegionName: &(&types.S{S: "region"}).S,
			},
		},
	}, nil
}

func (t mockEc2Client) DescribeVolumes(input *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
	return nil, nil
}

func (t mockEc2Client) DescribeImages(input *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error) {
	return nil, nil
}

func (t mockEc2Client) DeleteVolume(input *ec2.DeleteVolumeInput) (*ec2.DeleteVolumeOutput, error) {
	t.deregisterVolumesChannel <- *input.VolumeId
	return nil, nil
}

func (t mockEc2Client) DetachVolume(input *ec2.DetachVolumeInput) (*ec2.VolumeAttachment, error) {
	return nil, nil
}

func (t mockEc2Client) DeregisterImage(input *ec2.DeregisterImageInput) (*ec2.DeregisterImageOutput, error) {
	t.deregisterImagesChannel <- *input.ImageId
	return nil, nil
}

type mockCtClient struct {
}

func (t mockCtClient) LookupEvents(input *cloudtrail.LookupEventsInput) (*cloudtrail.LookupEventsOutput, error) {
	return nil, nil
}

type mockIamClient struct {
}

func (t mockIamClient) ListUsers(*iam.ListUsersInput) (*iam.ListUsersOutput, error) {
	return &iam.ListUsersOutput{
		Users: []*iam.User{
			{
				UserName: &(&types.S{S: "user"}).S,
			},
		},
	}, nil
}

func (t mockIamClient) ListAccessKeys(*iam.ListAccessKeysInput) (*iam.ListAccessKeysOutput, error) {
	now := time.Now()
	return &iam.ListAccessKeysOutput{
		AccessKeyMetadata: []*iam.AccessKeyMetadata{
			{
				AccessKeyId: &(&types.S{S: "ACCESSKEY_1"}).S,
				UserName:    &(&types.S{S: "user"}).S,
				CreateDate:  &now,
				Status:      &(&types.S{S: "Active"}).S,
			},
			{
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
		InstanceId:   &(&types.S{S: "ID"}).S,
		LaunchTime:   &now,
		Placement:    &ec2.Placement{},
		InstanceType: &(&types.S{S: "m5.xlarge"}).S,
	}
}
