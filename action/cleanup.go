package action

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

var defaultRetentionDays = 90

type cleanupAction struct {
	retentionDays int
}

func init() {
	initCleanup()
}

func initCleanup() {
	retentionDays := defaultRetentionDays
	retentionEnv := os.Getenv("RETENTION_DAYS")
	if len(retentionEnv) > 0 {
		parsed, err := strconv.Atoi(retentionEnv)
		if err != nil {
			log.Fatalf("[CLEANUP] Failed to parse RETENTION_DAYS, err: %s", err)
		}
		retentionDays = parsed
	}
	ctx.Actions[types.CleanupAction] = cleanupAction{retentionDays}
}

func (a cleanupAction) Execute(op types.OpType, filters []types.FilterType, items []types.CloudItem) {
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
				log.Infof("[CLEANUP] Cleaning up %d items on %s: %s", len(cloudItems), cType, items)
				var errors []error

				item := *cloudItems[0]
				switch t := item.GetItem().(type) {
				case types.Storage:
					errors = a.cleanupStorages(provider, cloudItems)
				default:
					panic(fmt.Sprintf("[CLEANUP] Operation on type %T is not allowed", t))
				}

				if len(errors) != 0 {
					for _, err := range errors {
						log.Errorf("[CLEANUP] Failed to clean up %ss on %s, err: %s", item.GetType(), cType, err.Error())
					}
					panic(fmt.Sprintf("[CLEANUP] Failed to clean up %ss on %s", item.GetType(), cType))
				}
			}
		}(t, p())
	}
	wg.Wait()
}

func (a cleanupAction) cleanupStorages(provider types.CloudProvider, items []*types.CloudItem) []error {
	var storages []*types.Storage
	for _, item := range items {
		storage := (*item).GetItem().(types.Storage)
		storages = append(storages, &storage)
	}
	return provider.CleanupStorages(types.NewStorageContainer(storages), a.retentionDays)
}
