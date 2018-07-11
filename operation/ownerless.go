package operation

import (
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/hortonworks/cloud-cost-reducer/utils"
)

func init() {
	ctx.Operations[types.OWNERLESS] = ownerless{}
}

type ownerless struct {
}

func (o ownerless) Execute(clouds []types.CloudType) []types.CloudItem {
	log.Debugf("Collecting owner less instances on: [%s]", clouds)
	itemsChan, errChan := collectRunningInstances(clouds)
	items := wait(itemsChan, errChan, "[OWNERLESS] Failed to collect owner less instances")
	return o.filter(items)
}

func (o ownerless) filter(items []types.CloudItem) []types.CloudItem {
	log.Debugf("Filtering instances (%d): [%s]", len(items), items)
	return filter(items, func(item types.CloudItem) bool {
		labels := []string{}
		if label, ok := ctx.IgnoreLabels[item.GetCloudType()]; ok {
			labels = append(labels, label)
		}
		if label, ok := ctx.OwnerLabels[item.GetCloudType()]; ok {
			labels = append(labels, label)
		}
		inst := item.(*types.Instance)
		match := !utils.IsAnyMatch(inst.Tags, labels...)
		log.Debugf("Instances: %s match: %b", inst.Name, match)
		return match
	})
}
