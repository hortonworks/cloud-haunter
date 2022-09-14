package aws

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/aws/aws-sdk-go/service/cloudformation"
	elb "github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/hortonworks/cloud-haunter/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudtrail"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/rds"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

const (
	TYPE_NATIVE = "Native"
	TYPE_CF     = "CloudFormation"
	CF_TAG      = "aws:cloudformation:stack-id"

	METADATA_TYPE            = "type"
	METADATA_LOAD_BALANCERS  = "loadBalancers"
	METADATA_INSTANCES       = "instances"
	METADATA_VOLUMES         = "volumes"
	METADATA_SECURITY_GROUPS = "securityGroups"
	METADATA_ELASTIC_IPS     = "elasticIps"
	METADATA_ALARMS          = "alarms"
)

var provider = awsProvider{}

var rateLimiter = rate.NewLimiter(rate.Every(time.Duration(ctx.AwsApiOperationRateLimitIntervalInSeconds)*time.Second), ctx.AwsApiOperationRateLimit)

type awsProvider struct {
	ec2Clients           map[string]*ec2.EC2
	autoScalingClients   map[string]*autoscaling.AutoScaling
	cloudTrailClient     map[string]*cloudtrail.CloudTrail
	cloudFormationClient map[string]*cloudformation.CloudFormation
	rdsClients           map[string]*rds.RDS
	elbClients           map[string]*elb.ELBV2
	cloudWatchClients    map[string]*cloudwatch.CloudWatch
	iamClient            *iam.IAM
	govCloud             bool
}

type ThrottledTransport struct {
	roundTripperWrap http.RoundTripper
	ratelimiter      *rate.Limiter
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
			}, false)
			if err != nil {
				panic("[AWS] Failed to initialize provider, err: " + err.Error())
			}
			log.Info("[AWS] Successfully prepared")
		}
		return provider
	}
}

