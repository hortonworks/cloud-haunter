package operation

import (
	"sync"

	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
)

var providers = ctx.CloudProviders

func collect(clouds []types.CloudType, getter func(types.CloudProvider) ([]types.CloudItem, error)) (chan []types.CloudItem, chan error) {
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
		if isNeeded(item) && !isIgnored(item, ctx.Ignores) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func isIgnored(item types.CloudItem, ignores *types.Ignores) bool {
	switch item.GetItem().(type) {
	case types.Instance:
		inst := item.GetItem().(types.Instance)
		if utils.IsAnyMatch(inst.Tags, ctx.IgnoreLabels[item.GetCloudType()]) {
			return true
		}
		if ignores != nil {
			switch item.GetCloudType() {
			case types.AWS:
				return isMatchWithIgnores(item, inst.Tags,
					ignores.Instance.Aws.Names, ignores.Instance.Aws.Owners, ignores.Instance.Aws.Labels)
			case types.AZURE:
				return isMatchWithIgnores(item, inst.Tags,
					ignores.Instance.Azure.Names, ignores.Instance.Azure.Owners, ignores.Instance.Azure.Labels)
			case types.GCP:
				return isMatchWithIgnores(item, inst.Tags,
					ignores.Instance.Gcp.Names, ignores.Instance.Gcp.Owners, ignores.Instance.Gcp.Labels)
			default:
				log.Warnf("[OPERATION] Cloud type not supported: ", item.GetCloudType())
			}
		}
	case types.Access:
		if ignores != nil {
			switch item.GetCloudType() {
			case types.AWS:
				return isNameOrOwnerMatch(item, ignores.Access.Aws.Names, ignores.Access.Aws.Owners)
			case types.AZURE:
				return isNameOrOwnerMatch(item, ignores.Access.Azure.Names, ignores.Access.Azure.Owners)
			case types.GCP:
				return isNameOrOwnerMatch(item, ignores.Access.Gcp.Names, ignores.Access.Gcp.Owners)
			default:
				log.Warnf("[OPERATION] Cloud type not supported: ", item.GetCloudType())
			}
		}
	}
	return false
}

func isMatchWithIgnores(item types.CloudItem, tags map[string]string, names, owners []string, labels []string) bool {
	if isNameOrOwnerMatch(item, names, owners) || utils.IsAnyStartsWith(tags, labels...) {
		return true
	}
	return false
}

func isNameOrOwnerMatch(item types.CloudItem, names, owners []string) bool {
	return utils.IsStartsWith(item.GetName(), names...) || utils.IsStartsWith(item.GetOwner(), owners...)
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
