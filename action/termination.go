package action

import (
	"sync"

	"fmt"
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
)

func init() {
	ctx.Actions[types.TerminationAction] = new(terminationAction)
}

type terminationAction struct {
}

func (a terminationAction) Execute(op types.OpType, filters []types.FilterType, items []types.CloudItem) {
	wg := sync.WaitGroup{}
	wg.Add(len(ctx.CloudProviders))
	for t, p := range ctx.CloudProviders {
		go func(cType types.CloudType, provider types.CloudProvider) {
			defer wg.Done()

			var cloudItems []*types.CloudItem
			for _, item := range items {
				if item.GetCloudType() == cType {
					cloudItems = append(cloudItems, &item)
				}
			}

			if len(cloudItems) > 0 {
				log.Infof("[TERMINATION] Terminating %d items on %s: %s", len(cloudItems), cType, items)
				var errors []error

				item := *cloudItems[0]
				switch t := item.GetItem().(type) {
				case types.Instance:
					errors = terminateInstances(provider, cloudItems)
				case types.Disk:
					errors = terminateDisks(provider, cloudItems)
				default:
					panic(fmt.Sprintf("[TERMINATION] Operation on type %T is not allowed", t))
				}

				for _, err := range errors {
					log.Errorf("[TERMINATION] Failed to terminate %ss on %s, err: %s", item.GetType(), cType, err.Error())
				}
			}
		}(t, p())
	}
	wg.Wait()
}

func terminateInstances(provider types.CloudProvider, items []*types.CloudItem) []error {
	log.Debugf("[TERMINATION] Instances to terminate (%d): [%s]", len(items), items)

	var instances []*types.Instance
	for _, item := range items {
		inst := (*item).GetItem().(types.Instance)
		instances = append(instances, &inst)
	}

	return provider.TerminateInstances(instances)
}

func terminateDisks(provider types.CloudProvider, items []*types.CloudItem) []error {
	log.Debugf("[TERMINATION] Disks to terminate (%d): [%s]", len(items), items)

	var disks []*types.Disk
	for _, item := range items {
		disk := (*item).GetItem().(types.Disk)
		disks = append(disks, &disk)
	}

	return provider.DeleteDisks(disks)
}
