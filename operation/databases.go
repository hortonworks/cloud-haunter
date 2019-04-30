package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Operations[types.Databases] = databases{}
}

type databases struct {
}

func (o databases) Execute(clouds []types.CloudType) []types.CloudItem {
	log.Debugf("[GET_DATABASES] Collecting databases on: [%s]", clouds)
	itemsChan, errChan := o.collect(clouds)
	return wait(itemsChan, errChan, "[GET_DATABASES] Failed to collect databases")
}

func (o databases) collect(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
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
