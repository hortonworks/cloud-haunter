package aws

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/hortonworks/cloud-haunter/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudtrail"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

var provider = awsProvider{}

type awsProvider struct {
	ec2Clients           map[string]*ec2.EC2
	autoScalingClients   map[string]*autoscaling.AutoScaling
	cloudTrailClient     map[string]*cloudtrail.CloudTrail
	cloudFormationClient map[string]*cloudformation.CloudFormation
	rdsClients           map[string]*rds.RDS
	s3Clients            map[string]*s3.S3
	iamClient            *iam.IAM
}

func init() {
	accessKeyId := os.Getenv("AWS_ACCESS_KEY_ID")
	if len(accessKeyId) == 0 {
		log.Warn("[AWS] AWS_ACCESS_KEY_ID environment variable is missing")
		return
	}
	secretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if len(secretAccessKey) == 0 {
		log.Warn("[AWS] AWS_SECRET_ACCESS_KEY environment variable is missing")
		return
	}
	ctx.CloudProviders[types.AWS] = func() types.CloudProvider {
		if len(provider.ec2Clients) == 0 {
			log.Debug("[AWS] Trying to prepare")
			ec2Client, err := newEc2Client("eu-west-1")
			if err != nil {
				panic("[AWS] Failed to create ec2 client, err: " + err.Error())
			}
			err = provider.init(func() ([]string, error) {
				log.Debug("[AWS] Fetching regions")
				return getRegions(ec2Client)
			})
			if err != nil {
				panic("[AWS] Failed to initialize provider, err: " + err.Error())
			}
			log.Info("[AWS] Successfully prepared")
		}
		return provider
	}
}

func (p *awsProvider) init(getRegions func() ([]string, error)) error {
	regions, err := getRegions()
	if err != nil {
		return err
	}

	p.ec2Clients = map[string]*ec2.EC2{}
	p.autoScalingClients = map[string]*autoscaling.AutoScaling{}
	p.rdsClients = map[string]*rds.RDS{}
	p.cloudTrailClient = map[string]*cloudtrail.CloudTrail{}
	p.cloudFormationClient = map[string]*cloudformation.CloudFormation{}
	p.s3Clients = map[string]*s3.S3{}

	for _, region := range regions {
		if client, err := newEc2Client(region); err != nil {
			panic(fmt.Sprintf("[AWS] Failed to create EC2 client in region %s, err: %s", region, err.Error()))
		} else {
			p.ec2Clients[region] = client
		}

		if client, err := newAutoscalingClient(region); err != nil {
			panic(fmt.Sprintf("[AWS] Failed to create ASG client in region %s, err: %s", region, err.Error()))
		} else {
			p.autoScalingClients[region] = client
		}

		if client, err := newRdsClient(region); err != nil {
			panic(fmt.Sprintf("[AWS] Failed to create RDS client in region %s, err: %s", region, err.Error()))
		} else {
			p.rdsClients[region] = client
		}

		if ctClient, err := newCloudTrailClient(region); err != nil {
			panic(fmt.Sprintf("[AWS] Failed to create CloudTrail client, err: %s", err.Error()))
		} else {
			p.cloudTrailClient[region] = ctClient
		}

		if cfClient, err := newCloudFormationClient(region); err != nil {
			panic(fmt.Sprintf("[AWS] Failed to create CloudFormation client, err: %s", err.Error()))
		} else {
			p.cloudFormationClient[region] = cfClient
		}

		if s3Client, err := newS3Client(region); err != nil {
			panic(fmt.Sprintf("[AWS] Failed to create S3 client, err: %s", err.Error()))
		} else {
			p.s3Clients[region] = s3Client
		}
	}
	if iamClient, err := newIamClient(); err != nil {
		panic(fmt.Sprintf("[AWS] Failed to create IAM client, err: %s", err.Error()))
	} else {
		p.iamClient = iamClient
	}
	return nil
}

func (p awsProvider) GetAccountName() string {
	log.Debugf("[AWS] Fetch account aliases")
	if result, err := p.iamClient.ListAccountAliases(&iam.ListAccountAliasesInput{}); err != nil {
		log.Errorf("[AWS] Failed to retrieve account aliases, err: %s", err.Error())
	} else {
		var aliases []string
		for _, a := range result.AccountAliases {
			aliases = append(aliases, *a)
		}
		if len(aliases) != 0 {
			return strings.Join(aliases, ",")
		}
	}
	return "unknown"
}

func (p awsProvider) GetInstances() ([]*types.Instance, error) {
	log.Debug("[AWS] Fetching instances")
	ec2Clients, ctClients := p.getEc2AndCTClientsByRegion()
	return getInstances(ec2Clients, ctClients)
}

func (p awsProvider) GetStacks() ([]*types.Stack, error) {
	log.Debug("[AWS] Fetching CloudFormation stacks")
	cfClients := p.getCFClientsByRegion()
	return getCFStacks(cfClients)
}

func (p awsProvider) GetDatabases() ([]*types.Database, error) {
	log.Debug("[AWS] Fetch databases")
	rdsClients := map[string]rdsClient{}
	ctClients := map[string]cloudTrailClient{}
	for k := range p.rdsClients {
		rdsClients[k] = p.rdsClients[k]
		ctClients[k] = p.cloudTrailClient[k]
	}
	return getDatabases(rdsClients, ctClients)
}

func (p awsProvider) GetDisks() ([]*types.Disk, error) {
	log.Debug("[AWS] Fetch volumes")
	ec2Clients, _ := p.getEc2AndCTClientsByRegion()
	return getDisks(ec2Clients)
}

func (p awsProvider) getEc2AndCTClientsByRegion() (map[string]ec2Client, map[string]cloudTrailClient) {
	ec2Clients := map[string]ec2Client{}
	ctClients := map[string]cloudTrailClient{}
	for k := range p.ec2Clients {
		ec2Clients[k] = p.ec2Clients[k]
		ctClients[k] = p.cloudTrailClient[k]
	}
	return ec2Clients, ctClients
}

func (p awsProvider) getCFClientsByRegion() map[string]cfClient {
	cfClients := map[string]cfClient{}
	for k := range p.cloudFormationClient {
		cfClients[k] = p.cloudFormationClient[k]
	}
	return cfClients
}

