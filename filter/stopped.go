package filter

import (
	"fmt"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewStopped returns the stopped filter implementation.
func NewStopped(cfg *config.Config) types.Filter {
	return stopped{cfg}
}

type stopped struct {
	cfg *config.Config
}

func (f stopped) Execute(items []types.CloudItem) ([]types.CloudItem, error) {
	log.Debugf("[STOPPED] Filtering items (%d): [%s]", len(items), items)
	return filter(f.cfg, "RUNNING", items, types.ExclusiveFilter, func(item types.CloudItem) (bool, error) {
		switch item.GetItem().(type) {
		case types.Instance:
			if item.GetItem().(types.Instance).State != types.Stopped {
				log.Debugf("[STOPPED] Filter instance, because it's not in STOPPED state: %s", item.GetName())
				return false, nil
			}
		default:
			return false, fmt.Errorf("[STOPPED] filter does not apply for cloud item: %s", item.GetName())
		}
		return true, nil
	})
}
