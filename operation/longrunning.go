package operation

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	context.Operations[types.LONGRUNNING] = LongRunning{}
}

type LongRunning struct {
}

func (o LongRunning) Execute(clouds []types.CloudType) {
	var longRunningInstances = make([]*types.Instance, 0)
	for _, cloud := range clouds {
		provider, ok := context.CloudProviders[cloud]
		if !ok {
			panic("Cloud provider not supported")
		}
		instances := provider.GetRunningInstances()
		longRunningInstances = append(longRunningInstances, getInstancesRunningLongerThan(instances, 24*time.Hour)...)
		for _, instance := range longRunningInstances {
			log.Infof("[%s] Instance %s is running for more than: %s", cloud.String(), instance.Name, time.Since(instance.Created))
		}
	}
	if len(longRunningInstances) > 0 {
		types.Dispatchers["hipchat"].Send(types.Notification{Instances: longRunningInstances})
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
