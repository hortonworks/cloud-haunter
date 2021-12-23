package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Operations[types.Alerts] = alerts{}
}

type alerts struct {
}

func (a alerts) Execute(clouds []types.CloudType) []types.CloudItem {
	log.Debugf("[ALERTS] Collecting alerts on: [%s]", clouds)
	itemsChan, errChan := a.collect(clouds)
	return wait(itemsChan, errChan, "[ALERTS] Failed to collect alerts")
}

func (a alerts) collect(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
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
