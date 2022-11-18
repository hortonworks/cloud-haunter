package operation

import (
	"time"

	"os"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
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
	ctx.Filters[types.LongRunningFilter] = longRunning{runningPeriod}
}

func (f longRunning) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[LONGRUNNING] Filtering items (%d): [%s]", len(items), items)
	now := time.Now()
	return filter("LONGRUNNING", items, types.ExclusiveFilter, func(item types.CloudItem) bool {
		switch item.GetItem().(type) {
		case types.Instance:
			if item.GetItem().(types.Instance).State != types.Running {
				log.Debugf("[LONGRUNNING] Filter instance, because it's not in RUNNING state: %s", item.GetName())
				return false
			}
		case types.Stack:
			if item.GetItem().(types.Stack).State != types.Running {
				log.Debugf("[LONGRUNNING] Filter stack, because it's not in RUNNING state: %s", item.GetName())
				return false
			}
		case types.Database:
			if item.GetItem().(types.Database).State != types.Running {
				log.Debugf("[LONGRUNNING] Filter database, because it's not in RUNNING state: %s", item.GetName())
				return false
			}
		case types.Disk:
			if item.GetItem().(types.Disk).State != types.Unused {
				log.Debugf("[LONGRUNNING] Filter disk, because it's in used state: %s", item.GetName())
				return false
			}
		case types.Alert:
			if item.GetItem().(types.Alert).State != types.Unused {
				log.Debugf("[LONGRUNNING] Filter alert, because it's in used state: %s", item.GetName())
			}
		case types.Cluster:
			if item.GetItem().(types.Cluster).State != types.Running {
				log.Debugf("[LONGRUNNING] Filter instance, because it's not in RUNNING state: %s", item.GetName())
				return false
			}
		default:
			log.Fatalf("[LONGRUNNING] Filter does not apply for cloud item: %s", item.GetName())
			return true
		}
		match := item.GetCreated().Add(f.runningPeriod).Before(now)
		log.Debugf("[LONGRUNNING] %s: %s match: %v", item.GetType(), item.GetName(), match)
		return match
	})
}
