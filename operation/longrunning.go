package operation

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	types.Operations[types.LONGRUNNING] = LongRunning{}
}

type LongRunning struct {
}

func (o LongRunning) Execute(clouds []types.CloudType) {
	for _, cloud := range clouds {
		provider, ok := types.CloudProviders[cloud]
		if !ok {
			panic("Cloud provider not supported")
		}
		instances := provider.GetRunningInstances()
		longRunningInstances := getInstancesRunningLongerThan(instances, 24*time.Hour)
		for _, instance := range longRunningInstances {
			log.Infof("[%s] Instance %s is running for more than: %s", cloud.String(), instance.Name, time.Since(instance.Created))
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
