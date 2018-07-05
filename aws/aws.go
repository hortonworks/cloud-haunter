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

var (
	regions       = make([]string, 0)
	regionClients = make(map[string]*ec2.EC2, 0)
)

func init() {
	context.CloudProviders[types.AWS] = func() types.CloudProvider {
		prepare()
		return new(AwsProvider)
	}
}

func prepare() {
	if len(regionClients) == 0 {
		log.Infof("[AWS] Trying to prepare")
		var err error
		regions, err = getRegions()
		if err != nil {
			panic("[AWS] Failed to authenticate, err: " + err.Error())
		}
		for _, region := range regions {
			if client, err := newEc2Client(&region); err != nil {
				panic(fmt.Sprintf("[AWS] Failed to create client in region %s, err: %s", region, err.Error()))
			} else {
				regionClients[region] = client
			}
		}
		log.Infof("[AWS] Successfully prepared")
	}
}

type AwsProvider struct {
}

func (p *AwsProvider) GetRunningInstances() ([]*types.Instance, error) {
	if context.DryRun {
		log.Debug("[AWS] Fetching instanes")
	}
	instChan := make(chan *types.Instance, 5)
	wg := sync.WaitGroup{}
	wg.Add(len(regions))
	for _, region := range regions {
		if context.DryRun {
			log.Debugf("[AWS] Fetching instanes from: %s", region)
		}
		go func(region string) {
			defer wg.Done()

			filterName := "instance-state-name"
			filterValue := ec2.InstanceStateNameRunning
			runningFilter := []*ec2.Filter{{Name: &filterName, Values: []*string{&filterValue}}}
			instanceResult, e := regionClients[region].DescribeInstances(&ec2.DescribeInstancesInput{
				Filters: runningFilter,
			})
			if e != nil {
				log.Errorf("[AWS] Failed to fetch the running instances in region: %s, err: %s", region, e)
				return
			}
			if context.DryRun {
				log.Debugf("[AWS] Processing instances: [%s] in region: %s", instanceResult.Reservations, region)
			}
			for _, res := range instanceResult.Reservations {
				for _, inst := range res.Instances {
					instChan <- newInstance(inst)
				}
			}
		}(region)
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

func (a AwsProvider) TerminateInstances([]*types.Instance) error {
	return errors.New("[AWS] Termination not supported")
}

func (a AwsProvider) GetAccesses() ([]*types.Access, error) {
	client, err := newIamClient()
	if err != nil {
		return nil, err
	}
	if context.DryRun {
		log.Debug("[AWS] Fetching users")
	}
	users, err := client.ListUsers(&iam.ListUsersInput{MaxItems: &(&types.I64{I: 1000}).I})
	if err != nil {
		return nil, err
	}
	if context.DryRun {
		log.Debugf("[AWS] Processing users: [%s]", users.Users)
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
		resp, err := client.ListAccessKeys(req)
		if err != nil {
			return nil, err
		}
		if context.DryRun {
			log.Debugf("[AWS] Processing access keys: [%s]", resp.AccessKeyMetadata)
		}
		for _, akm := range resp.AccessKeyMetadata {
			if *akm.Status == "Active" {
				accessKey := *akm.AccessKeyId
				accesses = append(accesses, &types.Access{
					CloudType: types.AWS,
					Name:      accessKey[0:4] + "...",
					Owner:     *akm.UserName,
					Created:   *akm.CreateDate,
				})
			}
		}
	}
	return accesses, nil
}

func getRegions() ([]string, error) {
	client, err := newEc2Client(nil)
	if err != nil {
		return nil, err
	}
	regionResult, e := client.DescribeRegions(&ec2.DescribeRegionsInput{})
	if e != nil {
		return nil, e
	}
	regions := make([]string, 0)
	for _, region := range regionResult.Regions {
		regions = append(regions, *region.RegionName)
	}
	return regions, nil
}

func newIamClient() (*iam.IAM, error) {
	awsSession, err := newSession(nil)
	if err != nil {
		return nil, err
	}
	return iam.New(awsSession), nil
}

func newEc2Client(region *string) (*ec2.EC2, error) {
	if region == nil || len(*region) == 0 {
		region = &(&types.S{S: "eu-west-1"}).S
	}
	awsSession, err := newSession(func(config *aws.Config) {
		config.Region = region
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

func getTags(ec2Tags []*ec2.Tag) types.Tags {
	tags := make(types.Tags, 0)
	for _, t := range ec2Tags {
		tags[*t.Key] = *t.Value
	}
	return tags
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

func getRegionFromAvailabilityZone(az *string) string {
	if az == nil || len(*az) < 1 {
		return ""
	}
	return (*az)[0 : len(*az)-1]
}
