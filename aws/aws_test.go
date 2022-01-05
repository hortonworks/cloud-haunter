package aws

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudtrail"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	elb "github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/rds"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

var (
	RESOURCE_GROUPING_TAG_VALUE = "crn"
	NOW                         = time.Now()
	OWNER                       = "user"
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
	elbClients := map[string]elbClient{
		"eu-central-1": mockElbClient{operationChannel: operationChannel},
	}
	cwClients := map[string]cloudWatchClient{
		"eu-central-1": mockCwClient{operationChannel: operationChannel},
	}
	stacks := []*types.Stack{
		{
			CloudType: types.AWS,
			Name:      "stack",
			Region:    "eu-central-1",
			Metadata: map[string]string{
				METADATA_TYPE:        TYPE_CF,
				METADATA_ELASTIC_IPS: "ip-1",
			},
		},
	}

	go func() {
		defer close(operationChannel)

		deleteStacks(cfClients, rdsClients, ec2Clients, elbClients, cwClients, stacks)
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
	assert.Equal(t, "DeleteLoadBalancer:elb-arn", <-operationChannel)
	assert.Equal(t, "DeleteStack", <-operationChannel)
	assert.Equal(t, "WaitUntilStackDeleteComplete", <-operationChannel)
	assert.Equal(t, "ReleaseAddress:ip-1", <-operationChannel)
}

func TestRemoveNativeStack(t *testing.T) {
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
	elbClients := map[string]elbClient{
		"eu-central-1": mockElbClient{operationChannel: operationChannel},
	}
	cwClients := map[string]cloudWatchClient{
		"eu-central-1": mockCwClient{operationChannel: operationChannel},
	}
	stacks := []*types.Stack{
		{
			CloudType: types.AWS,
			Name:      "stack",
			Region:    "eu-central-1",
			Metadata: map[string]string{
				METADATA_TYPE:            TYPE_NATIVE,
				METADATA_LOAD_BALANCERS:  "lb-1,lb-2",
				METADATA_INSTANCES:       "i",
				METADATA_VOLUMES:         "vol",
				METADATA_SECURITY_GROUPS: "sg",
				METADATA_ELASTIC_IPS:     "ip",
				METADATA_ALARMS:          "alarm",
			},
		},
	}

	go func() {
		defer close(operationChannel)

		deleteStacks(cfClients, rdsClients, ec2Clients, elbClients, cwClients, stacks)
	}()

	assert.Equal(t, "TerminateInstances:i", <-operationChannel)
	assert.Equal(t, "WaitUntilInstanceTerminated:i", <-operationChannel)

	asyncOperations := []string{}
	for op := range operationChannel {
		asyncOperations = append(asyncOperations, op)
	}

	assert.Equal(t, 6, len(asyncOperations))
	assert.Contains(t, asyncOperations, "DeleteVolume:vol")
	assert.Contains(t, asyncOperations, "DeleteSecurityGroup:sg")
	assert.Contains(t, asyncOperations, "DeleteLoadBalancer:lb-1")
	assert.Contains(t, asyncOperations, "DeleteLoadBalancer:lb-2")
	assert.Contains(t, asyncOperations, "ReleaseAddress:ip")
	assert.Contains(t, asyncOperations, "DeleteAlarms:alarm")
}

func TestGetNativeStacks(t *testing.T) {
	operationChannel := make(chan string)

	ec2Clients := map[string]ec2Client{
		"eu-central-1": mockEc2Client{operationChannel: operationChannel},
	}
	elbClients := map[string]elbClient{
		"eu-central-1": mockElbClient{operationChannel: operationChannel},
	}
	cwClients := map[string]cloudWatchClient{
		"eu-central-1": mockCwClient{operationChannel: operationChannel},
	}

	var stack *types.Stack

	go func() {
		defer close(operationChannel)

		stacks, err := getNativeStacks(ec2Clients, elbClients, cwClients)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(stacks))
		stack = stacks[0]
	}()

	for _ = range operationChannel {
		// ignore
	}

	assert.Equal(t, RESOURCE_GROUPING_TAG_VALUE, stack.ID)
	assert.Equal(t, RESOURCE_GROUPING_TAG_VALUE, stack.Name)
	assert.Equal(t, NOW, stack.Created)
	assert.Equal(t, types.AWS, stack.CloudType)
	assert.Equal(t, getEc2Tags(getResourceGroupedEc2Tags()), stack.Tags)
	assert.Equal(t, OWNER, stack.Owner)
	assert.Equal(t, "eu-central-1", stack.Region)
	assert.Equal(t, types.Running, stack.State)
	assert.Equal(t, TYPE_NATIVE, stack.Metadata[METADATA_TYPE])
	assert.Equal(t, "lb-1", stack.Metadata[METADATA_LOAD_BALANCERS])
	assert.Equal(t, "ID", stack.Metadata[METADATA_INSTANCES])
	assert.Equal(t, "vol-2", stack.Metadata[METADATA_VOLUMES])
	assert.Equal(t, "default-group-id,custom-group-id", stack.Metadata[METADATA_SECURITY_GROUPS])
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
	t.operationChannel <- "DeleteVolume:" + *input.VolumeId
	if t.deregisterVolumesChannel != nil {
		t.deregisterVolumesChannel <- *input.VolumeId
	}
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
				Tags:      getResourceGroupedEc2Tags(),
			},
			{
				GroupId:   &(&types.S{S: "custom-group-id"}).S,
				GroupName: &(&types.S{S: "custom"}).S,
				Tags:      getResourceGroupedEc2Tags(),
			},
		},
	}, nil
}

func (t mockEc2Client) DeleteSecurityGroup(input *ec2.DeleteSecurityGroupInput) (*ec2.DeleteSecurityGroupOutput, error) {
	t.operationChannel <- "DeleteSecurityGroup:" + *input.GroupId
	return nil, nil
}

