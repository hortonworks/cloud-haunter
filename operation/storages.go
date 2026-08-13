package operation

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewStorages returns the getStorages operation implementation.
func NewStorages(cfg *config.Config) types.Operation {
	return storage{cfg}
}

type storage struct {
	cfg *config.Config
}

func (s storage) Execute(clouds []types.CloudType) ([]types.CloudItem, error) {
	log.Debugf("Collecting storage accounts on: [%s]", clouds)
	storageChan, errChan := s.collect(clouds)
	return wait(storageChan, errChan, "[ACCESS] Failed to collect storages"), nil
}

func (s storage) collect(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(s.cfg.CloudProviders, clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		storages, err := provider.GetStorages()
		if err != nil {
			return nil, err
		}
		return s.convertToCloudItems(storages), nil
	})
}

func (s storage) convertToCloudItems(storages []*types.Storage) []types.CloudItem {
	var items []types.CloudItem
	for _, storage := range storages {
		items = append(items, storage)
	}
	return items
}
