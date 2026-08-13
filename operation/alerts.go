package operation

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewAlerts returns the getAlerts operation implementation.
func NewAlerts(cfg *config.Config) types.Operation {
	return alerts{cfg}
}

type alerts struct {
	cfg *config.Config
}

func (a alerts) Execute(clouds []types.CloudType) ([]types.CloudItem, error) {
	log.Debugf("[ALERTS] Collecting alerts on: [%s]", clouds)
	itemsChan, errChan := a.collect(clouds)
	return wait(itemsChan, errChan, "[ALERTS] Failed to collect alerts"), nil
}

func (a alerts) collect(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(a.cfg.CloudProviders, clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		alerts, err := provider.GetAlerts()
		if err != nil {
			return nil, err
		}
		return a.convertToCloudItems(alerts), nil
	})
}

func (a alerts) convertToCloudItems(alerts []*types.Alert) []types.CloudItem {
	var items []types.CloudItem
	for _, alert := range alerts {
		items = append(items, alert)
	}
	return items
}
