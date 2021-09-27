package aws

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudtrail"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/rds"
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
	ec2Clients := map[string]ec2Client{"region": mockEc2Client{operationChannel: make(chan string, 10)}}
	ctClients := map[string]cloudTrailClient{"region": mockCtClient{}}

	instances, _ := getInstances(ec2Clients, ctClients)

	assert.Equal(t, 1, len(instances))
}

func TestGetAccesses(t *testing.T) {
	accesses, _ := getAccesses(mockIamClient{})

	assert.Equal(t, 1, len(accesses))
}

func TestGetRegions(t *testing.T) {
	regions, _ := getRegions(mockEc2Client{operationChannel: make(chan string, 10)})

	assert.Equal(t, 1, len(regions))
}

func TestDeleteVolumes(t *testing.T) {
	region1Chan := make(chan string)
	region2Chan := make(chan string)
	clients := map[string]ec2Client{
		"region-1": mockEc2Client{deregisterVolumesChannel: region1Chan, operationChannel: make(chan string, 10)},
		"region-2": mockEc2Client{deregisterVolumesChannel: region2Chan, operationChannel: make(chan string, 10)},
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
		"region-1": mockEc2Client{deregisterImagesChannel: region1Chan, operationChannel: make(chan string, 10)},
		"region-2": mockEc2Client{deregisterImagesChannel: region2Chan, operationChannel: make(chan string, 10)},
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

func TestRemoveCfStack(t *testing.T) {
	operationChannel := make(chan string)

	cfClients := map[string]cfClient{
		"eu-central-1": mockCfClient{operationChannel: operationChannel},
	}
	rdsClients := map[string]rdsClient{
		"eu-central-1": mockRdsClient{operationChannel: operationChannel},
	}
	ec2Clients := map[string]ec2Client{
		"eu-central-1": mockEc2Client{operationChannel: operationChannel},
	}
	stacks := []*types.Stack{
		{
			CloudType: types.AWS,
			Name:      "stack",
			Region:    "eu-central-1",
		},
	}

	go func() {
		defer close(operationChannel)

		deleteCFStacks(cfClients, rdsClients, ec2Clients, stacks)
	}()

	assert.Equal(t, "DescribeStackResources", <-operationChannel)
	assert.Equal(t, "ModifyDBInstance", <-operationChannel)
	assert.Equal(t, "DescribeVpcEndpoints", <-operationChannel)
	assert.Equal(t, "DeleteVpcEndpoints:vpc-endpoint-id-1,vpc-endpoint-id-2", <-operationChannel)
	assert.Equal(t, "DescribeSubnets", <-operationChannel)
	assert.Equal(t, "DeleteSubnet:subnet-id-1", <-operationChannel)
	assert.Equal(t, "DeleteSubnet:subnet-id-2", <-operationChannel)
	assert.Equal(t, "DescribeSecurityGroups", <-operationChannel)
	assert.Equal(t, "DeleteSecurityGroup:custom-group-id", <-operationChannel)
	assert.Equal(t, "DeleteVpc", <-operationChannel)
	assert.Equal(t, "DeleteStack", <-operationChannel)
	assert.Equal(t, "WaitUntilStackDeleteComplete", <-operationChannel)
}

type mockEc2Client struct {
	deregisterVolumesChannel chan (string)
	deregisterImagesChannel  chan (string)
	operationChannel         chan (string)
}

func (t mockEc2Client) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	t.operationChannel <- "DescribeInstances"
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
	t.operationChannel <- "DescribeRegions"
	return &ec2.DescribeRegionsOutput{
		Regions: []*ec2.Region{
			{
				RegionName: &(&types.S{S: "region"}).S,
			},
		},
	}, nil
}

func (t mockEc2Client) DescribeVolumes(input *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
	t.operationChannel <- "DescribeVolumes"
	return nil, nil
}

func (t mockEc2Client) DescribeImages(input *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error) {
	t.operationChannel <- "DescribeImages"
	return nil, nil
}

func (t mockEc2Client) DeleteVolume(input *ec2.DeleteVolumeInput) (*ec2.DeleteVolumeOutput, error) {
	t.operationChannel <- "DeleteVolume"
	t.deregisterVolumesChannel <- *input.VolumeId
	return nil, nil
}

func (t mockEc2Client) DetachVolume(input *ec2.DetachVolumeInput) (*ec2.VolumeAttachment, error) {
	t.operationChannel <- "DetachVolume"
	return nil, nil
}

func (t mockEc2Client) DeregisterImage(input *ec2.DeregisterImageInput) (*ec2.DeregisterImageOutput, error) {
	t.operationChannel <- "DeregisterImage"
	t.deregisterImagesChannel <- *input.ImageId
	return nil, nil
}

func (t mockEc2Client) DescribeVpcEndpoints(input *ec2.DescribeVpcEndpointsInput) (*ec2.DescribeVpcEndpointsOutput, error) {
	t.operationChannel <- "DescribeVpcEndpoints"
	return &ec2.DescribeVpcEndpointsOutput{
		VpcEndpoints: []*ec2.VpcEndpoint{
			{
				VpcEndpointId: &(&types.S{S: "vpc-endpoint-id-1"}).S,
			},
			{
				VpcEndpointId: &(&types.S{S: "vpc-endpoint-id-2"}).S,
			},
		},
	}, nil
}

func (t mockEc2Client) DeleteVpcEndpoints(input *ec2.DeleteVpcEndpointsInput) (*ec2.DeleteVpcEndpointsOutput, error) {
	ids := []string{}
	for _, id := range input.VpcEndpointIds {
		ids = append(ids, *id)
	}
	t.operationChannel <- "DeleteVpcEndpoints:" + strings.Join(ids, ",")
	return nil, nil
}

func (t mockEc2Client) DeleteVpc(input *ec2.DeleteVpcInput) (*ec2.DeleteVpcOutput, error) {
	t.operationChannel <- "DeleteVpc"
	return nil, nil
}

func (t mockEc2Client) DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	t.operationChannel <- "DescribeSubnets"
	return &ec2.DescribeSubnetsOutput{
		Subnets: []*ec2.Subnet{
			{
				SubnetId: &(&types.S{S: "subnet-id-1"}).S,
			},
			{
				SubnetId: &(&types.S{S: "subnet-id-2"}).S,
			},
		},
	}, nil
}

