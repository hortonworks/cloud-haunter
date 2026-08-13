package filter

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewOwnerless returns the ownerless filter implementation.
func NewOwnerless(cfg *config.Config) types.Filter {
	return ownerless{cfg}
}

type ownerless struct {
	cfg *config.Config
}

func (o ownerless) Execute(items []types.CloudItem) ([]types.CloudItem, error) {
	log.Debugf("[OWNERLESS] Filtering instances without tag %s (%d): [%s]", o.cfg.OwnerLabel, len(items), items)
	return filter(o.cfg, "OWNERLESS", items, types.ExclusiveFilter, func(item types.CloudItem) (bool, error) {
		match := len(item.GetOwner()) == 0 || item.GetOwner() == "???"
		log.Infof("[OWNERLESS] %T: %s match: %v", item, item.GetName(), match)
		return match, nil
	})
}
