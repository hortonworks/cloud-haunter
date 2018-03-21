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

func (o Ownerless) Execute(clouds []types.CloudType) []*types.Instance {
	instsChan, errChan := collectRunningInstances(clouds)
	instances := waitForInstances(instsChan, errChan, "[OWNERLESS] Failed to collect owner less instances")
	return filterInstancesWithoutOwner(instances)
}

func filterInstancesWithoutOwner(instances []*types.Instance) []*types.Instance {
	return filter(instances, func(inst *types.Instance) bool {
		labels := []string{}
		if label, ok := context.IgnoreLabels[inst.CloudType]; ok {
			labels = append(labels, label)
		}
		if label, ok := context.OwnerLabels[inst.CloudType]; ok {
			labels = append(labels, label)
		}
		return !utils.IsAnyMatch(inst.Tags, labels...)
	})
}