func (t mockEc2Client) DeleteSubnet(input *ec2.DeleteSubnetInput) (*ec2.DeleteSubnetOutput, error) {
	t.operationChannel <- "DeleteSubnet:" + *input.SubnetId
	return nil, nil
}

func (t mockEc2Client) DescribeSecurityGroups(input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error) {
	t.operationChannel <- "DescribeSecurityGroups"
	return &ec2.DescribeSecurityGroupsOutput{
		SecurityGroups: []*ec2.SecurityGroup{
			{
				GroupId:   &(&types.S{S: "default-group-id"}).S,
				GroupName: &(&types.S{S: "default"}).S,
			},
			{
				GroupId:   &(&types.S{S: "custom-group-id"}).S,
				GroupName: &(&types.S{S: "custom"}).S,
			},
		},
	}, nil
}

func (t mockEc2Client) DeleteSecurityGroup(input *ec2.DeleteSecurityGroupInput) (*ec2.DeleteSecurityGroupOutput, error) {
	t.operationChannel <- "DeleteSecurityGroup:" + *input.GroupId
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

type mockCfClient struct {
	operationChannel chan (string)
}

func (t mockCfClient) DescribeStacks(*cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	t.operationChannel <- "DescribeStacks"
	return nil, nil
}

func (t mockCfClient) DeleteStack(input *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	t.operationChannel <- "DeleteStack"
	return nil, nil
}

func (t mockCfClient) DescribeStackResource(input *cloudformation.DescribeStackResourceInput) (*cloudformation.DescribeStackResourceOutput, error) {
	t.operationChannel <- "DescribeStackResource"
	return nil, nil
}

func (t mockCfClient) DescribeStackResources(input *cloudformation.DescribeStackResourcesInput) (*cloudformation.DescribeStackResourcesOutput, error) {
	t.operationChannel <- "DescribeStackResources"
	return &cloudformation.DescribeStackResourcesOutput{
		StackResources: []*cloudformation.StackResource{
			{
				ResourceType:       &(&types.S{S: "AWS::RDS::DBInstance"}).S,
				PhysicalResourceId: &(&types.S{S: "rds-id"}).S,
			},
			{
				ResourceType:       &(&types.S{S: "AWS::EC2::VPC"}).S,
				PhysicalResourceId: &(&types.S{S: "vpc-id"}).S,
			},
		},
	}, nil
}

func (t mockCfClient) WaitUntilStackDeleteComplete(input *cloudformation.DescribeStacksInput) error {
	t.operationChannel <- "WaitUntilStackDeleteComplete"
	return nil
}

type mockRdsClient struct {
	operationChannel chan (string)
}

func (t mockRdsClient) DescribeDBInstances(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
	t.operationChannel <- "DescribeDBInstances"
	return nil, nil
}

func (t mockRdsClient) ListTagsForResource(input *rds.ListTagsForResourceInput) (*rds.ListTagsForResourceOutput, error) {
	t.operationChannel <- "ListTagsForResource"
	return nil, nil
}

func (t mockRdsClient) ModifyDBInstance(input *rds.ModifyDBInstanceInput) (*rds.ModifyDBInstanceOutput, error) {
	t.operationChannel <- "ModifyDBInstance"
	return nil, nil
}