func (p awsProvider) getRdsClientsByRegion() map[string]rdsClient {
	rdsClients := map[string]rdsClient{}
	for k := range p.rdsClients {
		rdsClients[k] = p.rdsClients[k]
	}
	return rdsClients
}

func (p awsProvider) getS3ClientsByRegion() map[string]s3Client {
	s3Clients := map[string]s3Client{}
	for k := range p.s3Clients {
		s3Clients[k] = s3ClientImpl{p.s3Clients[k]}
	}
	return s3Clients
}

func (p awsProvider) TerminateInstances(*types.InstanceContainer) []error {
	return []error{errors.New("[AWS] Termination is not supported")}
}

func (p awsProvider) TerminateStacks(stacks *types.StackContainer) []error {
	log.Debug("[AWS] Delete CloudFormation stacks")
	cfClients := p.getCFClientsByRegion()
	rdsClients := p.getRdsClientsByRegion()
	ec2Clients, _ := p.getEc2AndCTClientsByRegion()
	s3Clients := p.getS3ClientsByRegion()
	return deleteCFStacks(cfClients, rdsClients, ec2Clients, s3Clients, stacks.Get(types.AWS))
}

func (p awsProvider) DeleteDisks(volumes *types.DiskContainer) []error {
	log.Debug("[AWS] Delete volumes")
	ec2Clients, _ := p.getEc2AndCTClientsByRegion()
	return deleteVolumes(ec2Clients, volumes.Get(types.AWS))
}

func (p awsProvider) GetImages() ([]*types.Image, error) {
	log.Debug("[AWS] Fetch images")
	ec2Clients, _ := p.getEc2AndCTClientsByRegion()
	return getImages(ec2Clients)
}

func (p awsProvider) DeleteImages(images *types.ImageContainer) []error {
	log.Debug("[AWS] Delete images")
	ec2Clients, _ := p.getEc2AndCTClientsByRegion()
	return deleteImages(ec2Clients, images.Get(types.AWS))
}

