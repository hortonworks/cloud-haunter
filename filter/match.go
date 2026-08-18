package filter

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewMatch returns the match filter implementation.
func NewMatch(cfg *config.Config) types.Filter {
	return match{cfg}
}

type match struct {
	cfg *config.Config
}

func (f match) Execute(items []types.CloudItem) ([]types.CloudItem, error) {
	log.Debugf("[MATCH] Filtering items (%d): [%s]", len(items), items)
	return filter(f.cfg, "MATCH", items, types.InclusiveFilter, func(item types.CloudItem) (bool, error) {
		return true, nil
	})
}
