package action

import (
	"sync"

	"fmt"
	ctx "github.com/blentz/cloud-haunter/context"
	"github.com/blentz/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
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
					i := item
					cloudItems = append(cloudItems, &i)
				}
			}

			if len(cloudItems) > 0 {
				log.Infof("[TERMINATION] Terminating %d items on %s: %s", len(cloudItems), cType, items)
				var errors []error

				item := *cloudItems[0]
				switch t := item.GetItem().(type) {
				case types.Instance:
					errors = terminateInstances(provider, cloudItems)
				case types.Stack:
					errors = terminateStacks(provider, cloudItems)
				case types.Disk:
					errors = deleteDisks(provider, cloudItems)
				case types.Image:
					errors = deleteImages(provider, cloudItems)
				default:
					panic(fmt.Sprintf("[TERMINATION] Operation on type %T is not allowed", t))
				}

				if len(errors) != 0 {
					for _, err := range errors {
						log.Errorf("[TERMINATION] Failed to terminate %ss on %s, err: %s", item.GetType(), cType, err.Error())
					}
					panic(fmt.Sprintf("[TERMINATION] Failed to terminate %ss on %s", item.GetType(), cType))
				}
			}
		}(t, p())
	}
	wg.Wait()
}

func terminateInstances(provider types.CloudProvider, items []*types.CloudItem) []error {
	var instances []*types.Instance
	for _, item := range items {
		inst := (*item).GetItem().(types.Instance)
		instances = append(instances, &inst)
	}
	return provider.TerminateInstances(types.NewInstanceContainer(instances))
}

func terminateStacks(provider types.CloudProvider, items []*types.CloudItem) []error {
	var stacks []*types.Stack
	for _, item := range items {
		stack := (*item).GetItem().(types.Stack)
		stacks = append(stacks, &stack)
	}
	return provider.TerminateStacks(types.NewStackContainer(stacks))
}

func deleteDisks(provider types.CloudProvider, items []*types.CloudItem) []error {
	var disks []*types.Disk
	for _, item := range items {
		disk := (*item).GetItem().(types.Disk)
		disks = append(disks, &disk)
	}
	return provider.DeleteDisks(types.NewDiskContainer(disks))
}

func deleteImages(provider types.CloudProvider, items []*types.CloudItem) []error {
	var images []*types.Image
	for _, item := range items {
		image := (*item).GetItem().(types.Image)
		images = append(images, &image)
	}
	return provider.DeleteImages(types.NewImageContainer(images))
}