func (p awsProvider) StopInstances(instances *types.InstanceContainer) []error {
	log.Debug("[AWS] Stopping instances")
	regionInstances := map[string][]*types.Instance{}
	for _, instance := range instances.Get(types.AWS) {
		regionInstances[instance.Region] = append(regionInstances[instance.Region], instance)
	}
	log.Debugf("[AWS] Stopping instances: %v", regionInstances)

	wg := sync.WaitGroup{}
	wg.Add(len(regionInstances))
	errChan := make(chan error)

	for r, i := range regionInstances {
		go func(region string, instances []*types.Instance) {
			defer wg.Done()

			for i := 0; i < len(instances); i += ctx.AwsBulkOperationSize {
				log.Infof("[AWS] Round %d for stop operation", (i/ctx.AwsBulkOperationSize)+1)
				arrayEnd := i + ctx.AwsBulkOperationSize
				if ctx.AwsBulkOperationSize > len(instances[i:]) {
					arrayEnd = i + len(instances[i:])
				}

				instanceChunk := instances[i:arrayEnd]
				instIDNames, instanceIDs := getNameIDPairs(instanceChunk)
				log.Debugf("[AWS] Detecting auto scaling group for instances at %s (%d): %v", region, len(instanceIDs), instanceIDs)

				// We need to suspend the ASGs as it will terminate the stopped instances
				// Instances that are not part of an ASG are not returned
				asgInstances, err := p.autoScalingClients[region].DescribeAutoScalingInstances(&autoscaling.DescribeAutoScalingInstancesInput{
					InstanceIds: instanceIDs,
				})
				if err != nil {
					log.Errorf("[AWS] Failed to fetch the ASG instances in region: %s, err: %s", region, err)
					return
				}

				for _, instance := range asgInstances.AutoScalingInstances {
					instanceId := instance.InstanceId
					compactInstanceName := fmt.Sprintf("%s:%s", *instanceId, instIDNames[*instanceId])
					log.Debugf("[AWS] The following instance is in an ASG and will be suspended in region %s: %s", region, compactInstanceName)

					if _, err := p.autoScalingClients[region].SuspendProcesses(&autoscaling.ScalingProcessQuery{
						AutoScalingGroupName: instance.AutoScalingGroupName,
						ScalingProcesses: []*string{
							&(&types.S{S: "Launch"}).S,
							&(&types.S{S: "HealthCheck"}).S,
							&(&types.S{S: "ReplaceUnhealthy"}).S,
							&(&types.S{S: "AZRebalance"}).S,
							&(&types.S{S: "AlarmNotification"}).S,
							&(&types.S{S: "ScheduledActions"}).S,
							&(&types.S{S: "AddToLoadBalancer"}).S,
							&(&types.S{S: "RemoveFromLoadBalancerLowPriority"}).S,
						},
					}); err != nil {
						log.Errorf("[AWS] Failed to suspend ASG %v for instance %s, err: %s", instance.AutoScalingGroupName, compactInstanceName, err.Error())
						// Do not stop the instance if the ASG cannot be suspended otherwise the ASG will terminate the instance
						instanceIDs = removeInstance(instanceIDs, instanceId)
					}
				}

				if len(instanceIDs) > 0 {
					var spotInstanceIDs []*string
					for _, inst := range instanceChunk {
						if inst.Ephemeral {
							// could be that the instance is already removed due to failed suspend API call
							if containsInstanceID(instanceIDs, &inst.ID) {
								log.Infof("[AWS] Spot instance will be terminated: %s:%s", inst.ID, instIDNames[inst.ID])
								spotInstanceIDs = append(spotInstanceIDs, &inst.ID)
								instanceIDs = removeInstance(instanceIDs, &inst.ID)
							}
						}
					}

					stopInstancesNames := ""
					for _, inst := range instanceIDs {
						name := instIDNames[*inst]
						stopInstancesNames += fmt.Sprintf("\n%s:%s", *inst, name)
					}
					log.Infof("[AWS] Sending request to stop instances in region %s (%d): %v", region, len(instanceIDs), stopInstancesNames)
					if _, err := p.ec2Clients[region].StopInstances(&ec2.StopInstancesInput{InstanceIds: instanceIDs}); err != nil {
						errChan <- err
					}

					if len(spotInstanceIDs) > 0 {
						spotInstanceNames := ""
						for _, inst := range spotInstanceIDs {
							name := instIDNames[*inst]
							spotInstanceNames += fmt.Sprintf("\n%s:%s", *inst, name)
						}

						log.Infof("[AWS] Sending request to terminate spot instances in region %s (%d): %v", region, len(spotInstanceIDs), spotInstanceNames)
						if _, err := p.ec2Clients[region].TerminateInstances(&ec2.TerminateInstancesInput{InstanceIds: spotInstanceIDs}); err != nil {
							errChan <- err
						}
					} else {
						log.Info("[AWS] There are not spot instances to terminate")
					}
				}
			}
		}(r, i)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	return errs
}

func (p awsProvider) StopDatabases(databases *types.DatabaseContainer) []error {
	log.Debug("[AWS] Stopping databases")
	regionDatabases := map[string][]*types.Database{}
	for _, database := range databases.Get(types.AWS) {
		regionDatabases[database.Region] = append(regionDatabases[database.Region], database)
	}
	log.Debugf("[AWS] Stopping databases: %v", regionDatabases)

	wg := sync.WaitGroup{}
	wg.Add(len(regionDatabases))
	errChan := make(chan error)

	for r, db := range regionDatabases {
		go func(region string, databases []*types.Database) {
			defer wg.Done()
			for _, db := range databases {
				log.Infof("[AWS] Stop database: %s", db.Name)
				if _, err := p.rdsClients[region].StopDBInstance(&rds.StopDBInstanceInput{
					DBInstanceIdentifier: &db.Name,
				}); err != nil {
					errChan <- err
				}
			}
		}(r, db)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	return errs
}

func deleteCFStacks(cfClients map[string]cfClient, rdsClients map[string]rdsClient, ec2Clients map[string]ec2Client, s3Clients map[string]s3Client, stacks []*types.Stack) []error {
	regionCFStacks := map[string][]*types.Stack{}
	for _, stack := range stacks {
		regionCFStacks[stack.Region] = append(regionCFStacks[stack.Region], stack)
	}
	log.Debugf("[AWS] Delete CloudFormation stacks: %v", regionCFStacks)

	wg := sync.WaitGroup{}
	errChan := make(chan error)

	for r, stacks := range regionCFStacks {
		wg.Add(len(stacks))

		for _, s := range stacks {
			go func(cfClient cfClient, rdsClient rdsClient, ec2Client ec2Client, s3Client s3Client, region string, stack *types.Stack) {
				defer wg.Done()

				if ctx.DryRun {
					log.Infof("[AWS] Dry-run set, CloudFormation stack is not deleted: %s, region: %s", stack.Name, region)
				} else {
					stackResources, err := cfClient.DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
						StackName: &stack.Name,
					})
					if err != nil {
						log.Errorf("[AWS] Failed to fetch the CloudFormation stack resources for stack: %s, err: %s", stack.Name, err)
						errChan <- err
						return
					}
					var resourceErr error
					retainResources := []*string{}
					for _, stackResource := range stackResources.StackResources {
						switch *stackResource.ResourceType {
						case "AWS::RDS::DBInstance":
							if *stackResource.ResourceStatus != cloudformation.ResourceStatusDeleteComplete {
								var dbExists bool
								dbExists, resourceErr = disableDeleteProtection(rdsClient, stack.Name, *stackResource.PhysicalResourceId, region)
								if !dbExists {
									retainResources = append(retainResources, stackResource.LogicalResourceId)
								}
							}
						case "AWS::EC2::VPC":
							resourceErr = deleteVpcWithDependencies(ec2Client, stack.Name, *stackResource.PhysicalResourceId, region)
						case "AWS::S3::Bucket":
							resourceErr = deleteS3BucketWithContents(s3Client, stack.Name, *stackResource.PhysicalResourceId, region)
						}
						if resourceErr != nil {
							break
						}
					}
					if resourceErr != nil {
						log.Errorf("[AWS] Failed to delete resource of CloudFormation stack: %s, err: %s", stack.Name, resourceErr)
						errChan <- resourceErr
						return
					}

					log.Infof("[AWS] Delete CloudFormation stack: %s, region: %s", stack.Name, region)
					deleteStackInput := &cloudformation.DeleteStackInput{
						StackName:       &stack.Name,
						RetainResources: retainResources,
					}
					if _, err := cfClient.DeleteStack(deleteStackInput); err != nil {
						log.Errorf("[AWS] Failed to start deletion of CloudFormation stack: %s, err: %s", stack.Name, err)
						errChan <- err
						return
					}
					describeStacksInput := &cloudformation.DescribeStacksInput{StackName: &stack.Name}
					if err := cfClient.WaitUntilStackDeleteComplete(describeStacksInput); err != nil {
						if stacks, describeErr := cfClient.DescribeStacks(describeStacksInput); describeErr == nil && stacks.Stacks != nil {
							cfStack := *stacks.Stacks[0]
							err = fmt.Errorf("Cloudformation stack: %s, region: %s, is in status %s with reason: %s", *cfStack.StackName, region, *cfStack.StackStatus, *cfStack.StackStatusReason)
						} else {
							log.Warnf("[AWS] Failed to describe stack: %s, region: %s for status, err: %s", stack.Name, region, describeErr)
						}
						log.Errorf("[AWS] Failed to wait for delete complete of CloudFormation stack: %s, err: %s", stack.Name, err)
						errChan <- err
						return
					}
					log.Infof("[AWS] Delete CloudFormation stack: %s, region: %s was successful", stack.Name, region)
				}
			}(cfClients[r], rdsClients[r], ec2Clients[r], s3Clients[r], r, s)
		}
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	return errs
}

func disableDeleteProtection(rdsClient rdsClient, stackName string, dbInstanceId string, region string) (bool, error) {
	log.Infof("[AWS] Disabling DeletionProtection for DB instance: %s, region: %s", dbInstanceId, region)
	dbExists := true

	_, err := rdsClient.ModifyDBInstance(&rds.ModifyDBInstanceInput{
		DBInstanceIdentifier: &dbInstanceId,
		DeletionProtection:   aws.Bool(false),
	})
	if err != nil {
		dbExists = false
		if strings.Contains(err.Error(), "DB instance not found") {
			return dbExists, nil
		}
		log.Errorf("[AWS] Skipping stack %s delete because failed to disable DeletionProtection on db instance: %s, err: %s", stackName, dbInstanceId, err)
	}
	return dbExists, err
}

/**
 * CloudFormation is not able to delete a VPC if it has dependencies, so we have to delete them manually.
 */
func deleteVpcWithDependencies(ec2Client ec2Client, stackName string, vpcId string, region string) error {
	log.Infof("[AWS] Deleting endpoints of VPC: %s, region: %s", vpcId, region)

	vpcIdFilter := &ec2.Filter{}
	vpcIdFilter.SetName("vpc-id")
	vpcIdFilter.SetValues([]*string{&vpcId})
	vpcEndpoints, err := ec2Client.DescribeVpcEndpoints(&ec2.DescribeVpcEndpointsInput{
		Filters: []*ec2.Filter{vpcIdFilter},
	})
	if err != nil {
		log.Errorf("[AWS] Skipping stack %s delete because failed to list endpoints of VPC: %s, err: %s", stackName, vpcId, err)
		return err
	}

	vpcEndpointIds := []*string{}
	for _, vpcEndpoint := range vpcEndpoints.VpcEndpoints {
		vpcEndpointIds = append(vpcEndpointIds, vpcEndpoint.VpcEndpointId)
	}
	if len(vpcEndpointIds) > 0 {
		log.Debugf("[AWS] Deleting %d endpoints of VPC %s", len(vpcEndpointIds), vpcId)

		_, err = ec2Client.DeleteVpcEndpoints(&ec2.DeleteVpcEndpointsInput{
			VpcEndpointIds: vpcEndpointIds,
		})
		if err != nil {
			log.Errorf("[AWS] Skipping stack %s delete because failed to delete endpoints of VPC: %s, err: %s", stackName, vpcId, err)
			return err
		}
	}

	log.Infof("[AWS] Deleting subnets of VPC: %s, region: %s", vpcId, region)

	subnets, err := ec2Client.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{vpcIdFilter},
	})
	if err != nil {
		log.Errorf("[AWS] Skipping stack %s delete because failed to list subnets of VPC: %s, err: %s", stackName, vpcId, err)
		return err
	}
	for _, subnet := range subnets.Subnets {
		_, err = ec2Client.DeleteSubnet(&ec2.DeleteSubnetInput{
			SubnetId: subnet.SubnetId,
		})
		if err != nil {
			log.Errorf("[AWS] Skipping stack %s delete because failed to delete subnet: %s, err: %s", stackName, *subnet.SubnetId, err)
			return err
		}
	}

	log.Infof("[AWS] Deleting security groups of VPC: %s, region: %s", vpcId, region)

	securityGroups, err := ec2Client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{vpcIdFilter},
	})
	if err != nil {
		log.Errorf("[AWS] Skipping stack %s delete because failed to list security groups of VPC: %s, err: %s", stackName, vpcId, err)
		return err
	}
	for _, securityGroup := range securityGroups.SecurityGroups {
		if *securityGroup.GroupName == "default" {
			// the default subnet can not be deleted
			continue
		}
		_, err = ec2Client.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
			GroupId: securityGroup.GroupId,
		})
		if err != nil {
			log.Errorf("[AWS] Skipping stack %s delete because failed to delete security group: %s, err: %s", stackName, *securityGroup.GroupId, err)
			return err
		}
	}

	log.Infof("[AWS] Deleting VPC: %s, region: %s", vpcId, region)

	_, err = ec2Client.DeleteVpc(&ec2.DeleteVpcInput{
		VpcId: &vpcId,
	})
	if err != nil && err.(awserr.Error).Code() != "InvalidVpcID.NotFound" {
		log.Errorf("[AWS] Skipping stack %s delete because failed to delete VPC: %s, err: %s", stackName, vpcId, err)
		return err
	}

	return nil
}

