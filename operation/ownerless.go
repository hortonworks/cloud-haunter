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
	instsChan, errChan := collectInstances(clouds, func(provider types.CloudProvider) ([]*types.Instance, error) {
		return provider.GetOwnerLessInstances()
	})
	return waitForInstances(instsChan, errChan, "[OWNERLESS] Failed to collect owner less instances", nil)
}
