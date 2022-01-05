package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Operations[types.Storages] = storage{}
}

type storage struct {
}

func (s storage) Execute(clouds []types.CloudType) []types.CloudItem {
	log.Debugf("Collecting storage accounts on: [%s]", clouds)
	storageChan, errChan := s.collect(clouds)
	return wait(storageChan, errChan, "[ACCESS] Failed to collect storages")
}

func (s storage) collect(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
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
