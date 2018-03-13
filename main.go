package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/aws"
	"github.com/hortonworks/cloud-cost-reducer/gcp"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"time"
	"github.com/hortonworks/cloud-cost-reducer/azure"
)

func init() {
	aws.Init()
	gcp.Init()
	azure.Init()
}

func main() {
	for name, provider := range types.CloudProviders {
		instances := provider.GetRunningInstances()
		longRunningInstances := getInstancesRunningLongerThan(instances, 24*time.Hour)
		for _, instance := range longRunningInstances {
			log.Infof("[%s] Instance %s is running for more than: %s", name, instance.Name, time.Since(instance.Created))
		}
	}
}

func getInstancesRunningLongerThan(instances []*types.Instance, period time.Duration) []*types.Instance {
	filtered := make([]*types.Instance, 0)
	for _, instance := range instances {
		if instance.Created.Add(period).Before(time.Now()) {
			filtered = append(filtered, instance)
		}
	}
	return filtered
}
