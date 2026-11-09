package filter

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewIgnored returns the ignored filter implementation.
func NewIgnored(cfg *config.Config) types.Filter {
	return ignored{cfg}
}

type ignored struct {
	cfg *config.Config
}

func (f ignored) Execute(items []types.CloudItem) ([]types.CloudItem, error) {
	log.Debugf("[IGNORED] Filtering items (%d): [%s]", len(items), items)
	var kept []types.CloudItem
	for _, item := range items {
		if f.isIgnored(item) {
			kept = append(kept, item)
		}
	}
	return kept, nil
}

func (f ignored) isIgnored(item types.CloudItem) bool {
	_, labelled := item.GetTags()[f.cfg.IgnoreLabel]
	return labelled || types.GetItemMetadata(item)[types.ResourceGroupIgnoredMetadata] == "true"
}
