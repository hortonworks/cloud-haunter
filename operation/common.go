package operation

import (
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func collect(clouds []types.CloudType, getter func(types.CloudProvider) ([]types.CloudItem, error)) (chan []types.CloudItem, chan error) {
	itemsChan := make(chan []types.CloudItem, 10)
	errChan := make(chan error, 5)
	wg := sync.WaitGroup{}
	wg.Add(len(clouds))
	for _, c := range clouds {
		go func(cloud types.CloudType) {
			defer wg.Done()

			items, err := getter(context.CloudProviders[cloud]())
			if err != nil {
				errChan <- err
			}
			itemsChan <- items
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
	exit := false
	for !exit {
		select {
		case items, ok := <-itemsChan:
			if !ok {
				exit = true
				break
			}
			allItems = append(allItems, items...)
		case err, ok := <-errChan:
			if !ok {
				exit = true
				break
			}
			log.Errorf(errorMsg+", err: %s", err.Error())
		}
	}
	return allItems
}

func filter(items []types.CloudItem, isNeeded func(types.CloudItem) bool) []types.CloudItem {
	filtered := []types.CloudItem{}
	for _, item := range items {
		if isNeeded(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func collectRunningInstances(clouds []types.CloudType) (chan []types.CloudItem, chan error) {
	return collect(clouds, func(provider types.CloudProvider) ([]types.CloudItem, error) {
		instances, err := provider.GetRunningInstances()
		if err != nil {
			return nil, err
		}
		return convertInstancesToCloudItems(instances), nil
	})
}

func convertInstancesToCloudItems(instances []*types.Instance) []types.CloudItem {
	items := []types.CloudItem{}
	for _, inst := range instances {
		items = append(items, inst)
	}
	return items
}
