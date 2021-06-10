package action

import (
	"fmt"
	"strings"
	"sync"

	ctx "github.com/blentz/cloud-haunter/context"
	"github.com/blentz/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Actions[types.StopAction] = new(stopAction)
}

type stopAction struct {
}

func (s stopAction) Execute(_ types.OpType, _ []types.FilterType, items []types.CloudItem) {
	instancesPerCloud := map[types.CloudType][]*types.Instance{}
	databasesPerCloud := map[types.CloudType][]*types.Database{}
	for _, item := range items {
		switch t := item.GetItem().(type) {
		case types.Instance:
			instancesPerCloud[item.GetCloudType()] = append(instancesPerCloud[item.GetCloudType()], item.(*types.Instance))
		case types.Database:
			databasesPerCloud[item.GetCloudType()] = append(databasesPerCloud[item.GetCloudType()], item.(*types.Database))
		default:
			log.Debugf("[STOP] Ignoring cloud item: %s, because it's not a stoppable resource: %s", t, item.GetType())
		}
	}

	wg := sync.WaitGroup{}
	if len(instancesPerCloud) > 0 {
		wg.Add(len(instancesPerCloud))
		stopInstances(instancesPerCloud, &wg)
	}
	if len(databasesPerCloud) > 0 {
		wg.Add(len(databasesPerCloud))
		stopDatabases(databasesPerCloud, &wg)
	}

	wg.Wait()
}

func stopInstances(instancesPerCloud map[types.CloudType][]*types.Instance, wg *sync.WaitGroup) {
	for cloud, instances := range instancesPerCloud {
		go func(cloud types.CloudType, instances []*types.Instance) {
			defer wg.Done()
			log.Infof("[STOP] Stop %d instances on %s: %s", len(instances), cloud, strings.Join(getInstanceNames(instances), ","))
			if errors := ctx.CloudProviders[cloud]().StopInstances(types.NewInstanceContainer(instances)); len(errors) != 0 {
				for _, err := range errors {
					log.Errorf("[STOP] Failed to stop instances on cloud: %s, err: %s", cloud, err.Error())
				}
				panic(fmt.Sprintf("[STOP] Failed to stop instances on cloud: %s", cloud))
			}
		}(cloud, instances)
	}
}

func stopDatabases(databasesPerCloud map[types.CloudType][]*types.Database, wg *sync.WaitGroup) {
	for cloud, databases := range databasesPerCloud {
		go func(cloud types.CloudType, databases []*types.Database) {
			defer wg.Done()
			log.Infof("[STOP] Stop %d databases on %s: %s", len(databases), cloud, strings.Join(getDatabaseNames(databases), ","))
			if errors := ctx.CloudProviders[cloud]().StopDatabases(types.NewDatabaseContainer(databases)); len(errors) != 0 {
				for _, err := range errors {
					log.Errorf("[STOP] Failed to stop databases on cloud: %s, err: %s", cloud, err.Error())
				}
				panic(fmt.Sprintf("[STOP] Failed to stop databases on cloud: %s", cloud))
			}
		}(cloud, databases)
	}
}

func getInstanceNames(instances []*types.Instance) []string {
	result := make([]string, len(instances))
	for i, inst := range instances {
		result[i] = fmt.Sprintf("%s:%s", inst.ID, inst.Name)
	}
	return result
}

func getDatabaseNames(databases []*types.Database) []string {
	result := make([]string, len(databases))
	for i, db := range databases {
		result[i] = fmt.Sprintf("%s:%s", db.ID, db.Name)
	}
	return result
}
