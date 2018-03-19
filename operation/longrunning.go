package operation

import (
	"time"

	"os"

	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

var runningPeriod = 24 * time.Hour

func init() {
	ctx.Operations[types.LONGRUNNING] = LongRunning{}
	running := os.Getenv("RUNNING_PERIOD")
	if len(running) > 0 {
		if duration, err := time.ParseDuration(running); err != nil {
			log.Warnf("[LONGRUNNING] err: %s", err)
		} else {
			runningPeriod = duration
		}
	}
	log.Infof("[LONGRUNNING] running period set to: %s", runningPeriod)
}

type LongRunning struct {
}

func (o LongRunning) Execute(clouds []types.CloudType) []*types.Instance {
	var allInstances = make([]*types.Instance, 0)
	for _, cloud := range clouds {
		provider, ok := ctx.CloudProviders[cloud]
		if !ok {
			panic("Cloud provider not supported")
		}
		instances, err := provider.GetRunningInstances()
		if err != nil {
			continue
		}
		allInstances = append(allInstances, getInstancesRunningLongerThan(instances, runningPeriod)...)
	}
	for _, instance := range allInstances {
		log.Infof("[%s] Instance %s is running for more than: %.1f hours", instance.CloudType, instance.Name, time.Since(instance.Created).Hours())
	}
	return allInstances
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
