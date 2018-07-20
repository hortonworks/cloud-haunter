package aws

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
)

var provider = awsProvider{}

type awsProvider struct {
	ec2Clients         map[string]*ec2.EC2
	autoScalingClients map[string]*autoscaling.AutoScaling
}

func init() {
	ctx.CloudProviders[types.AWS] = func() types.CloudProvider {
		if len(provider.ec2Clients) == 0 {
			log.Infof("[AWS] Trying to prepare")
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
			log.Infof("[AWS] Successfully prepared")
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

	}
	return nil
}

func (p awsProvider) GetInstances() ([]*types.Instance, error) {
	log.Debug("[AWS] Fetching instances")
	ec2Clients := map[string]ec2Client{}
	for k, v := range p.ec2Clients {
		ec2Clients[k] = v
	}
	return getInstances(ec2Clients)
}

func (p awsProvider) TerminateInstances([]*types.Instance) error {
	return errors.New("[AWS] Termination not supported")
}

func (p awsProvider) StopInstances(instances []*types.Instance) error {
	regionInstances := map[string][]*types.Instance{}
	for _, instance := range instances {
		regionInstances[instance.Region] = append(regionInstances[instance.Region], instance)
	}

	numRegions := len(regionInstances)
	wg := sync.WaitGroup{}
	wg.Add(numRegions)
	errChan := make(chan error, numRegions)

	for region, instances := range regionInstances {
		go func(region string, instances []*types.Instance) {
			defer wg.Done()

			var instIdNames = make(map[string]string)
			var instanceIds []*string
			for _, inst := range instances {
				instIdNames[inst.Id] = inst.Name
				instanceIds = append(instanceIds, &inst.Id)
			}

			// We need to suspend the ASGs as it will terminate the stopped instances
			// Instances that are not part of an ASG are not returned
			asgInstances, err := p.autoScalingClients[region].DescribeAutoScalingInstances(&autoscaling.DescribeAutoScalingInstancesInput{
				InstanceIds: instanceIds,
			})
			if err != nil {
				log.Errorf("[AWS] Failed to fetch the ASG instances in region: %s, err: %s", region, err)
				return
			}
			for _, instance := range asgInstances.AutoScalingInstances {
				asgName := instance.AutoScalingGroupName
				compactInstanceName := fmt.Sprintf("%s:%s", *instance.InstanceId, instIdNames[*instance.InstanceId])
				log.Infof("[AWS] The following instance is in an ASG and will be suspended in region %s: %s", region, compactInstanceName)

				if _, err := p.autoScalingClients[region].SuspendProcesses(&autoscaling.ScalingProcessQuery{AutoScalingGroupName: asgName}); err != nil {
					log.Errorf("[AWS] Failed to suspend ASG %s for instance %s", asgName, compactInstanceName)

					// Do not stop the instance if the ASG cannot be suspended otherwise the ASG will terminate the instance
					var tempInstanceIds []*string
					for _, instId := range instanceIds {
						if *instId == *instance.InstanceId {
							continue
						}
						tempInstanceIds = append(tempInstanceIds, instId)
					}
					instanceIds = tempInstanceIds
				}
			}

			log.Infof("[AWS] Sending request to stop instances in region %s (%d): %v", region, len(instanceIds), instIdNames)
			if _, err := p.ec2Clients[region].StopInstances(&ec2.StopInstancesInput{InstanceIds: instanceIds}); err != nil {
				errChan <- err
			}

		}(region, instances)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errorMessage = ""
	for err := range errChan {
		errorMessage += err.Error() + " "
	}
	if len(errorMessage) > 0 {
		return errors.New(errorMessage)
	}

	return nil
}

func (p awsProvider) GetAccesses() ([]*types.Access, error) {
	log.Debug("[AWS] Fetching users")
	iamClient, err := newIamClient()
	if err != nil {
		return nil, err
	}
	return getAccesses(iamClient)
}

type ec2Client interface {
	DescribeRegions(*ec2.DescribeRegionsInput) (*ec2.DescribeRegionsOutput, error)
	DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
}

type iamClient interface {
	ListUsers(*iam.ListUsersInput) (*iam.ListUsersOutput, error)
	ListAccessKeys(*iam.ListAccessKeysInput) (*iam.ListAccessKeysOutput, error)
}

func getInstances(ec2Clients map[string]ec2Client) ([]*types.Instance, error) {
	instChan := make(chan *types.Instance, 5)
	wg := sync.WaitGroup{}
	wg.Add(len(ec2Clients))

	for r, c := range ec2Clients {
		log.Debugf("[AWS] Fetching instances from: %s", r)
		go func(region string, ec2Client ec2Client) {
			defer wg.Done()

			instanceResult, e := ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{})
			if e != nil {
				log.Errorf("[AWS] Failed to fetch the instances in region: %s, err: %s", region, e)
				return
			}
			log.Debugf("[AWS] Processing instances (%d): [%s] in region: %s", len(instanceResult.Reservations), instanceResult.Reservations, region)
			for _, res := range instanceResult.Reservations {
				for _, inst := range res.Instances {
					instChan <- newInstance(inst)
				}
			}
		}(r, c)
	}
	go func() {
		wg.Wait()
		close(instChan)
	}()
	instances := []*types.Instance{}
	for inst := range instChan {
		instances = append(instances, inst)
	}
	return instances, nil
}

func getAccesses(iamClient iamClient) ([]*types.Access, error) {
	users, err := iamClient.ListUsers(&iam.ListUsersInput{MaxItems: &(&types.I64{I: 1000}).I})
	if err != nil {
		return nil, err
	}
	log.Debugf("[AWS] Processing users (%d): [%s]", len(users.Users), users.Users)
	accesses := []*types.Access{}
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

func newIamClient() (*iam.IAM, error) {
	awsSession, err := newSession(nil)
	if err != nil {
		return nil, err
	}
	return iam.New(awsSession), nil
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
		Id:           *inst.InstanceId,
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
func getInstanceState(instance *ec2.Instance) types.InstanceState {
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
