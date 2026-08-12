package filter

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewNoMatch returns the nomatch filter implementation.
func NewNoMatch(cfg *config.Config) types.Filter {
	return noMatch{cfg}
}

type noMatch struct {
	cfg *config.Config
}

func (f noMatch) Execute(items []types.CloudItem) ([]types.CloudItem, error) {
	log.Debugf("[NO_MATCH] Filtering items (%d): [%s]", len(items), items)
	return filter(f.cfg, "NO_MATCH", items, types.ExclusiveFilter, func(item types.CloudItem) (bool, error) {
		return true, nil
	})
}
