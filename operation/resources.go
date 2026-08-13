package operation

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewResources returns the getResources operation implementation.
func NewResources(cfg *config.Config) types.Operation {
	return resources{cfg}
}

type resources struct {
	cfg *config.Config
}

func (o resources) Execute(clouds []types.CloudType) ([]types.CloudItem, error) {
	log.Debugf("[GET_RESOURCES] Collecting resources on: [%s]", clouds)
	itemsChan, errChan := collectResources(o.cfg.CloudProviders, clouds)
	return wait(itemsChan, errChan, "[GET_RESOURCES] Failed to collect resources"), nil
}