func (p *awsProvider) init(getRegions func() ([]string, error), govCloud bool) error {
	regions, err := getRegions()
	if err != nil {
		return err
	}

	p.ec2Clients = map[string]*ec2.EC2{}
	p.autoScalingClients = map[string]*autoscaling.AutoScaling{}
	p.rdsClients = map[string]*rds.RDS{}
	p.elbClients = map[string]*elb.ELBV2{}
	p.cloudTrailClient = map[string]*cloudtrail.CloudTrail{}
	p.cloudFormationClient = map[string]*cloudformation.CloudFormation{}
	p.cloudWatchClients = map[string]*cloudwatch.CloudWatch{}
	p.govCloud = govCloud

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

		if elbClient, err := newElbClient(region); err != nil {
			panic(fmt.Sprintf("[AWS] Failed to create ELB client in region %s, err: %s", region, err.Error()))
		} else {
			p.elbClients[region] = elbClient
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

		if cwClient, err := newCloudWatchClient(region); err != nil {
			panic(fmt.Sprintf("[AWS] Failed to create CloudWatch client, err: %s", err.Error()))
		} else {
			p.cloudWatchClients[region] = cwClient
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
	if !p.govCloud {
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
	cfStacks, cfError := getCFStacks(cfClients)
	if cfError != nil {
		return nil, cfError
	}
	log.Debug("[AWS] Fetching native stacks")
	elbClients := p.getElbClientsByRegion()
	ec2Clients, _ := p.getEc2AndCTClientsByRegion()
	cloudWatchClients := p.getCloudWatchClientsByRegion()
	nativeStacks, nativeError := getNativeStacks(ec2Clients, elbClients, cloudWatchClients)
	if nativeError != nil {
		return nil, nativeError
	}
	return append(cfStacks, nativeStacks...), nil
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

func (p awsProvider) getElbClientsByRegion() map[string]elbClient {
	elbClients := map[string]elbClient{}
	for k := range p.elbClients {
		elbClients[k] = p.elbClients[k]
	}
	return elbClients
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

func (p awsProvider) getCloudWatchClientsByRegion() map[string]cloudWatchClient {
	cwClients := map[string]cloudWatchClient{}
	for k := range p.cloudWatchClients {
		cwClients[k] = p.cloudWatchClients[k]
	}
	return cwClients
}

func (p awsProvider) TerminateInstances(*types.InstanceContainer) []error {
	return []error{errors.New("[AWS] Termination is not supported")}
}

func (p awsProvider) TerminateStacks(stacks *types.StackContainer) []error {
	log.Debug("[AWS] Delete CloudFormation stacks")
	cfClients := p.getCFClientsByRegion()
	rdsClients := p.getRdsClientsByRegion()
	ec2Clients, _ := p.getEc2AndCTClientsByRegion()
	elbClients := p.getElbClientsByRegion()
	cloudWatchClients := p.getCloudWatchClientsByRegion()
	return deleteStacks(cfClients, rdsClients, ec2Clients, elbClients, cloudWatchClients, stacks.Get(types.AWS))
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

func deleteStacks(cfClients map[string]cfClient, rdsClients map[string]rdsClient, ec2Clients map[string]ec2Client, elbClients map[string]elbClient, cloudWatchClients map[string]cloudWatchClient, stacks []*types.Stack) []error {
	regionStacks := map[string][]*types.Stack{}
	for _, stack := range stacks {
		regionStacks[stack.Region] = append(regionStacks[stack.Region], stack)
	}
	stacksJson, _ := json.MarshalIndent(regionStacks, "", "  ")
	log.Debugf("[AWS] Delete stacks: %s", stacksJson)

	wg := sync.WaitGroup{}
	errChan := make(chan error)

	for r, stacks := range regionStacks {
		wg.Add(len(stacks))

		for _, s := range stacks {
			go func(cfClient cfClient, rdsClient rdsClient, ec2Client ec2Client, elbClient elbClient, cloudWatchClient cloudWatchClient, region string, stack *types.Stack) {
				defer wg.Done()

				if ctx.DryRun {
					log.Infof("[AWS] Dry-run set, stack is not deleted: %s, region: %s", stack.Name, region)
				} else {
					switch stack.Metadata[METADATA_TYPE] {
					case TYPE_NATIVE:
						err := deleteNativeStack(ec2Client, elbClient, cloudWatchClient, region, stack)
						if err != nil {
							errChan <- err
						}
					case TYPE_CF:
						err := deleteCfStack(cfClient, rdsClient, ec2Client, elbClient, region, stack)
						if err != nil {
							errChan <- err
						}
					default:
						errChan <- fmt.Errorf("[AWS] Stack type %s (of stack %s in region %s) delete is not implemented", stack.Metadata[METADATA_TYPE], stack.ID, region)
					}
				}
			}(cfClients[r], rdsClients[r], ec2Clients[r], elbClients[r], cloudWatchClients[r], r, s)
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

func deleteNativeStack(ec2Client ec2Client, elbClient elbClient, cloudWatchClient cloudWatchClient, region string, stack *types.Stack) error {
	log.Infof("[AWS] Delete Native stack: %s, region: %s", stack.Name, region)

	instanceIds := getResourceList(stack.Metadata[METADATA_INSTANCES])

	_, err := ec2Client.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice(instanceIds),
	})
	if err != nil {
		log.Errorf("[AWS] Failed to terminate instances: %s in stack: %s, err: %s", stack.Metadata[METADATA_INSTANCES], stack.ID, err)
		return err
	}

	err = ec2Client.WaitUntilInstanceTerminated(&ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice(instanceIds),
	})
	if err != nil {
		log.Errorf("[AWS] Failed to wait for terminated instances: %s in stack: %s, err: %s", stack.Metadata[METADATA_INSTANCES], stack.ID, err)
		return err
	}

	wg := sync.WaitGroup{}
	errChan := make(chan error)

	volumes := getResourceList(stack.Metadata[METADATA_VOLUMES])
	wg.Add(len(volumes))

	for _, v := range volumes {
		go func(volume string) {
			defer wg.Done()

			_, err := ec2Client.DeleteVolume(&ec2.DeleteVolumeInput{
				VolumeId: aws.String(volume),
			})
			if err != nil {
				log.Errorf("[AWS] Failed to delete volume: %s in stack: %s, err: %s", volume, stack.ID, err)
				errChan <- err
			}
		}(v)
	}

	securityGroups := getResourceList(stack.Metadata[METADATA_SECURITY_GROUPS])
	wg.Add(len(securityGroups))

	for _, sg := range securityGroups {
		go func(securityGroup string) {
			defer wg.Done()

			_, err := ec2Client.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
				GroupId: aws.String(securityGroup),
			})
			if err != nil {
				log.Errorf("[AWS] Failed to delete security group: %s in stack: %s, err: %s", securityGroup, stack.ID, err)
				errChan <- err
			}
		}(sg)
	}

	loadBalancers := getResourceList(stack.Metadata[METADATA_LOAD_BALANCERS])
	wg.Add(len(loadBalancers))

	for _, lb := range loadBalancers {
		go func(loadBalancerArn string) {
			defer wg.Done()

			err := deleteLoadBalancer(elbClient, stack.ID, loadBalancerArn, region)
			if err != nil {
				errChan <- err
			}
		}(lb)
	}

	elasticIps := getResourceList(stack.Metadata[METADATA_ELASTIC_IPS])
	wg.Add(len(elasticIps))

	for _, i := range elasticIps {
		go func(ip string) {
			defer wg.Done()

			_, err := ec2Client.ReleaseAddress(&ec2.ReleaseAddressInput{
				AllocationId: aws.String(ip),
			})
			if err != nil {
				log.Errorf("[AWS] Failed to release elastic IP: %s in stack: %s, err: %s", ip, stack.ID, err)
				errChan <- err
			}
		}(i)
	}

	alarms := getResourceList(stack.Metadata[METADATA_ALARMS])
	wg.Add(1)

	go func() {
		defer wg.Done()

		_, err := cloudWatchClient.DeleteAlarms(&cloudwatch.DeleteAlarmsInput{
			AlarmNames: aws.StringSlice(alarms),
		})
		if err != nil {
			log.Errorf("[AWS] Failed to delete alarms: %s in stack: %s, err: %s", stack.Metadata[METADATA_ALARMS], stack.ID, err)
			errChan <- err
		}
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		close(errChan)
		return err
	}

	log.Infof("[AWS] Delete Native stack: %s, region: %s was successful", stack.Name, region)
	return nil
}

func getResourceList(resources string) []string {
	if resources != "" {
		return strings.Split(resources, ",")
	}
	return []string{}
}

func deleteCfStack(cfClient cfClient, rdsClient rdsClient, ec2Client ec2Client, elbClient elbClient, region string, stack *types.Stack) error {
	stackResources, err := cfClient.DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
		StackName: &stack.Name,
	})
	if err != nil {
		log.Errorf("[AWS] Failed to fetch the CloudFormation stack resources for stack: %s, err: %s", stack.Name, err)
		return err
	}
	var resourceErr error
	retainResources := []*string{}
	for _, stackResource := range stackResources.StackResources {
		if *stackResource.PhysicalResourceId == nil {
			log.Error("[AWS] Missing Physical ID in the CloudFormation stack resources for stack: %s", stack.Name)
			break
		}
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
		case "AWS::ElasticLoadBalancingV2::LoadBalancer":
			resourceErr = deleteLoadBalancer(elbClient, stack.Name, *stackResource.PhysicalResourceId, region)
		}

		if resourceErr != nil {
			log.Warnf("[AWS] Failed to delete resource of CloudFormation stack: %s, err: %s", stack.Name, resourceErr)
		}
	}

	log.Infof("[AWS] Delete CloudFormation stack: %s, region: %s", stack.Name, region)
	deleteStackInput := &cloudformation.DeleteStackInput{
		StackName: &stack.Name,
	}
	if stack.State == types.Failed {
		deleteStackInput.SetRetainResources(retainResources)
	}
	if _, err := cfClient.DeleteStack(deleteStackInput); err != nil {
		log.Errorf("[AWS] Failed to start deletion of CloudFormation stack: %s, err: %s", stack.Name, err)
		return err
	}
	describeStacksInput := &cloudformation.DescribeStacksInput{StackName: &stack.Name}
	if err := cfClient.WaitUntilStackDeleteComplete(describeStacksInput); err != nil {
		if stacks, describeErr := cfClient.DescribeStacks(describeStacksInput); describeErr == nil && stacks.Stacks != nil {
			cfStack := *stacks.Stacks[0]
			err = fmt.Errorf("[AWS] Cloudformation stack: %s, region: %s, is in status %s with reason: %s", *cfStack.StackName, region, *cfStack.StackStatus, *cfStack.StackStatusReason)
		} else {
			log.Warnf("[AWS] Failed to describe stack: %s, region: %s for status, err: %s", stack.Name, region, describeErr)
		}
		log.Errorf("[AWS] Failed to wait for delete complete of CloudFormation stack: %s, err: %s", stack.Name, err)
		return err
	}
	log.Infof("[AWS] Delete CloudFormation stack: %s, region: %s was successful", stack.Name, region)
	return nil
}

func disableDeleteProtection(rdsClient rdsClient, stackName string, dbInstanceId string, region string) (bool, error) {
	log.Infof("[AWS] Disabling DeletionProtection for DB instance: %s in stack: %s, region: %s", dbInstanceId, stackName, region)
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
		log.Errorf("[AWS] Failed to disable DeletionProtection on db instance: %s in stack: %s, err: %s", dbInstanceId, stackName, err)
	}
	return dbExists, err
}

