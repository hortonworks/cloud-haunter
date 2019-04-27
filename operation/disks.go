package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Operations[types.Disks] = disks{}
}

type disks struct {
}

func (o disks) Execute(clouds []types.CloudType) []types.CloudItem {
	log.Debugf("[GET_DISKS] Collecting disks on: [%s]", clouds)
	itemsChan, errChan := o.collect(clouds)
	return wait(itemsChan, errChan, "[GET_DISKS] Failed to collect disks")
}

func (o disks) collect(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		disks, err := provider.GetDisks()
		if err != nil {
			return nil, err
		}
		return o.convertToCloudItems(disks), nil
	})
}

func (o disks) convertToCloudItems(databases []*types.Disk) []types.CloudItem {
	var items []types.CloudItem
	for _, access := range databases {
		items = append(items, access)
	}
	return items
}
