package operation

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewStacks returns the getStacks operation implementation.
func NewStacks(cfg *config.Config) types.Operation {
	return stacks{cfg}
}

type stacks struct {
	cfg *config.Config
}

func (o stacks) Execute(clouds []types.CloudType) ([]types.CloudItem, error) {
	log.Debugf("[GET_STACKS] Collecting stacks on: [%s]", clouds)
	itemsChan, errChan := collectStacks(o.cfg.CloudProviders, clouds)
	return wait(itemsChan, errChan, "[GET_STACKS] Failed to collect stacks"), nil
}
