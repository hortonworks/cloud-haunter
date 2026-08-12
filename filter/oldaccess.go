package filter

import (
	"fmt"
	"time"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

type oldAccess struct {
	cfg *config.Config
}

// NewOldAccess returns the oldaccess filter implementation. The available period
// is taken from cfg.AccessAvailablePeriod (resolved from ACCESS_AVAILABLE_PERIOD
// when the config is built).
func NewOldAccess(cfg *config.Config) types.Filter {
	return oldAccess{cfg}
}

func (f oldAccess) Execute(items []types.CloudItem) ([]types.CloudItem, error) {
	log.Debugf("[OLDACCESS] Filtering accesses (%d): [%s]", len(items), items)
	return filter(f.cfg, "OLDACCESS", items, types.ExclusiveFilter, func(item types.CloudItem) (bool, error) {
		switch item.GetItem().(type) {
		case types.Access:
			match := item.GetCreated().Add(f.cfg.AccessAvailablePeriod).Before(time.Now())
			log.Debugf("[OLDACCESS] Access: %s match: %v", item.GetName(), match)
			return match, nil
		default:
			return false, fmt.Errorf("[OLDACCESS] filter does not apply for cloud item: %s", item.GetName())
		}
	})
}
