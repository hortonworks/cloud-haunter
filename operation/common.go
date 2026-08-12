package operation

import (
	"sync"

	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func collect(providers map[types.CloudType]func() types.CloudProvider, clouds []types.CloudType, getter func(types.CloudProvider) ([]types.CloudItem, error)) (chan []types.CloudItem, chan error) {
	itemsChan := make(chan []types.CloudItem, 10)
	errChan := make(chan error, 5)
	wg := sync.WaitGroup{}
	wg.Add(len(clouds))
	for _, c := range clouds {
		go func(cloud types.CloudType) {
			defer wg.Done()

			if items, err := getter(providers[cloud]()); err != nil {
				errChan <- err
			} else {
				itemsChan <- items
			}
		}(c)
	}
	go func() {
		wg.Wait()
		close(itemsChan)
		close(errChan)
	}()
	return itemsChan, errChan
}

func wait(itemsChan chan []types.CloudItem, errChan chan error, errorMsg string) []types.CloudItem {
	allItems := make([]types.CloudItem, 0)
	// collect closes both channels once every provider has finished. Keep reading
	// until BOTH are closed so buffered items are never dropped: a closed channel
	// is set to nil, which disables its select case (a receive on nil blocks
	// forever) while the other channel continues to drain.
	for itemsChan != nil || errChan != nil {
		select {
		case items, ok := <-itemsChan:
			if !ok {
				itemsChan = nil
				continue
			}
			allItems = append(allItems, items...)
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			log.Errorf(errorMsg+", err: %s", err.Error())
		}
	}
	return allItems
}

func collectInstances(providers map[types.CloudType]func() types.CloudProvider, clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(providers, clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		instances, err := provider.GetInstances()
		if err != nil {
			return nil, err
		}
		return convertInstancesToCloudItems(instances), nil
	})
}

func collectStacks(providers map[types.CloudType]func() types.CloudProvider, clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(providers, clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		stacks, err := provider.GetStacks()
		if err != nil {
			return nil, err
		}
		return convertStacksToCloudItems(stacks), nil
	})
}

func collectResources(providers map[types.CloudType]func() types.CloudProvider, clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(providers, clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		resources, err := provider.GetResources()
		if err != nil {
			return nil, err
		}
		return convertResourcesToCloudItems(resources), nil
	})
}

func convertInstancesToCloudItems(instances []*types.Instance) []types.CloudItem {
	var items []types.CloudItem
	for _, inst := range instances {
		items = append(items, inst)
	}
	return items
}

func convertToCloudItems(images []*types.Image) []types.CloudItem {
	var items []types.CloudItem
	for _, img := range images {
		items = append(items, img)
	}
	return items
}

func convertStacksToCloudItems(stacks []*types.Stack) []types.CloudItem {
	var items []types.CloudItem
	for _, stack := range stacks {
		items = append(items, stack)
	}
	return items
}

func convertResourcesToCloudItems(resources []*types.Resource) []types.CloudItem {
	var items []types.CloudItem
	for _, resource := range resources {
		items = append(items, resource)
	}
	return items
}
