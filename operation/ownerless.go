package operation

import (
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/hortonworks/cloud-cost-reducer/utils"
)

func init() {
	context.Operations[types.OWNERLESS] = Ownerless{}
}

type Ownerless struct {
}

func (o Ownerless) Execute(clouds []types.CloudType) []types.CloudItem {
	itemsChan, errChan := collectRunningInstances(clouds)
	items := wait(itemsChan, errChan, "[OWNERLESS] Failed to collect owner less instances")
	return o.filter(items)
}

func (o Ownerless) filter(items []types.CloudItem) []types.CloudItem {
	return filter(items, func(item types.CloudItem) bool {
		labels := []string{}
		if label, ok := context.IgnoreLabels[item.GetCloudType()]; ok {
			labels = append(labels, label)
		}
		if label, ok := context.OwnerLabels[item.GetCloudType()]; ok {
			labels = append(labels, label)
		}
		inst := item.(*types.Instance)
		return !utils.IsAnyMatch(inst.Tags, labels...)
	})
}
