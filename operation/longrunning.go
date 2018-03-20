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
	instsChan, errChan := collectInstances(clouds, func(provider types.CloudProvider) ([]*types.Instance, error) {
		return provider.GetRunningInstances()
	})
	return waitForInstances(instsChan, errChan, "[LONGRUNNING] Failed to collect long running instances", getInstancesRunningLongerThan)
}

func getInstancesRunningLongerThan(instances []*types.Instance) []*types.Instance {
	filtered := make([]*types.Instance, 0)
	for _, instance := range instances {
		if instance.Created.Add(runningPeriod).Before(time.Now()) {
			filtered = append(filtered, instance)
		}
	}
	return filtered
}