func (t mockEc2Client) TerminateInstances(input *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	t.operationChannel <- "TerminateInstances:" + strings.Join(aws.StringValueSlice(input.InstanceIds), ",")
	return nil, nil
}

func (t mockEc2Client) WaitUntilInstanceTerminated(input *ec2.DescribeInstancesInput) error {
	t.operationChannel <- "WaitUntilInstanceTerminated:" + strings.Join(aws.StringValueSlice(input.InstanceIds), ",")
	return nil
}

func (t mockEc2Client) DescribeAddresses(input *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error) {
	t.operationChannel <- "DescribeAddresses"
	return &ec2.DescribeAddressesOutput{
		Addresses: []*ec2.Address{
			{
				AllocationId: aws.String("ip-1"),
				Tags:         getResourceGroupedEc2Tags(),
			},
		},
	}, nil
}

func (t mockEc2Client) ReleaseAddress(input *ec2.ReleaseAddressInput) (*ec2.ReleaseAddressOutput, error) {
	t.operationChannel <- "ReleaseAddress:" + aws.StringValue(input.AllocationId)
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
	return &ec2.Instance{
		InstanceId:   &(&types.S{S: "ID"}).S,
		LaunchTime:   &NOW,
		Placement:    &ec2.Placement{},
		InstanceType: &(&types.S{S: "m5.xlarge"}).S,
		State: &ec2.InstanceState{
			Code: aws.Int64(0),
			Name: aws.String(ec2.StateAvailable),
		},
		BlockDeviceMappings: []*ec2.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda"),
				Ebs: &ec2.EbsInstanceBlockDevice{
					DeleteOnTermination: aws.Bool(true),
					VolumeId:            aws.String("vol-1"),
				},
			},
			{
				DeviceName: aws.String("/dev/sdb"),
				Ebs: &ec2.EbsInstanceBlockDevice{
					DeleteOnTermination: aws.Bool(false),
					VolumeId:            aws.String("vol-2"),
				},
			},
		},
		Tags: getResourceGroupedEc2Tags(),
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
				LogicalResourceId:  &(&types.S{S: "rds-id"}).S,
				ResourceStatus:     &(&types.S{S: "AVAILABLE"}).S,
			},
			{
				ResourceType:       &(&types.S{S: "AWS::EC2::VPC"}).S,
				PhysicalResourceId: &(&types.S{S: "vpc-id"}).S,
				LogicalResourceId:  &(&types.S{S: "vpc-id"}).S,
				ResourceStatus:     &(&types.S{S: "AVAILABLE"}).S,
			},
			{
				ResourceType:       &(&types.S{S: "AWS::ElasticLoadBalancingV2::LoadBalancer"}).S,
				PhysicalResourceId: &(&types.S{S: "elb-arn"}).S,
				LogicalResourceId:  &(&types.S{S: "elb-id"}).S,
				ResourceStatus:     &(&types.S{S: "AVAILABLE"}).S,
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

type mockElbClient struct {
	operationChannel chan (string)
}

func (t mockElbClient) DescribeLoadBalancers(input *elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error) {
	t.operationChannel <- "DescribeLoadBalancers"
	return &elb.DescribeLoadBalancersOutput{
		LoadBalancers: []*elb.LoadBalancer{
			{
				LoadBalancerArn: aws.String("lb-1"),
			},
		},
	}, nil
}

func (t mockElbClient) DescribeTags(input *elb.DescribeTagsInput) (*elb.DescribeTagsOutput, error) {
	t.operationChannel <- "DescribeLoadBalancers"
	return &elb.DescribeTagsOutput{
		TagDescriptions: []*elb.TagDescription{
			{
				ResourceArn: aws.String("lb-1"),
				Tags:        getResourceGroupedElbTags(),
			},
		},
	}, nil
}

func (t mockElbClient) DeleteLoadBalancer(input *elb.DeleteLoadBalancerInput) (*elb.DeleteLoadBalancerOutput, error) {
	t.operationChannel <- "DeleteLoadBalancer:" + *input.LoadBalancerArn
	return nil, nil
}

type mockCwClient struct {
	operationChannel chan (string)
}

func (t mockCwClient) DescribeAlarms(input *cloudwatch.DescribeAlarmsInput) (*cloudwatch.DescribeAlarmsOutput, error) {
	t.operationChannel <- "DescribeAlarms"
	return &cloudwatch.DescribeAlarmsOutput{
		MetricAlarms: []*cloudwatch.MetricAlarm{
			{
				AlarmName: aws.String("alarm-1"),
				Dimensions: []*cloudwatch.Dimension{
					{
						Name:  aws.String("InstanceId"),
						Value: aws.String("ID"),
					},
				},
			},
		},
	}, nil
}

func (t mockCwClient) DeleteAlarms(input *cloudwatch.DeleteAlarmsInput) (*cloudwatch.DeleteAlarmsOutput, error) {
	t.operationChannel <- "DeleteAlarms:" + strings.Join(aws.StringValueSlice(input.AlarmNames), ",")
	return nil, nil
}

func getResourceGroupedElbTags() []*elb.Tag {
	return []*elb.Tag{
		{
			Key:   aws.String(ctx.ResourceGroupingLabel),
			Value: aws.String(RESOURCE_GROUPING_TAG_VALUE),
		},
	}
}

func getResourceGroupedEc2Tags() []*ec2.Tag {
	return []*ec2.Tag{
		{
			Key:   aws.String(ctx.ResourceGroupingLabel),
			Value: aws.String(RESOURCE_GROUPING_TAG_VALUE),
		},
		{
			Key:   aws.String(ctx.OwnerLabel),
			Value: aws.String(OWNER),
		},
	}
}
