package filter

import (
	"fmt"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewRunning returns the running filter implementation.
func NewRunning(cfg *config.Config) types.Filter {
	return running{cfg}
}

type running struct {
	cfg *config.Config
}

func (f running) Execute(items []types.CloudItem) ([]types.CloudItem, error) {
	log.Debugf("[RUNNING] Filtering instances (%d): [%s]", len(items), items)
	return filter(f.cfg, "RUNNING", items, types.ExclusiveFilter, func(item types.CloudItem) (bool, error) {
		switch item.GetItem().(type) {
		case types.Instance:
			if item.GetItem().(types.Instance).State != types.Running {
				log.Debugf("[RUNNING] Filter instance, because it's not in RUNNING state: %s", item.GetName())
				return false, nil
			}
		case types.Stack:
			if item.GetItem().(types.Stack).State != types.Running {
				log.Debugf("[RUNNING] Filter stack, because it's not in RUNNING state: %s", item.GetName())
				return false, nil
			}
		case types.Database:
			if item.GetItem().(types.Database).State != types.Running {
				log.Debugf("[RUNNING] Filter database, because it's not in RUNNING state: %s", item.GetName())
				return false, nil
			}
		case types.Disk:
			if item.GetItem().(types.Disk).State != types.Unused {
				log.Debugf("[RUNNING] Filter disk, because it's in used state: %s", item.GetName())
				return false, nil
			}
		default:
			return false, fmt.Errorf("[RUNNING] filter does not apply for cloud item: %s", item.GetName())
		}
		return true, nil
	})
}