func deleteS3BucketWithContents(s3Client s3Client, stackName string, s3BucketName string, region string) error {
	log.Infof("[AWS] Deleting S3 bucket: %s, region: %s", s3BucketName, region)

	if err := s3Client.DeleteBucketContents(s3BucketName); err != nil {
		log.Errorf("[AWS] Skipping stack %s delete because failed to delete contents of S3 bucket %s", stackName, s3BucketName)
		return err
	}

	if _, err := s3Client.DeleteBucket(&s3.DeleteBucketInput{Bucket: &s3BucketName}); err != nil {
		log.Errorf("[AWS] Skipping stack %s delete because failed to start deletion of S3 bucket %s, err: %s", stackName, s3BucketName, err)
		return err
	}

	if err := s3Client.WaitUntilBucketNotExists(&s3.HeadBucketInput{Bucket: &s3BucketName}); err != nil {
		log.Errorf("[AWS] Skipping stack %s delete because failed to delete S3 bucket %s, err: %s", stackName, s3BucketName, err)
		return err
	}

	return nil
}

func deleteVolumes(ec2Clients map[string]ec2Client, volumes []*types.Disk) []error {
	regionVolumes := map[string][]*types.Disk{}
	for _, vol := range volumes {
		if vol.CloudType == types.AWS {
			regionVolumes[vol.Region] = append(regionVolumes[vol.Region], vol)
		}
	}
	log.Debugf("[AWS] Delete volumes: %v", regionVolumes)

	wg := sync.WaitGroup{}
	wg.Add(len(regionVolumes))
	errChan := make(chan error)

	for r, v := range regionVolumes {
		go func(ec2Client ec2Client, region string, volumes []*types.Disk) {
			defer wg.Done()

			for _, vol := range volumes {
				if ctx.DryRun {
					log.Infof("[AWS] Dry-run set, volume is not deleted: %s:%s, region: %s", vol.Name, vol.ID, region)
				} else {
					log.Infof("[AWS] Initiate delete volume: %s:%s", vol.Name, vol.ID)
					var detachError error
					if vol.State == types.InUse {
						log.Infof("[AWS] Volume %s:%s is in-use, trying to detach", vol.Name, vol.ID)
						if _, detachError = ec2Client.DetachVolume(&ec2.DetachVolumeInput{VolumeId: &vol.ID}); detachError == nil {
							detachError = waitForVolumeUnusedState(ec2Client, vol)
						}
					}

					if detachError != nil {
						log.Infof("[AWS] Skip volume %s:%s as it can not be detached by [%s].", vol.Name, vol.ID, detachError)
						continue
					}
					log.Infof("[AWS] Delete volume: %s:%s", vol.Name, vol.ID)
					if _, err := ec2Client.DeleteVolume(&ec2.DeleteVolumeInput{VolumeId: &vol.ID}); err != nil {
						errChan <- err
					}
				}
			}
		}(ec2Clients[r], r, v)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	return errs
}

func waitForVolumeUnusedState(ec2Client ec2Client, vol *types.Disk) error {
	log.Infof("[AWS] Waiting for Volume %s:%s 'available' state...", vol.Name, vol.ID)
	//Polling state max 10 times with 1 sec interval
	var counter int = 0
	d, e := getDisk(ec2Client, vol.ID)
	for e == nil && d.State != types.Unused && counter < 10 {
		time.Sleep(1 * time.Second)
		d, e = getDisk(ec2Client, vol.ID)
		counter++
	}
	if e != nil {
		return errors.New(fmt.Sprintf("Detach verification failed: %s", e))
	} else if d.State != types.Unused {
		return errors.New(fmt.Sprintf("Detach verification failed, disk state is: %s", d.State))
	} else {
		log.Infof("[AWS] Volume %s:%s is detached so it can be deleted.", vol.Name, vol.ID)
	}
	return nil
}

func deleteImages(ec2Clients map[string]ec2Client, images []*types.Image) []error {
	regionImages := map[string][]*types.Image{}
	for _, image := range images {
		if image.CloudType == types.AWS {
			regionImages[image.Region] = append(regionImages[image.Region], image)
		}
	}
	log.Debugf("[AWS] Delete images: %v", regionImages)

	wg := sync.WaitGroup{}
	wg.Add(len(regionImages))
	errChan := make(chan error)
	sem := make(chan bool, 10)

	for r, v := range regionImages {
		go func(ec2Client ec2Client, region string, images []*types.Image) {
			sem <- true
			defer func() {
				wg.Done()
				<-sem
			}()

			for _, image := range images {
				if ctx.DryRun {
					log.Infof("[AWS] Dry-run set, image is not deleted: %s:%s, region: %s", image.Name, image.ID, region)
				} else {
					log.Infof("[AWS] Delete image: %s:%s", image.Name, image.ID)
					if _, err := ec2Client.DeregisterImage(&ec2.DeregisterImageInput{ImageId: &image.ID}); err != nil {
						log.Errorf("[AWS] Unable to delete image: %s because: %s", image.ID, err.Error())
						errChan <- errors.New(image.ID)
					}
				}
			}
		}(ec2Clients[r], r, v)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	return errs
}

func (p awsProvider) GetAccesses() ([]*types.Access, error) {
	log.Debug("[AWS] Fetching users")
	return getAccesses(p.iamClient)
}

type ec2Client interface {
	DescribeRegions(*ec2.DescribeRegionsInput) (*ec2.DescribeRegionsOutput, error)
	DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	DescribeVolumes(input *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error)
	DeleteVolume(input *ec2.DeleteVolumeInput) (*ec2.DeleteVolumeOutput, error)
	DescribeImages(input *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error)
	DeregisterImage(input *ec2.DeregisterImageInput) (*ec2.DeregisterImageOutput, error)
	DetachVolume(input *ec2.DetachVolumeInput) (*ec2.VolumeAttachment, error)
	DescribeVpcEndpoints(input *ec2.DescribeVpcEndpointsInput) (*ec2.DescribeVpcEndpointsOutput, error)
	DeleteVpcEndpoints(input *ec2.DeleteVpcEndpointsInput) (*ec2.DeleteVpcEndpointsOutput, error)
	DeleteVpc(input *ec2.DeleteVpcInput) (*ec2.DeleteVpcOutput, error)
	DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DeleteSubnet(input *ec2.DeleteSubnetInput) (*ec2.DeleteSubnetOutput, error)
	DescribeSecurityGroups(input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error)
	DeleteSecurityGroup(input *ec2.DeleteSecurityGroupInput) (*ec2.DeleteSecurityGroupOutput, error)
}

type cfClient interface {
	DescribeStacks(*cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
	DeleteStack(input *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error)
	DescribeStackResource(input *cloudformation.DescribeStackResourceInput) (*cloudformation.DescribeStackResourceOutput, error)
	DescribeStackResources(input *cloudformation.DescribeStackResourcesInput) (*cloudformation.DescribeStackResourcesOutput, error)
	WaitUntilStackDeleteComplete(input *cloudformation.DescribeStacksInput) error
}

type cloudTrailClient interface {
	LookupEvents(input *cloudtrail.LookupEventsInput) (*cloudtrail.LookupEventsOutput, error)
}

type iamClient interface {
	ListUsers(*iam.ListUsersInput) (*iam.ListUsersOutput, error)
	ListAccessKeys(*iam.ListAccessKeysInput) (*iam.ListAccessKeysOutput, error)
}

type rdsClient interface {
	DescribeDBInstances(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error)
	ListTagsForResource(input *rds.ListTagsForResourceInput) (*rds.ListTagsForResourceOutput, error)
	ModifyDBInstance(input *rds.ModifyDBInstanceInput) (*rds.ModifyDBInstanceOutput, error)
}

type s3Client interface {
	DeleteBucket(input *s3.DeleteBucketInput) (*s3.DeleteBucketOutput, error)
	WaitUntilBucketNotExists(input *s3.HeadBucketInput) error
	DeleteBucketContents(s3BucketName string) error
}

type s3ClientImpl struct {
	s3iface.S3API
}

func (c s3ClientImpl) DeleteBucketContents(s3BucketName string) error {
	iterator := s3manager.NewDeleteListIterator(c.S3API, &s3.ListObjectsInput{Bucket: &s3BucketName})
	return s3manager.NewBatchDeleteWithClient(c.S3API).Delete(aws.BackgroundContext(), iterator)
}

func getInstances(ec2Clients map[string]ec2Client, cloudTrailClients map[string]cloudTrailClient) ([]*types.Instance, error) {
	instChan := make(chan *types.Instance, 5)
	wg := sync.WaitGroup{}
	wg.Add(len(ec2Clients))

	for r, c := range ec2Clients {
		log.Debugf("[AWS] Fetching instances from: %s", r)
		go func(region string, ec2Client ec2Client, cloudTrailClient cloudTrailClient) {
			defer wg.Done()

			instanceResult, e := ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{})
			if e != nil {
				log.Errorf("[AWS] Failed to fetch the instances in region: %s, err: %s", region, e)
				return
			}
			log.Debugf("[AWS] Processing instances (%d): [%s] in region: %s", len(instanceResult.Reservations), instanceResult.Reservations, region)
			for _, res := range instanceResult.Reservations {
				for _, inst := range res.Instances {
					i := newInstance(inst)
					if len(i.Owner) == 0 && i.State == types.Running {
						log.Debugf("[AWS] instance %s does not have an %s tag, check CloudTrail logs", i.Name, ctx.OwnerLabel)
						if iamUser := getIAMUserFromCloudTrail(*inst.InstanceId, cloudTrailClient); iamUser != nil {
							i.Metadata = map[string]string{"IAMUser": *iamUser}
						}
					}
					instChan <- i
				}
			}
		}(r, c, cloudTrailClients[r])
	}

	go func() {
		wg.Wait()
		close(instChan)
	}()

	var instances []*types.Instance
	for inst := range instChan {
		instances = append(instances, inst)
	}
	return instances, nil
}

func getCFStacks(cfClients map[string]cfClient) ([]*types.Stack, error) {
	cfChan := make(chan *types.Stack, 5)
	wg := sync.WaitGroup{}
	wg.Add(len(cfClients))

	for r, c := range cfClients {
		log.Debugf("[AWS] Fetching CloudFormation from: %s", r)
		go func(region string, cfClient cfClient) {
			defer wg.Done()
			nextToken := ""

			for {
				request := &cloudformation.DescribeStacksInput{}
				if nextToken != "" {
					request.SetNextToken(nextToken)
				}
				stackResult, e := cfClient.DescribeStacks(request)
				if e != nil {
					log.Errorf("[AWS] Failed to fetch the CloudFormation stacks in region: %s, err: %s", region, e)
					return
				}
				log.Debugf("[AWS] Processing stacks (%d) in region: %s: [%s]", len(stackResult.Stacks), region, stackResult.Stacks)
				for _, s := range stackResult.Stacks {
					stack := newStack(s, region)
					cfChan <- stack
				}
				if stackResult.NextToken != nil {
					nextToken = *stackResult.NextToken
				} else {
					break
				}
			}
		}(r, c)
	}

	go func() {
		wg.Wait()
		close(cfChan)
	}()

	var stacks []*types.Stack
	for stack := range cfChan {
		stacks = append(stacks, stack)
	}
	return stacks, nil
}

func getImages(ec2Clients map[string]ec2Client) ([]*types.Image, error) {
	imgChan := make(chan *types.Image)
	wg := sync.WaitGroup{}
	wg.Add(len(ec2Clients))

	for r, c := range ec2Clients {
		log.Debugf("[AWS] Fetching images from region: %s", r)
		go func(region string, ec2Client ec2Client) {
			defer wg.Done()

			result, err := ec2Client.DescribeImages(&ec2.DescribeImagesInput{Owners: []*string{&(&types.S{S: "self"}).S}})
			if err != nil {
				log.Errorf("[AWS] Failed to fetch the images in region: %s, err: %s", region, err)
				return
			}
			log.Debugf("[AWS] Processing images (%d): [%s] in region: %s", len(result.Images), result.Images, region)
			for _, img := range result.Images {
				imgChan <- newImage(img, region)
			}

		}(r, c)
	}

	go func() {
		wg.Wait()
		close(imgChan)
	}()

	var images []*types.Image
	for img := range imgChan {
		images = append(images, img)
	}

	return images, nil
}

func getDisk(ec2Client ec2Client, volumeId string) (*types.Disk, error) {
	result, err := ec2Client.DescribeVolumes(&ec2.DescribeVolumesInput{VolumeIds: []*string{&volumeId}})
	if err != nil {
		log.Errorf("[AWS] Failed to fetch the volume, err: %s", err)
		return nil, err
	}
	log.Debugf("[AWS] Processing volumes (%d): [%s]", len(result.Volumes), result.Volumes)

	if len(result.Volumes) == 0 {
		return nil, errors.New(fmt.Sprintf("Volume not found with id '%s'", volumeId))
	}
	return newDisk(result.Volumes[0]), nil

}

func getDisks(ec2Clients map[string]ec2Client) ([]*types.Disk, error) {
	diskChan := make(chan *types.Disk)
	wg := sync.WaitGroup{}
	wg.Add(len(ec2Clients))

	for r, c := range ec2Clients {
		log.Debugf("[AWS] Fetching volumes from region: %s", r)
		go func(region string, ec2Client ec2Client) {
			defer wg.Done()

			result, err := ec2Client.DescribeVolumes(&ec2.DescribeVolumesInput{})
			if err != nil {
				log.Errorf("[AWS] Failed to fetch the volumes in region: %s, err: %s", region, err)
				return
			}
			log.Debugf("[AWS] Processing volumes (%d): [%s] in region: %s", len(result.Volumes), result.Volumes, region)
			for _, vol := range result.Volumes {
				diskChan <- newDisk(vol)
			}

		}(r, c)
	}

	go func() {
		wg.Wait()
		close(diskChan)
	}()

	var disks []*types.Disk
	for d := range diskChan {
		disks = append(disks, d)
	}

	return disks, nil
}

type cloudTrailEvent struct {
	UserIdentity struct {
		T string `json:"type"`
	} `json:"userIdentity"`
}

func getIAMUserFromCloudTrail(resourceID string, cloudTrailClient cloudTrailClient) *string {
	attributes := []*cloudtrail.LookupAttribute{{
		AttributeKey:   &(&types.S{S: "ResourceName"}).S,
		AttributeValue: &(&types.S{S: resourceID}).S,
	}}
	events, err := cloudTrailClient.LookupEvents(&cloudtrail.LookupEventsInput{LookupAttributes: attributes})
	if err != nil {
		log.Errorf("[AWS] Failed to retrieve CloudTrail events for resource %s, err: %s", resourceID, err.Error())
		return nil
	}
	for _, event := range events.Events {
		var eventSource cloudTrailEvent
		if err := json.Unmarshal([]byte(*event.CloudTrailEvent), &eventSource); err != nil {
			log.Errorf("[AWS] Failed to unmarshal the CloudTrail event source, err: %s", err.Error())
			return nil
		}
		idType := eventSource.UserIdentity.T
		if idType == "IAMUser" || idType == "AssumedRole" {
			return event.Username
		}
	}
	return nil
}

func getAccesses(iamClient iamClient) ([]*types.Access, error) {
	users, err := iamClient.ListUsers(&iam.ListUsersInput{MaxItems: &(&types.I64{I: 1000}).I})
	if err != nil {
		return nil, err
	}
	log.Debugf("[AWS] Processing users (%d): %s", len(users.Users), users.Users)
	var accesses []*types.Access
	for _, u := range users.Users {
		log.Debugf("[AWS] Fetching access keys for: %s", *u.UserName)
		req := &iam.ListAccessKeysInput{
			UserName: u.UserName,
			MaxItems: &(&types.I64{I: 1000}).I,
		}
		resp, err := iamClient.ListAccessKeys(req)
		if err != nil {
			return nil, err
		}
		log.Debugf("[AWS] Processing access keys (%d): [%s]", len(resp.AccessKeyMetadata), resp.AccessKeyMetadata)
		for _, akm := range resp.AccessKeyMetadata {
			accessKey := *akm.AccessKeyId
			name := accessKey[0:10] + "..."
			if *akm.Status != "Active" {
				log.Debugf("[AWS] Access key is not active: %s", name)
				continue
			}
			accesses = append(accesses, &types.Access{
				CloudType: types.AWS,
				Name:      name,
				Owner:     *akm.UserName,
				Created:   getCreated(akm.CreateDate),
				Tags:      types.Tags{},
			})
		}
	}
	return accesses, nil
}

func getDatabases(rdsClients map[string]rdsClient, cloudTrailClients map[string]cloudTrailClient) ([]*types.Database, error) {
	dbChan := make(chan *types.Database)
	wg := sync.WaitGroup{}
	wg.Add(len(rdsClients))

	for r, c := range rdsClients {
		log.Debugf("[AWS] Fetching RDS instances from: %s", r)
		go func(region string, rdsClient rdsClient, cloudTrailClient cloudTrailClient) {
			defer wg.Done()

			result, err := rdsClient.DescribeDBInstances(&rds.DescribeDBInstancesInput{})
			if err != nil {
				log.Errorf("[AWS] Failed to fetch the RDS instances in region: %s, err: %s", region, err)
				return
			}
			databases := result.DBInstances
			log.Debugf("[AWS] Processing databases (%d): %s", len(databases), databases)
			for _, db := range databases {
				tags, err := rdsClient.ListTagsForResource(&rds.ListTagsForResourceInput{
					ResourceName: db.DBInstanceArn,
				})
				if err != nil {
					log.Debugf("[AWS] Cannot list tags for DB: %s", *db.DBName)
				}
				d := newDatabase(*db, tags)
				if d.State == types.Running && len(d.Owner) == 0 {
					log.Debugf("[AWS] Check CloudTrail for database: %s", d.Name)
					if iamUser := getIAMUserFromCloudTrail(d.Name, cloudTrailClient); iamUser != nil {
						d.Metadata = map[string]string{"IAMUser": *iamUser}
					}
				}
				dbChan <- d
			}

		}(r, c, cloudTrailClients[r])
	}

	go func() {
		wg.Wait()
		close(dbChan)
	}()

	var databases []*types.Database
	for db := range dbChan {
		databases = append(databases, db)
	}
	return databases, nil
}

func getRegions(ec2Client ec2Client) ([]string, error) {
	regionResult, e := ec2Client.DescribeRegions(&ec2.DescribeRegionsInput{})
	if e != nil {
		return nil, e
	}
	log.Debugf("[AWS] Processing regions (%d): [%s]", len(regionResult.Regions), regionResult.Regions)
	regions := make([]string, 0)
	for _, region := range regionResult.Regions {
		regions = append(regions, *region.RegionName)
	}
	log.Infof("[AWS] Available regions: %v", regions)
	return regions, nil
}

func getNameIDPairs(instances []*types.Instance) (instIDNames map[string]string, instanceIDs []*string) {
	instIDNames = map[string]string{}
	for _, inst := range instances {
		instIDNames[inst.ID] = inst.Name
		instanceIDs = append(instanceIDs, &inst.ID)
	}
	return
}

func removeInstance(originalIDs []*string, instanceID *string) (instanceIDs []*string) {
	for _, ID := range originalIDs {
		if *ID != *instanceID {
			instanceIDs = append(instanceIDs, ID)
		}
	}
	return
}

func containsInstanceID(instanceIDs []*string, instanceID *string) bool {
	found := false
	for _, ID := range instanceIDs {
		if *ID == *instanceID {
			found = true
			break
		}
	}
	return found
}

func newIamClient() (*iam.IAM, error) {
	awsSession, err := newSession(nil)
	if err != nil {
		return nil, err
	}
	return iam.New(awsSession), nil
}

func newRdsClient(region string) (*rds.RDS, error) {
	awsSession, err := newSession(func(config *aws.Config) {
		config.Region = &region
	})
	if err != nil {
		return nil, err
	}
	return rds.New(awsSession), nil
}

func newEc2Client(region string) (*ec2.EC2, error) {
	awsSession, err := newSession(func(config *aws.Config) {
		config.Region = &region
	})
	if err != nil {
		return nil, err
	}
	return ec2.New(awsSession), nil
}

func newAutoscalingClient(region string) (*autoscaling.AutoScaling, error) {
	awsSession, err := newSession(func(config *aws.Config) {
		config.Region = &region
	})
	if err != nil {
		return nil, err
	}
	return autoscaling.New(awsSession), nil
}

func newCloudTrailClient(region string) (*cloudtrail.CloudTrail, error) {
	awsSession, err := newSession(func(config *aws.Config) {
		config.Region = &region
	})
	if err != nil {
		return nil, err
	}
	return cloudtrail.New(awsSession), nil
}

func newCloudFormationClient(region string) (*cloudformation.CloudFormation, error) {
	awsSession, err := newSession(func(config *aws.Config) {
		config.Region = &region
	})
	if err != nil {
		return nil, err
	}
	return cloudformation.New(awsSession), nil
}

func newS3Client(region string) (*s3.S3, error) {
	awsSession, err := newSession(func(config *aws.Config) {
		config.Region = &region
	})
	if err != nil {
		return nil, err
	}
	return s3.New(awsSession), nil
}

func newSession(configure func(*aws.Config)) (*session.Session, error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}}
	config := aws.Config{HTTPClient: httpClient}
	if configure != nil {
		configure(&config)
	}
	return session.NewSession(&config)
}

