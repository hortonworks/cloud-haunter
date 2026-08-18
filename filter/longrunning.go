package filter

import (
	"fmt"
	"time"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

type longRunning struct {
	cfg *config.Config
}

// NewLongRunning returns the longrunning filter implementation. The running
// period is taken from cfg.LongRunningPeriod (resolved from RUNNING_PERIOD when
// the config is built).
func NewLongRunning(cfg *config.Config) types.Filter {
	return longRunning{cfg}
}

func (f longRunning) Execute(items []types.CloudItem) ([]types.CloudItem, error) {
	log.Debugf("[LONGRUNNING] Filtering instances (%d): [%s]", len(items), items)
	now := time.Now()
	return filter(f.cfg, "LONGRUNNING", items, types.ExclusiveFilter, func(item types.CloudItem) (bool, error) {
		switch item.GetItem().(type) {
		case types.Instance:
			if item.GetItem().(types.Instance).State != types.Running {
				log.Debugf("[LONGRUNNING] Filter instance, because it's not in RUNNING state: %s", item.GetName())
				return false, nil
			}
		case types.Stack:
			if item.GetItem().(types.Stack).State != types.Running {
				log.Debugf("[LONGRUNNING] Filter stack, because it's not in RUNNING state: %s", item.GetName())
				return false, nil
			}
		case types.Database:
			if item.GetItem().(types.Database).State != types.Running {
				log.Debugf("[LONGRUNNING] Filter database, because it's not in RUNNING state: %s", item.GetName())
				return false, nil
			}
		case types.Disk:
			if item.GetItem().(types.Disk).State != types.Unused {
				log.Debugf("[LONGRUNNING] Filter disk, because it's in used state: %s", item.GetName())
				return false, nil
			}
		case types.Alert:
			if item.GetItem().(types.Alert).State != types.Unused {
				log.Debugf("[LONGRUNNING] Filter alert, because it's in used state: %s", item.GetName())
				return false, nil
			}
		case types.Resource:
			if item.GetItem().(types.Resource).ResourceType == types.Vpc {
				log.Debugf("[LONGRUNNING] Do not filter resource, because VPC has no 'Created' property: %s", item.GetName())
				return true, nil
			}
		default:
			return false, fmt.Errorf("[LONGRUNNING] filter does not apply for cloud item: %s", item.GetName())
		}
		match := item.GetCreated().Add(f.cfg.LongRunningPeriod).Before(now)
		log.Debugf("[LONGRUNNING] %s: %s match: %v", item.GetType(), item.GetName(), match)
		return match, nil
	})
}
