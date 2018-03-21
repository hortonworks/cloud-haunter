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
	instChan := make(chan *types.Instance, 5)
	wg := sync.WaitGroup{}
	wg.Add(len(regions))
	for _, region := range regions {
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

func newEc2Client(region *string) (*ec2.EC2, error) {
	var awsSession *session.Session
	var err error
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}}
	if region != nil && len(*region) > 0 {
		awsSession, err = session.NewSession(
			&aws.Config{
				Region:     aws.String(*region),
				HTTPClient: httpClient,
			})
	} else {
		awsSession, err = session.NewSession(
			&aws.Config{
				Region:     aws.String("eu-west-1"),
				HTTPClient: httpClient,
			})
	}
	if err != nil {
		return nil, err
	}
	return ec2.New(awsSession), nil
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
	}
}