func newInstance(inst *ec2.Instance) *types.Instance {
	tags := getEc2Tags(inst.Tags)
	var name string
	if n, ok := tags["Name"]; ok {
		name = n
	} else {
		name = *inst.InstanceId
	}
	ephemeral := false
	if inst.InstanceLifecycle != nil && strings.ToLower(*inst.InstanceLifecycle) == "spot" {
		ephemeral = true
	}
	return &types.Instance{
		ID:           *inst.InstanceId,
		Name:         name,
		Created:      getCreated(inst.LaunchTime),
		CloudType:    types.AWS,
		Tags:         tags,
		Owner:        tags[ctx.OwnerLabel],
		Region:       getRegionFromAvailabilityZone(inst.Placement.AvailabilityZone),
		InstanceType: *inst.InstanceType,
		State:        getInstanceState(inst),
		Ephemeral:    ephemeral,
	}
}

func newStack(stack *cloudformation.Stack, region string) *types.Stack {
	tags := getCFTags(stack.Tags)
	return &types.Stack{
		ID:        *stack.StackId,
		Name:      *stack.StackName,
		Created:   getCreated(stack.CreationTime),
		CloudType: types.AWS,
		Tags:      tags,
		Owner:     tags[ctx.OwnerLabel],
		Region:    region,
		State:     getCFState(stack),
	}
}

