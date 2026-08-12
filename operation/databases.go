package operation

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewDatabases returns the getDatabases operation implementation.
func NewDatabases(cfg *config.Config) types.Operation {
	return databases{cfg}
}

type databases struct {
	cfg *config.Config
}

func (o databases) Execute(clouds []types.CloudType) ([]types.CloudItem, error) {
	log.Debugf("[GET_DATABASES] Collecting databases on: [%s]", clouds)
	itemsChan, errChan := o.collect(clouds)
	return wait(itemsChan, errChan, "[GET_DATABASES] Failed to collect databases"), nil
}

func (o databases) collect(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(o.cfg.CloudProviders, clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		databases, err := provider.GetDatabases()
		if err != nil {
			return nil, err
		}
		return o.convertToCloudItems(databases), nil
	})
}

func (o databases) convertToCloudItems(databases []*types.Database) []types.CloudItem {
	var items []types.CloudItem
	for _, access := range databases {
		items = append(items, access)
	}
	return items
}
