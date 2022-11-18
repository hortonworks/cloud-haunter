package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Operations[types.Clusters] = clusters{}
}

type clusters struct {
}

func (o clusters) Execute(clouds []types.CloudType) []types.CloudItem {
	log.Debugf("[GET_CLUSTERS] Collecting clusters on: [%s]", clouds)
	itemsChan, errChan := collectClusters(clouds)
	return wait(itemsChan, errChan, "[GET_CLUSTERS] Failed to collect clusters")
}

func (o clusters) collect(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		clusters, err := provider.GetClusters()
		if err != nil {
			return nil, err
		}
		return convertClustersToCloudItems(clusters), nil
	})
}