// The low byte represents the state. The high byte is an opaque internal value
// and should be ignored.
//    * 0 : pending
//
//    * 16 : running
//
//    * 32 : shutting-down
//
//    * 48 : terminated
//
//    * 64 : stopping
//
//    * 80 : stopped
func getInstanceState(instance *ec2.Instance) types.State {
	if instance.State == nil {
		return types.Unknown
	}
	switch *instance.State.Code {
	case 0, 16:
		return types.Running
	case 32, 48:
		return types.Terminated
	case 64, 80:
		return types.Stopped
	default:
		return types.Unknown
	}
}

func getCFState(stack *cloudformation.Stack) types.State {
	if stack.StackStatus == nil {
		return types.Unknown
	}
	switch *stack.StackStatus {
	case cloudformation.StackStatusCreateFailed, cloudformation.StackStatusDeleteFailed:
		return types.Failed
	}
	return types.Running
}

func newDisk(volume *ec2.Volume) *types.Disk {
	tags := getEc2Tags(volume.Tags)
	var name string
	if n, ok := tags["Name"]; ok {
		name = n
	} else {
		name = *volume.VolumeId
	}
	return &types.Disk{
		ID:        *volume.VolumeId,
		Name:      name,
		State:     getVolumeState(volume),
		CloudType: types.AWS,
		Region:    getRegionFromAvailabilityZone(volume.AvailabilityZone),
		Created:   getCreated(volume.CreateTime),
		Size:      *volume.Size,
		Type:      *volume.VolumeType,
		Owner:     tags[ctx.OwnerLabel],
		Tags:      tags,
	}
}

