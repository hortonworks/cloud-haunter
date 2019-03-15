package operation

import (
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
)

func init() {
	ctx.Operations[types.Images] = images{}
}

type images struct {
}

func (o images) Execute(clouds []types.CloudType) []types.CloudItem {
	log.Debugf("[GET_IMAGES] Collecting images on: [%s]", clouds)
	itemsChan, errChan := o.collect(clouds)
	return wait(itemsChan, errChan, "[GET_IMAGES] Failed to collect images")
}

func (o images) collect(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		images, err := provider.GetImages()
		if err != nil {
			return nil, err
		}
		return o.convertToCloudItems(images), nil
	})
}

func (o images) convertToCloudItems(images []*types.Image) []types.CloudItem {
	var items []types.CloudItem
	for _, img := range images {
		items = append(items, img)
	}
	return items
}
