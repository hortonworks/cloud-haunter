package action

import (
	"sync"

	"fmt"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
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
				case types.Disk:
					errors = deleteDisks(provider, cloudItems)
				case types.Image:
					errors = deleteImages(provider, cloudItems)
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
	var instances []*types.Instance
	for _, item := range items {
		inst := (*item).GetItem().(types.Instance)
		instances = append(instances, &inst)
	}
	return provider.TerminateInstances(types.NewInstanceContainer(instances))
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
