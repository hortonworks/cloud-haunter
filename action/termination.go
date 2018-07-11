package action

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	ctx.Actions[types.TerminationAction] = new(terminationAction)
}

type terminationAction struct {
}

func (a terminationAction) Execute(op *types.OpType, items []types.CloudItem) {
	log.Debugf("[TERMINATION] Terminating instances (%d): [%s]", len(items), items)
	wg := sync.WaitGroup{}
	wg.Add(len(ctx.CloudProviders))
	for t, p := range ctx.CloudProviders {
		log.Debugf("[TERMINATION] Terminating %s instances", t)
		go func(cType types.CloudType, provider types.CloudProvider) {
			defer wg.Done()

			instances := []*types.Instance{}
			for _, item := range items {
				switch t := item.GetItem().(type) {
				case types.Instance:
					if item.GetCloudType() == cType {
						inst := item.GetItem().(types.Instance)
						instances = append(instances, &inst)
					}
				default:
					panic(fmt.Sprintf("[TERMINATION] Operation on type %T not allowed", t))
				}
			}
			log.Infof("[TERMINATION] Terminating %d instances on %s", len(instances), cType)
			log.Debugf("[TERMINATION] Instances to terminate (%d): [%s]", len(instances), instances)
			if len(instances) > 0 {
				if err := provider.TerminateInstances(instances); err != nil {
					log.Errorf("[TERMINATION] Failed to terminate instances on %s, err: %s", cType.String(), err.Error())
				}
			}
		}(t, p())
	}
	wg.Wait()
}
