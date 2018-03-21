package operation

import (
	"time"

	"os"

	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/hortonworks/cloud-cost-reducer/utils"
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
	instsChan, errChan := collectRunningInstances(clouds)
	instances := waitForInstances(instsChan, errChan, "[LONGRUNNING] Failed to collect long running instances")
	return filterInstancesRunningLongerThan(instances)
}

func filterInstancesRunningLongerThan(instances []*types.Instance) []*types.Instance {
	return filter(instances, func(inst *types.Instance) bool {
		ignoreLabel, ok := ctx.IgnoreLabels[inst.CloudType]
		return (!ok || !utils.IsAnyMatch(inst.Tags, ignoreLabel)) && inst.Created.Add(runningPeriod).Before(time.Now())
	})
}
