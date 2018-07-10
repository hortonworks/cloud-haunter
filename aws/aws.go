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
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

var provider = awsProvider{}

type awsProvider struct {
	ec2Clients map[string]*ec2.EC2
}

func init() {
	context.CloudProviders[types.AWS] = func() types.CloudProvider {
		if len(provider.ec2Clients) == 0 {
			ec2Client, err := newEc2Client("eu-west-1")
			if err != nil {
				panic("[AWS] Failed to create ec2 client, err: " + err.Error())
			}
			err = provider.init(func() ([]string, error) {
				return getRegions(ec2Client)
			})
			if err != nil {
				panic("[AWS] Failed to fetch regions, err: " + err.Error())
			}
		}
		return provider
	}
}

func (p *awsProvider) init(getRegions func() ([]string, error)) error {
	log.Infof("[AWS] Trying to prepare")
	regions, err := getRegions()
	if err != nil {
		return err
	}
	p.ec2Clients = map[string]*ec2.EC2{}
	for _, region := range regions {
		if client, err := newEc2Client(region); err != nil {
			panic(fmt.Sprintf("[AWS] Failed to create client in region %s, err: %s", region, err.Error()))
		} else {
			p.ec2Clients[region] = client
		}
	}
	log.Infof("[AWS] Successfully prepared")
	return nil
}

func (p awsProvider) GetRunningInstances() ([]*types.Instance, error) {
	ec2Clients := map[string]ec2Client{}
	for k, v := range p.ec2Clients {
		ec2Clients[k] = v
	}
	return getRunningInstances(ec2Clients)
}

func (a awsProvider) TerminateInstances([]*types.Instance) error {
	return errors.New("[AWS] Termination not supported")
}

func (a awsProvider) GetAccesses() ([]*types.Access, error) {
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

func getRunningInstances(ec2Clients map[string]ec2Client) ([]*types.Instance, error) {
	if context.DryRun {
		log.Debug("[AWS] Fetching instanes")
	}
	instChan := make(chan *types.Instance, 5)
	wg := sync.WaitGroup{}
	wg.Add(len(ec2Clients))

	filterName := "instance-state-name"
	filterValue := ec2.InstanceStateNameRunning
	runningFilter := []*ec2.Filter{{Name: &filterName, Values: []*string{&filterValue}}}

	for r, c := range ec2Clients {
		if context.DryRun {
			log.Debugf("[AWS] Fetching instanes from: %s", r)
		}
		go func(region string, ec2Client ec2Client) {
			defer wg.Done()

			instanceResult, e := ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{
				Filters: runningFilter,
			})
			if e != nil {
				log.Errorf("[AWS] Failed to fetch the running instances in region: %s, err: %s", region, e)
				return
			}
			if context.DryRun {
				log.Debugf("[AWS] Processing instances (%d): [%s] in region: %s", len(instanceResult.Reservations), instanceResult.Reservations, region)
			}
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
	if context.DryRun {
		log.Debug("[AWS] Fetching users")
	}
	users, err := iamClient.ListUsers(&iam.ListUsersInput{MaxItems: &(&types.I64{I: 1000}).I})
	if err != nil {
		return nil, err
	}
	if context.DryRun {
		log.Debugf("[AWS] Processing users (%d): [%s]", len(users.Users), users.Users)
	}
	accesses := []*types.Access{}
	for _, u := range users.Users {
		if context.DryRun {
			log.Debugf("[AWS] Fetching access keys for: %s", *u.UserName)
		}
		req := &iam.ListAccessKeysInput{
			UserName: u.UserName,
			MaxItems: &(&types.I64{I: 1000}).I,
		}
		resp, err := iamClient.ListAccessKeys(req)
		if err != nil {
			return nil, err
		}
		if context.DryRun {
			log.Debugf("[AWS] Processing access keys (%d): [%s]", len(resp.AccessKeyMetadata), resp.AccessKeyMetadata)
		}
		for _, akm := range resp.AccessKeyMetadata {
			if *akm.Status == "Active" {
				accessKey := *akm.AccessKeyId
				accesses = append(accesses, &types.Access{
					CloudType: types.AWS,
					Name:      accessKey[0:10] + "...",
					Owner:     *akm.UserName,
					Created:   *akm.CreateDate,
				})
			}
		}
	}
	return accesses, nil
}

func getRegions(ec2Client ec2Client) ([]string, error) {
	if context.DryRun {
		log.Debug("[AWS] Fetching regions")
	}
	regionResult, e := ec2Client.DescribeRegions(&ec2.DescribeRegionsInput{})
	if e != nil {
		return nil, e
	}
	if context.DryRun {
		log.Debugf("[AWS] Processing regions (%d): [%s]", len(regionResult.Regions), regionResult.Regions)
	}
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
	return &types.Instance{
		Id:        *inst.InstanceId,
		Name:      tags["Name"],
		Created:   *inst.LaunchTime,
		CloudType: types.AWS,
		Tags:      tags,
		Owner:     tags[context.AwsOwnerLabel],
		Region:    getRegionFromAvailabilityZone(inst.Placement.AvailabilityZone),
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
