package aws

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"encoding/json"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudtrail"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/rds"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
)

var provider = awsProvider{}

type awsProvider struct {
	ec2Clients         map[string]*ec2.EC2
	autoScalingClients map[string]*autoscaling.AutoScaling
	cloudTrailClient   map[string]*cloudtrail.CloudTrail
	rdsClients         map[string]*rds.RDS
	iamClient          *iam.IAM
}

func init() {
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

func (p awsProvider) TerminateInstances([]*types.Instance) []error {
	return []error{errors.New("[AWS] Termination is not supported")}
}

func (p awsProvider) DeleteDisks(volumes []*types.Disk) []error {
	log.Debug("[AWS] Delete volumes")
	ec2Clients, _ := p.getEc2AndCTClientsByRegion()
	return deleteVolumes(ec2Clients, volumes)
}

func (p awsProvider) StopInstances(instances []*types.Instance) []error {
	log.Debug("[AWS] Stopping instances")
	regionInstances := map[string][]*types.Instance{}
	for _, instance := range instances {
		regionInstances[instance.Region] = append(regionInstances[instance.Region], instance)
	}
	log.Debugf("[AWS] Stopping instances: %s", regionInstances)

	wg := sync.WaitGroup{}
	wg.Add(len(regionInstances))
	errChan := make(chan error)

	for r, i := range regionInstances {
		go func(region string, instances []*types.Instance) {
			defer wg.Done()

			instIDNames, instanceIDs := getNameIDPairs(instances)
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
				compactInstanceName := fmt.Sprintf("%s:%s", *instance.InstanceId, instIDNames[*instance.InstanceId])
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
					log.Errorf("[AWS] Failed to suspend ASG %v for instance %s", instance.AutoScalingGroupName, compactInstanceName)

					// Do not stop the instance if the ASG cannot be suspended otherwise the ASG will terminate the instance
					instanceIDs = removeInstance(instanceIDs, instance.InstanceId)
				}
			}

			log.Debugf("[AWS] Sending request to stop instances in region %s (%d): %v", region, len(instanceIDs), instIDNames)
			if _, err := p.ec2Clients[region].StopInstances(&ec2.StopInstancesInput{InstanceIds: instanceIDs}); err != nil {
				errChan <- err
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

func deleteVolumes(ec2Clients map[string]ec2Client, volumes []*types.Disk) []error {
	regionVolumes := map[string][]*types.Disk{}
	for _, vol := range volumes {
		regionVolumes[vol.Region] = append(regionVolumes[vol.Region], vol)
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

func (p awsProvider) GetAccesses() ([]*types.Access, error) {
	log.Debug("[AWS] Fetching users")
	return getAccesses(p.iamClient)
}

type ec2Client interface {
	DescribeRegions(*ec2.DescribeRegionsInput) (*ec2.DescribeRegionsOutput, error)
	DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	DescribeVolumes(input *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error)
	DeleteVolume(input *ec2.DeleteVolumeInput) (*ec2.DeleteVolumeOutput, error)
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
						log.Debugf("[AWS] instance %s does not have an %s tag, check CloudTrail logs", i.Name, ctx.AwsOwnerLabel)
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
				Created:   *akm.CreateDate,
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
				d := newDatabase(*db)
				if d.State == types.Running {
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
	tags := getTags(inst.Tags)
	var name string
	if n, ok := tags["Name"]; ok {
		name = n
	} else {
		name = *inst.InstanceId
	}
	return &types.Instance{
		ID:           *inst.InstanceId,
		Name:         name,
		Created:      *inst.LaunchTime,
		CloudType:    types.AWS,
		Tags:         tags,
		Owner:        tags[ctx.AwsOwnerLabel],
		Region:       getRegionFromAvailabilityZone(inst.Placement.AvailabilityZone),
		InstanceType: *inst.InstanceType,
		State:        getInstanceState(inst),
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

func newDisk(volume *ec2.Volume) *types.Disk {
	tags := getTags(volume.Tags)
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
		Created:   *volume.CreateTime,
		Size:      *volume.Size,
		Type:      *volume.VolumeType,
	}
}

func newDatabase(rds rds.DBInstance) *types.Database {
	return &types.Database{
		ID:           *rds.DbiResourceId,
		Name:         *rds.DBInstanceIdentifier,
		Created:      *rds.InstanceCreateTime,
		Region:       getRegionFromAvailabilityZone(rds.AvailabilityZone),
		InstanceType: *rds.DBInstanceClass,
		State:        getDatabaseState(rds),
		CloudType:    types.AWS,
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
	default:
		return types.Unknown
	}
}

func getTags(ec2Tags []*ec2.Tag) types.Tags {
	tags := make(types.Tags, 0)
	for _, t := range ec2Tags {
		tags[*t.Key] = *t.Value
	}
	return tags
}

func getRegionFromAvailabilityZone(az *string) string {
	if az == nil || len(*az) < 1 {
		return ""
	}
	return (*az)[0 : len(*az)-1]
}
