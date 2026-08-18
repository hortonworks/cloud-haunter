package operation

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewAccess returns the getAccess operation implementation.
func NewAccess(cfg *config.Config) types.Operation {
	return access{cfg}
}

type access struct {
	cfg *config.Config
}

func (o access) Execute(clouds []types.CloudType) ([]types.CloudItem, error) {
	log.Debugf("Collecting old accesses on: [%s]", clouds)
	accessChan, errChan := o.collect(clouds)
	return wait(accessChan, errChan, "[ACCESS] Failed to collect accesses"), nil
}

func (o access) collect(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(o.cfg.CloudProviders, clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		accesses, err := provider.GetAccesses()
		if err != nil {
			return nil, err
		}
		return o.convertToCloudItems(accesses), nil
	})
}

func (o access) convertToCloudItems(accesses []*types.Access) []types.CloudItem {
	var items []types.CloudItem
	for _, access := range accesses {
		items = append(items, access)
	}
	return items
}