func newImage(image *ec2.Image, region string) *types.Image {
	createdAt, err := utils.ConvertTimeLayout("2006-01-02T15:04:05.000Z", *image.CreationDate)
	if err != nil {
		log.Debugf("Failed to parse time: %s for image: %s", *image.CreationDate, *image.ImageId)
		createdAt = time.Now()
	}

	return &types.Image{
		ID:        *image.ImageId,
		Name:      *image.Name,
		CloudType: types.AWS,
		Region:    region,
		Created:   createdAt,
		Tags:      getEc2Tags(image.Tags),
	}
}

func newDatabase(rds rds.DBInstance, tagList *rds.ListTagsForResourceOutput) *types.Database {
	tags := getRdsTags(tagList)
	return &types.Database{
		ID:           *rds.DbiResourceId,
		Name:         *rds.DBInstanceIdentifier,
		Created:      getCreated(rds.InstanceCreateTime),
		Region:       getRegionFromAvailabilityZone(rds.AvailabilityZone),
		InstanceType: *rds.DBInstanceClass,
		State:        getDatabaseState(rds),
		Owner:        tags[ctx.OwnerLabel],
		Tags:         tags,
		CloudType:    types.AWS,
	}
}

func getCreated(created *time.Time) time.Time {
	if created != nil {
		return *created
	} else {
		// creation is in progress
		return time.Now()
	}
}