/**
 * CloudFormation is not able to delete a VPC if it has dependencies, so we have to delete them manually.
 */
func deleteVpcWithDependencies(ec2Client ec2Client, stackName string, vpcId string, region string) error {
	log.Infof("[AWS] Deleting endpoints of VPC: %s in stack: %s, region: %s", vpcId, stackName, region)

	vpcIdFilter := &ec2.Filter{}
	vpcIdFilter.SetName("vpc-id")
	vpcIdFilter.SetValues([]*string{&vpcId})
	vpcEndpoints, err := ec2Client.DescribeVpcEndpoints(&ec2.DescribeVpcEndpointsInput{
		Filters: []*ec2.Filter{vpcIdFilter},
	})
	if err != nil {
		log.Errorf("[AWS] Failed to list endpoints of VPC: %s in stack: %s, err: %s", vpcId, stackName, err)
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
			log.Errorf("[AWS] Failed to delete endpoints of VPC: %s in stack: %s, err: %s", vpcId, stackName, err)
			return err
		}
	}

	log.Infof("[AWS] Deleting subnets of VPC: %s in stack: %s, region: %s", vpcId, stackName, region)

	subnets, err := ec2Client.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{vpcIdFilter},
	})
	if err != nil {
		log.Errorf("[AWS] Failed to list subnets of VPC: %s in stack: %s, err: %s", vpcId, stackName, err)
		return err
	}
	for _, subnet := range subnets.Subnets {
		_, err = ec2Client.DeleteSubnet(&ec2.DeleteSubnetInput{
			SubnetId: subnet.SubnetId,
		})
		if err != nil {
			log.Errorf("[AWS] Failed to delete subnet: %s in stack: %s, err: %s", *subnet.SubnetId, stackName, err)
			return err
		}
	}

	log.Infof("[AWS] Deleting security groups of VPC: %s in stack: %s, region: %s", vpcId, stackName, region)

	securityGroups, err := ec2Client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{vpcIdFilter},
	})
	if err != nil {
		log.Errorf("[AWS] Failed to list security groups of VPC: %s in stack: %s, err: %s", vpcId, stackName, err)
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
			log.Errorf("[AWS] Failed to delete security group: %s in stack: %s, err: %s", *securityGroup.GroupId, stackName, err)
			return err
		}
	}

	log.Infof("[AWS] Deleting VPC: %s in stack: %s, region: %s", vpcId, stackName, region)

	_, err = ec2Client.DeleteVpc(&ec2.DeleteVpcInput{
		VpcId: &vpcId,
	})
	if err != nil && err.(awserr.Error).Code() != "InvalidVpcID.NotFound" {
		log.Errorf("[AWS] Failed to delete VPC: %s in stack: %s, err: %s", vpcId, stackName, err)
		return err
	}

	return nil
}

