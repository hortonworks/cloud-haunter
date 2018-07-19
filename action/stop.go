package action

import (
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"sync"
)

func init() {
	ctx.Actions[types.StopAction] = new(stopAction)
}

type stopAction struct {
}

func (s stopAction) Execute(op *types.OpType, items []types.CloudItem) {
	instancesPerCloud := map[types.CloudType][]*types.Instance{}
	for _, item := range items {
		switch t := item.GetItem().(type) {
		case types.Instance:
			instancesPerCloud[item.GetCloudType()] = append(instancesPerCloud[item.GetCloudType()], item.(*types.Instance))
		default:
			log.Printf("[STOP} Ignoring cloud item: %s, because it's not an instance, but a %s", t, item.GetType())
		}
	}

	wg := sync.WaitGroup{}
	wg.Add(len(instancesPerCloud))
	for cloud, instances := range instancesPerCloud {
		go func(cloud types.CloudType, instances []*types.Instance) {
			defer wg.Done()
			log.Printf("[STOP] Stop %d instances on cloud: %s", len(instances), cloud)
			if err := ctx.CloudProviders[cloud]().StopInstances(instances); err != nil {
				log.Errorf("[STOP] Failed to stop instances on cloud: %s, err: %s", cloud, err.Error())
			}
		}(cloud, instances)
	}

	wg.Wait()
}
