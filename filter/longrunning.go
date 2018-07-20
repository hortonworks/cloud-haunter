package operation

import (
	"time"

	"os"

	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
)

var defaultRunningPeriod = 24 * time.Hour

type longRunning struct {
	runningPeriod time.Duration
}

func init() {
	runningEnv := os.Getenv("RUNNING_PERIOD")
	var runningPeriod time.Duration
	if len(runningEnv) > 0 {
		duration, err := time.ParseDuration(runningEnv)
		if err != nil {
			log.Errorf("[LONGRUNNING] err: %s", err)
			return
		}
		runningPeriod = duration
	} else {
		runningPeriod = defaultRunningPeriod
	}
	log.Infof("[LONGRUNNING] running period set to: %s", runningPeriod)
	ctx.Filters[types.LongRunning] = longRunning{runningPeriod}
}

func (f longRunning) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[LONGRUNNING] Filtering instances (%d): [%s]", len(items), items)
	now := time.Now()
	return filter(items, func(item types.CloudItem) bool {
		if !isInstance(item) {
			log.Debugf("[LONGRUNNING] Filter does not apply for cloud item: %s", item.GetName())
			return true
		}
		if item.GetItem().(types.Instance).State != types.Running {
			log.Debugf("[LONGRUNNING] Filter instance, because it's not in RUNNING state: %s", item.GetName())
			return false
		}
		match := item.GetCreated().Add(f.runningPeriod).Before(now)
		log.Debugf("[LONGRUNNING] Instances: %s match: %b", item.GetName(), match)
		return match
	})
}
