package operation

import (
	"time"

	"os"

	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/hortonworks/cloud-cost-reducer/utils"
)

var defaultRunningPeriod = 24 * time.Hour

type longRunning struct {
	runningPeriod time.Duration
}

func init() {
	runningEnv := os.Getenv("RUNNING_PERIOD")
	var runningPeriod time.Duration
	if len(runningEnv) > 0 {
		if duration, err := time.ParseDuration(runningEnv); err != nil {
			log.Errorf("[LONGRUNNING] err: %s", err)
			return
		} else {
			runningPeriod = duration
		}
	} else {
		runningPeriod = defaultRunningPeriod
	}
	log.Infof("[LONGRUNNING] running period set to: %s", runningPeriod)
	ctx.Operations[types.LONGRUNNING] = longRunning{runningPeriod}
}

func (o longRunning) Execute(clouds []types.CloudType) []types.CloudItem {
	if ctx.DryRun {
		log.Debugf("Collecting long running instances on: [%s]", clouds)
	}
	itemsChan, errChan := collectRunningInstances(clouds)
	items := wait(itemsChan, errChan, "[LONGRUNNING] Failed to collect long running instances")
	return o.filter(items)
}

func (o longRunning) filter(items []types.CloudItem) []types.CloudItem {
	if ctx.DryRun {
		log.Debugf("Filtering instances (%d): [%s]", len(items), items)
	}
	now := time.Now()
	return filter(items, func(item types.CloudItem) bool {
		ignoreLabel, ok := ctx.IgnoreLabels[item.GetCloudType()]
		inst := item.(*types.Instance)
		match := (!ok || !utils.IsAnyMatch(inst.Tags, ignoreLabel)) && item.GetCreated().Add(o.runningPeriod).Before(now)
		if ctx.DryRun {
			log.Debugf("Instances: %s match: %b", inst.Name, match)
		}
		return match
	})
}
