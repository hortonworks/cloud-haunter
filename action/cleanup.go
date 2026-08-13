package action

import (
	"errors"
	"fmt"
	"sync"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

type cleanupAction struct {
	cfg *config.Config
}

// NewCleanup returns the cleanup action implementation. The retention period is
// taken from cfg.RetentionDays (resolved from RETENTION_DAYS when the config is
// built).
func NewCleanup(cfg *config.Config) types.Action {
	return cleanupAction{cfg}
}

func (a cleanupAction) Execute(op types.OpType, filters []types.FilterType, items []types.CloudItem) error {
	wg := sync.WaitGroup{}
	wg.Add(len(a.cfg.CloudProviders))
	errChan := make(chan error, len(a.cfg.CloudProviders))
	for t, p := range a.cfg.CloudProviders {
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
				log.Infof("[CLEANUP] Cleaning up %d items on %s: %s", len(cloudItems), cType, items)
				var errs []error

				item := *cloudItems[0]
				switch t := item.GetItem().(type) {
				case types.Storage:
					errs = a.cleanupStorages(provider, cloudItems)
				default:
					errChan <- fmt.Errorf("[CLEANUP] operation on type %T is not allowed", t)
					return
				}

				if len(errs) != 0 {
					for _, err := range errs {
						log.Errorf("[CLEANUP] Failed to clean up %ss on %s, err: %s", item.GetType(), cType, err.Error())
					}
					errChan <- fmt.Errorf("[CLEANUP] failed to clean up %ss on %s", item.GetType(), cType)
				}
			}
		}(t, p())
	}
	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (a cleanupAction) cleanupStorages(provider types.CloudProvider, items []*types.CloudItem) []error {
	var storages []*types.Storage
	for _, item := range items {
		storage := (*item).GetItem().(types.Storage)
		storages = append(storages, &storage)
	}
	return provider.CleanupStorages(types.NewStorageContainer(storages), a.cfg.RetentionDays)
}