func getVolumeState(volume *ec2.Volume) types.State {
	switch *volume.State {
	case "available":
		return types.Unused
	case "in-use":
		return types.InUse
	default:
		return types.Unknown
	}
}

func getDatabaseState(rds rds.DBInstance) types.State {
	switch *rds.DBInstanceStatus {
	case "available":
		return types.Running
	case "stopped":
		return types.Stopped
	default:
		return types.Unknown
	}
}

func getEc2Tags(ec2Tags []*ec2.Tag) types.Tags {
	tags := make(types.Tags, 0)
	for _, t := range ec2Tags {
		tags[*t.Key] = *t.Value
	}
	return tags
}

func getCFTags(cfTags []*cloudformation.Tag) types.Tags {
	tags := make(types.Tags, 0)
	for _, t := range cfTags {
		tags[*t.Key] = *t.Value
	}
	return tags
}

func getRdsTags(tagList *rds.ListTagsForResourceOutput) types.Tags {
	tags := make(types.Tags, 0)
	if tagList != nil && len(tagList.TagList) > 0 {
		for _, t := range tagList.TagList {
			tags[*t.Key] = *t.Value
		}
	}
	return tags
}

func getRegionFromAvailabilityZone(az *string) string {
	if az == nil || len(*az) < 1 {
		return ""
	}
	return (*az)[0 : len(*az)-1]
}
