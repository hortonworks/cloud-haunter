package aws

import (
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
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
	types.CloudProviders[types.AWS] = new(AwsProvider)
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
	if region != nil && len(*region) > 0 {
		awsSession, err = session.NewSession(&aws.Config{
			Region: aws.String(*region)})
	} else {
		awsSession, err = session.NewSession(&aws.Config{
			Region: aws.String("eu-west-1")})
	}
	if err != nil {
		return nil, err
	}
	return ec2.New(awsSession), nil
}

func (p *AwsProvider) GetRunningInstances() []*types.Instance {
	instances := make([]*types.Instance, 0)
	var wg sync.WaitGroup
	for _, region := range regions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			instanceResult, e := regionClients[region].DescribeInstances(&ec2.DescribeInstancesInput{})
			if e != nil {
				log.Errorf("[AWS] Failed to fetch the running instances in region: %s, err: %s", region, e)
				return
			}
			for _, res := range instanceResult.Reservations {
				for _, inst := range res.Instances {
					instances = append(instances, &types.Instance{
						Id:      *inst.InstanceId,
						Name:    getTagValue("Name", inst.Tags),
						Created: *inst.LaunchTime,
					})
				}
			}
		}(region)
	}
	wg.Wait()
	return instances
}

func getTagValue(key string, tags []*ec2.Tag) string {
	for _, tag := range tags {
		if *tag.Key == key {
			return *tag.Value
		}
	}
	return ""
}
