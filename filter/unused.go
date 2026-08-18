package filter

import (
	"fmt"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewUnused returns the unused filter implementation.
func NewUnused(cfg *config.Config) types.Filter {
	return unused{cfg}
}

type unused struct {
	cfg *config.Config
}

func (f unused) Execute(items []types.CloudItem) ([]types.CloudItem, error) {
	log.Debugf("[UNUSED] Filtering items (%d): [%s]", len(items), items)
	return filter(f.cfg, "UNUSED", items, types.ExclusiveFilter, func(item types.CloudItem) (bool, error) {
		switch item.GetItem().(type) {
		case types.Disk:
			if item.GetItem().(types.Disk).State != types.Unused {
				log.Debugf("[UNUSED] Filter disk, because it's in use: %s", item.GetName())
				return false, nil
			}
		case types.Alert:
			if item.GetItem().(types.Alert).State != types.Unused {
				log.Debugf("[UNUSED] Filter alert, because it's in use: %s", item.GetName())
				return false, nil
			}
		default:
			return false, fmt.Errorf("[UNUSED] filter does not apply for cloud item: %s", item.GetName())
		}
		log.Debugf("[UNUSED] Item was not filtered: %s", item.GetName())
		return true, nil
	})
}
