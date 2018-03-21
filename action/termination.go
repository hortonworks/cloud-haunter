package action

import (
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

func (a TerminationAction) Execute(allInstances []*types.Instance) {
	wg := sync.WaitGroup{}
	wg.Add(len(context.CloudProviders))
	for t, p := range context.CloudProviders {
		go func(cType types.CloudType, provider types.CloudProvider) {
			defer wg.Done()

			instances := []*types.Instance{}
			for _, inst := range allInstances {
				if inst.CloudType == cType {
					instances = append(instances, inst)
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
