package operation

import (
	"errors"
	"fmt"
	"strings"
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
				errChan <- withCloudPrefix(cloud, err)
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

// withCloudPrefix makes sure the error names its cloud, the way most provider errors already do: "[AWS] ...".
func withCloudPrefix(cloud types.CloudType, err error) error {
	prefix := "[" + cloud.String() + "]"
	if strings.HasPrefix(err.Error(), prefix) {
		return err
	}
	return fmt.Errorf("%s %w", prefix, err)
}

// wait collects the items of every provider. If any provider failed, it returns every error and no items, so an
// action is never run on an inventory that silently misses a provider. A provider that does not support the operation
// has not failed, its error is only logged.
func wait(itemsChan chan []types.CloudItem, errChan chan error, errorMsg string) ([]types.CloudItem, error) {
	allItems := make([]types.CloudItem, 0)
	var errs []error
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
			if errors.Is(err, types.ErrNotSupported) {
				log.Warnf("Skipping a cloud that does not support the operation: %s", err.Error())
				continue
			}
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("%s: %w", errorMsg, errors.Join(errs...))
	}
	return allItems, nil
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
