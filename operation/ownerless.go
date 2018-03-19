package operation

import (
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	context.Operations[types.OWNERLESS] = Ownerless{}
}

type Ownerless struct {
}

func (o Ownerless) Execute(clouds []types.CloudType) []*types.Instance {
	allInstances := []*types.Instance{}
	for _, cloud := range clouds {
		provider, ok := context.CloudProviders[cloud]
		if !ok {
			panic("Cloud provider not supported")
		}
		instances, err := provider.TerminateRunningInstances()
		if err != nil {
			continue
		}
		allInstances = append(allInstances, instances...)
	}
	return allInstances
}
