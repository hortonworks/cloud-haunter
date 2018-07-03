package action

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	context.Actions[types.TERMINATION_ACTION] = new(TerminationAction)
}

type TerminationAction struct {
}

func (a TerminationAction) Execute(op *types.OpType, items []types.CloudItem) {
	wg := sync.WaitGroup{}
	wg.Add(len(context.CloudProviders))
	for t, p := range context.CloudProviders {
		go func(cType types.CloudType, provider types.CloudProvider) {
			defer wg.Done()

			instances := []*types.Instance{}
			for _, item := range items {
				switch t := item.GetItem().(type) {
				case types.Instance:
					if item.GetCloudType() == cType {
						var inst types.Instance = item.(types.Instance)
						instances = append(instances, &inst)
					}
				default:
					panic(fmt.Sprintf("[TERMINATION] Operation on type %T not allowed", t))
				}
			}
			if len(instances) > 0 {
				if err := provider.TerminateInstances(instances); err != nil {
					log.Errorf("[TERMINATION] Failed to terminate instances on %s, err: %s", cType.String(), err.Error())
				}
			}
		}(t, p())
	}
	wg.Wait()
}
