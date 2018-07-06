package operation

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

var availablePeriod = 120 * 24 * time.Hour

func init() {
	context.Operations[types.OLDACCESS] = OldAccess{}
}

type OldAccess struct {
}

func (o OldAccess) Execute(clouds []types.CloudType) []types.CloudItem {
	if context.DryRun {
		log.Debugf("Collecting old accesses on: [%s]", clouds)
	}
	accessChan, errChan := o.collect(clouds)
	items := wait(accessChan, errChan, "[OLDACCESS] Failed to collect old accesses")
	return o.filter(items)
}

func (o OldAccess) filter(items []types.CloudItem) []types.CloudItem {
	if context.DryRun {
		log.Debugf("Filtering accesses (%d): [%s]", len(items), items)
	}
	return filter(items, func(item types.CloudItem) bool {
		match := item.GetCreated().Add(availablePeriod).Before(time.Now())
		if context.DryRun {
			log.Debugf("Access: %s match: %b", item.GetName(), match)
		}
		return match
	})
}

func (o OldAccess) collect(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		accesses, err := provider.GetAccesses()
		if err != nil {
			return nil, err
		}
		return o.convertToCloudItems(accesses), nil
	})
}

func (o OldAccess) convertToCloudItems(accesses []*types.Access) []types.CloudItem {
	items := []types.CloudItem{}
	for _, access := range accesses {
		items = append(items, access)
	}
	return items
}
