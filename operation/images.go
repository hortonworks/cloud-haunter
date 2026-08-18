package operation

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewImages returns the getImages operation implementation.
func NewImages(cfg *config.Config) types.Operation {
	return images{cfg}
}

type images struct {
	cfg *config.Config
}

func (o images) Execute(clouds []types.CloudType) ([]types.CloudItem, error) {
	log.Debugf("[GET_IMAGES] Collecting images on: [%s]", clouds)
	itemsChan, errChan := o.collect(clouds)
	return wait(itemsChan, errChan, "[GET_IMAGES] Failed to collect images"), nil
}

func (o images) collect(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(o.cfg.CloudProviders, clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		images, err := provider.GetImages()
		if err != nil {
			return nil, err
		}
		return convertToCloudItems(images), nil
	})
}
