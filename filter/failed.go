package filter

import (
	"fmt"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewFailed returns the failed filter implementation.
func NewFailed(cfg *config.Config) types.Filter {
	return failed{cfg}
}

type failed struct {
	cfg *config.Config
}

func (f failed) Execute(items []types.CloudItem) ([]types.CloudItem, error) {
	log.Debugf("[FAILED] Filtering instances (%d): [%s]", len(items), items)
	return filter(f.cfg, "FAILED", items, types.ExclusiveFilter, func(item types.CloudItem) (bool, error) {
		switch item.GetItem().(type) {
		case types.Stack:
			if item.GetItem().(types.Stack).State != types.Failed {
				log.Debugf("[FAILED] Filter stack, because it's not in FAILED state: %s", item.GetName())
				return false, nil
			}
		default:
			return false, fmt.Errorf("[FAILED] filter does not apply for cloud item: %s", item.GetName())
		}
		return true, nil
	})
}
