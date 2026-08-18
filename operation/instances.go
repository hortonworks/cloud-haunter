package operation

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewInstances returns the getInstances operation implementation.
func NewInstances(cfg *config.Config) types.Operation {
	return instances{cfg}
}

type instances struct {
	cfg *config.Config
}

func (o instances) Execute(clouds []types.CloudType) ([]types.CloudItem, error) {
	log.Debugf("[GET_INSTANCES] Collecting long instances instances on: [%s]", clouds)
	itemsChan, errChan := collectInstances(o.cfg.CloudProviders, clouds)
	return wait(itemsChan, errChan, "[GET_INSTANCES] Failed to collect instances"), nil
}