func deleteLoadBalancer(elbClient elbClient, stackName string, elbArn string, region string) error {
	log.Infof("[AWS] Deleting load balancer: %s in stack: %s, region: %s", elbArn, stackName, region)

	_, err := elbClient.DeleteLoadBalancer(&elb.DeleteLoadBalancerInput{
		LoadBalancerArn: aws.String(elbArn),
	})
	if err != nil {
		log.Errorf("[AWS] Failed to delete load balancer: %s in stack: %s, err: %s", elbArn, stackName, err)
	}
	return err
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

func (p awsProvider) GetAlerts() ([]*types.Alert, error) {
	log.Debug("[AWS] Fetch alerts")
	cloudWatchClients := p.getCloudWatchClientsByRegion()
	ec2Clients, _ := p.getEc2AndCTClientsByRegion()
	return getAlerts(cloudWatchClients, ec2Clients)
}

func (p awsProvider) DeleteAlerts(alertContainer *types.AlertContainer) []error {
	log.Debug("[AWS] Delete alerts")
	cloudWatchClients := p.getCloudWatchClientsByRegion()
	return deleteAlerts(cloudWatchClients, alertContainer.Get(types.AWS))
}

func (p awsProvider) GetStorages() ([]*types.Storage, error) {
	return nil, errors.New("[AWS] Getting storages is not supported yet")
}

func (p awsProvider) CleanupStorages(storageContainer *types.StorageContainer, retentionDays int) []error {
	return []error{errors.New("[AWS] Cleanup storages is not supported yet")}
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
	TerminateInstances(input *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error)
	WaitUntilInstanceTerminated(input *ec2.DescribeInstancesInput) error
	DescribeAddresses(input *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error)
	ReleaseAddress(input *ec2.ReleaseAddressInput) (*ec2.ReleaseAddressOutput, error)
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

type elbClient interface {
	DescribeLoadBalancers(input *elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error)
	DescribeTags(input *elb.DescribeTagsInput) (*elb.DescribeTagsOutput, error)
	DeleteLoadBalancer(input *elb.DeleteLoadBalancerInput) (*elb.DeleteLoadBalancerOutput, error)
}

type cloudWatchClient interface {
	DescribeAlarms(input *cloudwatch.DescribeAlarmsInput) (*cloudwatch.DescribeAlarmsOutput, error)
	DeleteAlarms(input *cloudwatch.DeleteAlarmsInput) (*cloudwatch.DeleteAlarmsOutput, error)
}

func getInstances(ec2Clients map[string]ec2Client, cloudTrailClients map[string]cloudTrailClient) ([]*types.Instance, error) {
	instChan := make(chan *types.Instance, 5)
	wg := sync.WaitGroup{}
	wg.Add(len(ec2Clients))

	for r, c := range ec2Clients {
		log.Debugf("[AWS] Fetching instances from: %s", r)
		go func(region string, ec2Client ec2Client, cloudTrailClient cloudTrailClient) {
			defer wg.Done()

			request := &ec2.DescribeInstancesInput{}
			for {
				instanceResult, e := ec2Client.DescribeInstances(request)
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
				if instanceResult.NextToken != nil {
					request.SetNextToken(*instanceResult.NextToken)
				} else {
					break
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
	sort.Slice(stacks, func(p, q int) bool {
		return stacks[p].Created.After(stacks[q].Created)
	})
	return stacks, nil
}

type AwsNativeStack struct {
	ID             string
	Region         string
	LoadBalancers  []string
	Instances      []string
	Volumes        []string
	SecurityGroups []string
	ElasticIps     []string
	Alarms         []string
	Created        time.Time
	Owner          string
	State          types.State
	Tags           types.Tags
}

func getNativeStacks(ec2Clients map[string]ec2Client, elbClients map[string]elbClient, cloudWatchClients map[string]cloudWatchClient) ([]*types.Stack, error) {
	stackChan := make(chan AwsNativeStack, 5)
	wg := sync.WaitGroup{}
	wg.Add(len(ec2Clients))

	for r, c := range ec2Clients {
		log.Debugf("[AWS] Fetching resources and assembling native stacks from region: %s", r)
		go func(region string, ec2Client ec2Client, elbClient elbClient, cloudWatchClient cloudWatchClient) {
			defer wg.Done()

			elbRequest := &elb.DescribeLoadBalancersInput{}
			loadBalancersByGroup := map[string][]*elb.LoadBalancer{}
			for {
				loadBalancersOutput, err := elbClient.DescribeLoadBalancers(elbRequest)
				if err != nil {
					log.Errorf("[AWS] Failed to fetch load balancers in region %s, err: %s", region, err)
					return
				}
				for _, loadBalancer := range loadBalancersOutput.LoadBalancers {
					tagsResponse, err := elbClient.DescribeTags(&elb.DescribeTagsInput{ResourceArns: []*string{loadBalancer.LoadBalancerArn}})
					if err != nil {
						log.Errorf("[AWS] Failed to fetch tags of load balancer %s in region %s", *loadBalancer.LoadBalancerName, region)
						continue
					}
					for _, tagDescription := range tagsResponse.TagDescriptions {
						for _, tag := range tagDescription.Tags {
							if *tag.Key == ctx.ResourceGroupingLabel {
								loadBalancersByGroup[*tag.Value] = append(loadBalancersByGroup[*tag.Value], loadBalancer)
								break
							}
						}
					}
				}
				if loadBalancersOutput.NextMarker != nil {
					elbRequest.SetMarker(*loadBalancersOutput.NextMarker)
				} else {
					break
				}
			}

			elasticIpsByGroup := map[string][]*ec2.Address{}
			elasticIpsOutput, err := ec2Client.DescribeAddresses(&ec2.DescribeAddressesInput{})
			if err != nil {
				log.Errorf("[AWS] Failed to fetch elastic IPs in region %s, err: %s", region, err)
				return
			}
			for _, ip := range elasticIpsOutput.Addresses {
				tags := getEc2Tags(ip.Tags)
				if _, ok := tags[CF_TAG]; ok {
					log.Debugf("[AWS] Skipping elastic IP %s in region %s for native stack assembly, as it has a CF tag", *ip.PublicIp, region)
					continue
				}
				if group, ok := tags[ctx.ResourceGroupingLabel]; ok {
					elasticIpsByGroup[group] = append(elasticIpsByGroup[group], ip)
				}
			}

			securityGroupsRequest := &ec2.DescribeSecurityGroupsInput{}
			securityGroupsByGroup := map[string][]*ec2.SecurityGroup{}
			for {
				securityGroupsOutput, err := ec2Client.DescribeSecurityGroups(securityGroupsRequest)
				if err != nil {
					log.Errorf("[AWS] Failed to fetch security groups in region %s, err: %s", region, err)
					return
				}
				for _, sg := range securityGroupsOutput.SecurityGroups {
					tags := getEc2Tags(sg.Tags)
					if _, ok := tags[CF_TAG]; ok {
						log.Debugf("[AWS] Skipping security group %s in region %s for native stack assembly, as it has a CF tag", *sg.GroupName, region)
						continue
					}
					if group, ok := tags[ctx.ResourceGroupingLabel]; ok {
						securityGroupsByGroup[group] = append(securityGroupsByGroup[group], sg)
					}
				}
				if securityGroupsOutput.NextToken != nil {
					securityGroupsRequest.SetNextToken(*securityGroupsOutput.NextToken)
				} else {
					break
				}
			}

			alarmsByInstanceId := map[string]*cloudwatch.MetricAlarm{}
			alarmRequest := &cloudwatch.DescribeAlarmsInput{}
			for {
				output, err := cloudWatchClient.DescribeAlarms(alarmRequest)
				if err != nil {
					log.Errorf("[AWS] Failed to fetch the CloudWatch alarms in region: %s, err: %s", region, err)
					return
				}
				for _, alarm := range output.MetricAlarms {
					for _, d := range alarm.Dimensions {
						if *d.Name == "InstanceId" {
							alarmsByInstanceId[*d.Value] = alarm
						}
					}
				}
				if output.NextToken != nil {
					alarmRequest.SetNextToken(*output.NextToken)
				} else {
					break
				}
			}

			ec2Request := &ec2.DescribeInstancesInput{}
			awsInstances := []*ec2.Instance{}
			for {
				instancesOutput, err := ec2Client.DescribeInstances(ec2Request)
				if err != nil {
					log.Errorf("[AWS] Failed to fetch the instances in region: %s, err: %s", region, err)
					return
				}
				for _, r := range instancesOutput.Reservations {
					awsInstances = append(awsInstances, r.Instances...)
				}
				if instancesOutput.NextToken != nil {
					ec2Request.SetNextToken(*instancesOutput.NextToken)
				} else {
					break
				}
			}

			nativeStacks := map[string]*AwsNativeStack{}
			for _, awsInstance := range awsInstances {
				instance := newInstance(awsInstance)
				if _, ok := instance.Tags[CF_TAG]; ok {
					log.Debugf("[AWS] Skipping instance %s in region %s for native stack assembly, as it has a CF tag", instance.ID, region)
					continue
				}
				group, ok := instance.Tags[ctx.ResourceGroupingLabel]
				if !ok {
					log.Warnf("[AWS] Failed to find stack for instance %s with grouping tag %s in region %s", instance.ID, ctx.ResourceGroupingLabel, region)
					continue
				}

				if _, ok := nativeStacks[group]; !ok {
					log.Debugf("[AWS] Creating stack for instance %s", instance.ID)
					newStack := &AwsNativeStack{
						ID:             group,
						Region:         region,
						LoadBalancers:  []string{},
						Instances:      []string{},
						SecurityGroups: []string{},
						ElasticIps:     []string{},
						Alarms:         []string{},
						Created:        instance.Created,
						Owner:          instance.Owner,
						State:          instance.State,
						Tags:           instance.Tags,
					}

					if loadBalancers, ok := loadBalancersByGroup[group]; ok {
						for _, loadBalancer := range loadBalancers {
							newStack.LoadBalancers = append(newStack.LoadBalancers, *loadBalancer.LoadBalancerArn)
						}
						delete(loadBalancersByGroup, group)
					}
					if elasticIps, ok := elasticIpsByGroup[group]; ok {
						for _, ip := range elasticIps {
							newStack.ElasticIps = append(newStack.ElasticIps, *ip.AllocationId)
						}
						delete(elasticIpsByGroup, group)
					}
					if securityGroups, ok := securityGroupsByGroup[group]; ok {
						for _, securityGroup := range securityGroups {
							newStack.SecurityGroups = append(newStack.SecurityGroups, *securityGroup.GroupId)
						}
						delete(securityGroupsByGroup, group)
					}

					nativeStacks[group] = newStack
				}
				nativeStack := nativeStacks[group]
				nativeStack.Instances = append(nativeStack.Instances, instance.ID)

				if alarm, ok := alarmsByInstanceId[instance.ID]; ok {
					nativeStack.Alarms = append(nativeStack.Alarms, *alarm.AlarmName)
				}

				for key, value := range instance.Tags {
					nativeStack.Tags[key] = value
				}

				if instance.State != nativeStack.State {
					nativeStack.State = types.Unknown
				}

				if instance.Created.Before(nativeStack.Created) {
					nativeStack.Created = instance.Created
				}

				for _, blockDeviceMapping := range awsInstance.BlockDeviceMappings {
					if !*blockDeviceMapping.Ebs.DeleteOnTermination {
						nativeStack.Volumes = append(nativeStack.Volumes, *blockDeviceMapping.Ebs.VolumeId)
					}
				}
			}

			if len(loadBalancersByGroup) != 0 {
				for group, loadBalancers := range loadBalancersByGroup {
					lbNames := []string{}
					for _, lb := range loadBalancers {
						lbNames = append(lbNames, *lb.LoadBalancerName)
					}
					log.Warnf("[AWS] Loadbalancers %s with group %s does not have a stack", strings.Join(lbNames, ","), group)
				}
			}

			if len(elasticIpsByGroup) != 0 {
				for group, ips := range elasticIpsByGroup {
					ipAddresses := []string{}
					for _, ip := range ips {
						ipAddresses = append(ipAddresses, *ip.PublicIp)
					}
					log.Warnf("[AWS] Elastic IPs %s with group %s does not have a stack", strings.Join(ipAddresses, ","), group)
				}
			}

			if len(securityGroupsByGroup) != 0 {
				for group, securityGroups := range securityGroupsByGroup {
					securityGroupIds := []string{}
					for _, sg := range securityGroups {
						securityGroupIds = append(securityGroupIds, *sg.GroupId)
					}
					log.Warnf("[AWS] Security groups %s with group %s does not have a stack", strings.Join(securityGroupIds, ","), group)
				}
			}

			for _, nativeStack := range nativeStacks {
				stackChan <- *nativeStack
			}
		}(r, c, elbClients[r], cloudWatchClients[r])
	}

	go func() {
		wg.Wait()
		close(stackChan)
	}()

	var stacks []*types.Stack
	for awsStack := range stackChan {
		stacks = append(stacks, &types.Stack{
			ID:        awsStack.ID,
			Name:      awsStack.ID,
			Created:   awsStack.Created,
			CloudType: types.AWS,
			Tags:      awsStack.Tags,
			Owner:     awsStack.Owner,
			Region:    awsStack.Region,
			State:     awsStack.State,
			Metadata: map[string]string{
				METADATA_TYPE:            TYPE_NATIVE,
				METADATA_LOAD_BALANCERS:  strings.Join(awsStack.LoadBalancers, ","),
				METADATA_INSTANCES:       strings.Join(awsStack.Instances, ","),
				METADATA_VOLUMES:         strings.Join(awsStack.Volumes, ","),
				METADATA_SECURITY_GROUPS: strings.Join(awsStack.SecurityGroups, ","),
				METADATA_ELASTIC_IPS:     strings.Join(awsStack.ElasticIps, ","),
				METADATA_ALARMS:          strings.Join(awsStack.Alarms, ","),
			},
		})
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

func getAlerts(cloudWatchClients map[string]cloudWatchClient, ec2Clients map[string]ec2Client) ([]*types.Alert, error) {
	alertChan := make(chan *types.Alert)
	wg := sync.WaitGroup{}
	wg.Add(len(cloudWatchClients))

	for r, c := range cloudWatchClients {
		go func(region string, cloudWatchClient cloudWatchClient, ec2Client ec2Client) {
			defer wg.Done()

			input := &cloudwatch.DescribeAlarmsInput{}
			for {
				log.Debugf("[AWS] Fetching CloudWatch alerts from: %s", region)
				output, err := cloudWatchClient.DescribeAlarms(input)
				if err != nil {
					log.Errorf("[AWS] Failed to fetch the CloudWatch alarms in region: %s, err: %s", region, err)
					return
				}
				for _, a := range output.MetricAlarms {
					alert := types.Alert{
						ID:        *a.AlarmArn,
						Name:      *a.AlarmName,
						Created:   *a.AlarmConfigurationUpdatedTimestamp,
						CloudType: types.AWS,
						Owner:     "",
						Region:    region,
						State:     getAlarmState(*a),
						Tags:      map[string]string{},
						Metadata:  map[string]string{},
					}
					for _, d := range a.Dimensions {
						alert.Metadata[*d.Name] = *d.Value
					}
					if instanceId, ok := alert.Metadata["InstanceId"]; ok && alert.State == types.Unused {
						// AWS reports alarms of stopped instances the same as of terminated instances, so their instances have to be queried
						log.Debugf("[AWS] Querying instance %s for alert %s", instanceId, alert.ID)
						request := &ec2.DescribeInstancesInput{
							InstanceIds: aws.StringSlice([]string{instanceId}),
						}
						_, err := ec2Client.DescribeInstances(request)
						if err != nil {
							if awsErr, ok := err.(awserr.Error); ok {
								if awsErr.Code() != ec2.UnsuccessfulInstanceCreditSpecificationErrorCodeInvalidInstanceIdNotFound {
									log.Warnf("[AWS] Error received when querying instance %s, err: %s", instanceId, err)
									alert.State = types.Unknown
								}
							} else {
								log.Warnf("[AWS] Error received when querying instance %s, err: %s", instanceId, err)
								alert.State = types.Unknown
							}
						} else {
							alert.State = types.InUse
						}
					}
					alertChan <- &alert
				}
				if output.NextToken != nil {
					input.SetNextToken(*output.NextToken)
				} else {
					break
				}
			}
		}(r, c, ec2Clients[r])
	}

	go func() {
		wg.Wait()
		close(alertChan)
	}()

	var alerts []*types.Alert
	for alert := range alertChan {
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

func deleteAlerts(cloudWatchClients map[string]cloudWatchClient, alerts []*types.Alert) []error {
	log.Infof("[AWS] Deleting %d alerts", len(alerts))

	regionAlerts := map[string][]*types.Alert{}
	for _, alert := range alerts {
		regionAlerts[alert.Region] = append(regionAlerts[alert.Region], alert)
	}

	errChan := make(chan error)
	wg := sync.WaitGroup{}
	wg.Add(len(regionAlerts))

	for r, a := range regionAlerts {
		go func(cloudWatchClient cloudWatchClient, region string, alertsInRegion []*types.Alert) {
			defer wg.Done()

			alertNames := []string{}
			for _, alert := range alerts {
				alertNames = append(alertNames, alert.Name)
			}

			for i := 0; i < len(alertsInRegion); i += ctx.AwsBulkOperationSize {
				endIndex := i + ctx.AwsBulkOperationSize
				if endIndex > len(alertsInRegion) {
					endIndex = len(alertsInRegion)
				}
				currentAlertNames := alertNames[i:endIndex]
				if ctx.DryRun {
					log.Infof("[AWS] Dry-run set, alerts %s were not deleted in region %s", currentAlertNames, region)
				} else {
					log.Infof("[AWS] Deleting alerts %s in region %s", currentAlertNames, region)
					_, err := cloudWatchClient.DeleteAlarms(&cloudwatch.DeleteAlarmsInput{
						AlarmNames: aws.StringSlice(currentAlertNames),
					})
					if err != nil {
						log.Errorf("[AWS] Failed to delete alerts %s in region %s, err: %s", currentAlertNames, region, err)
						errChan <- err
					}
				}
			}
		}(cloudWatchClients[r], r, a)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	return errors
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

func newCloudWatchClient(region string) (*cloudwatch.CloudWatch, error) {
	awsSession, err := newSession(func(config *aws.Config) {
		config.Region = &region
	})
	if err != nil {
		return nil, err
	}
	return cloudwatch.New(awsSession), nil
}

func newElbClient(region string) (*elb.ELBV2, error) {
	awsSession, err := newSession(func(config *aws.Config) {
		config.Region = &region
	})
	if err != nil {
		return nil, err
	}
	return elb.New(awsSession), nil
}

func NewThrottledTransport(rateLimiter *rate.Limiter, transportWrap http.RoundTripper) http.RoundTripper {
	return &ThrottledTransport{
		roundTripperWrap: transportWrap,
		ratelimiter:      rateLimiter,
	}
}

func (c *ThrottledTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	err := c.ratelimiter.Wait(r.Context())
	if err != nil {
		return nil, err
	}
	return c.roundTripperWrap.RoundTrip(r)
}

func newSession(configure func(*aws.Config)) (*session.Session, error) {
	httpClient := &http.Client{
		Transport: NewThrottledTransport(rateLimiter, &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		})}
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
		Metadata:  map[string]string{METADATA_TYPE: TYPE_CF},
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

func getAlarmState(alarm cloudwatch.MetricAlarm) types.State {
	switch *alarm.StateValue {
	case cloudwatch.StateValueOk, cloudwatch.StateValueAlarm:
		return types.InUse
	case cloudwatch.StateValueInsufficientData:
		return types.Unused
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
