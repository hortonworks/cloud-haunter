package aws

import (
	"crypto/tls"
	"net/http"

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
	log.Infof("[AWS] Trying to register as provider")
	var err error
	regions, err = getRegions()
	if err != nil {
		log.Errorf("[AWS] Failed to authenticate, err: %s", err.Error())
		return
	}
	for _, region := range regions {
		if client, err := newEc2Client(&region); err != nil {
			log.Errorf("[AWS] Failed to create client in region %s, err: %s", region, err.Error())
			return
		} else {
			regionClients[region] = client
		}
	}
	context.CloudProviders[types.AWS] = new(AwsProvider)
	log.Infof("[AWS] Successfully registered as provider")
}

type AwsProvider struct {
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

func (p *AwsProvider) GetRunningInstances() ([]*types.Instance, error) {
	instChan := make(chan *types.Instance, 5)
	doneChan := make(chan bool, 0)

	for _, region := range regions {
		go func(region string) {
			defer func() {
				doneChan <- true
			}()
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
				for _, inst := range res.Instances { // TODO filter by ignore tag
					tags := getTags(inst.Tags)
					instChan <- &types.Instance{
						Id:        *inst.InstanceId,
						Name:      tags["Name"],
						Created:   *inst.LaunchTime,
						CloudType: types.AWS,
						Tags:      tags,
					}
				}
			}
		}(region)
	}

	instances := []*types.Instance{}
	regionsLeft := len(regions)
	for regionsLeft != 0 {
		select {
		case inst, ok := <-instChan:
			if ok {
				instances = append(instances, inst)
			}
		case <-doneChan:
			regionsLeft -= 1
		}
	}

	return instances, nil
}

func getTags(ec2Tags []*ec2.Tag) types.Tags {
	tags := make(types.Tags, 0)
	for _, t := range ec2Tags {
		tags[*t.Key] = *t.Value
	}
	return tags
}

func (a AwsProvider) GetOwnerLessInstances() ([]*types.Instance, error) {
	panic("Operation not supported.")
}
