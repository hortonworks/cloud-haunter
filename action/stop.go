package action

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewStop returns the stop action implementation.
func NewStop(cfg *config.Config) types.Action {
	return stopAction{cfg}
}

type stopAction struct {
	cfg *config.Config
}

func (s stopAction) Execute(_ types.OpType, _ []types.FilterType, items []types.CloudItem) error {
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
	errChan := make(chan error, len(instancesPerCloud)+len(databasesPerCloud))
	if len(instancesPerCloud) > 0 {
		wg.Add(len(instancesPerCloud))
		stopInstances(s.cfg.CloudProviders, instancesPerCloud, &wg, errChan)
	}
	if len(databasesPerCloud) > 0 {
		wg.Add(len(databasesPerCloud))
		stopDatabases(s.cfg.CloudProviders, databasesPerCloud, &wg, errChan)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func stopInstances(providers map[types.CloudType]func() types.CloudProvider, instancesPerCloud map[types.CloudType][]*types.Instance, wg *sync.WaitGroup, errChan chan<- error) {
	for cloud, instances := range instancesPerCloud {
		go func(cloud types.CloudType, instances []*types.Instance) {
			defer wg.Done()
			log.Infof("[STOP] Stop %d instances on %s: %s", len(instances), cloud, strings.Join(getInstanceNames(instances), ","))
			if errs := providers[cloud]().StopInstances(types.NewInstanceContainer(instances)); len(errs) != 0 {
				for _, err := range errs {
					log.Errorf("[STOP] Failed to stop instances on cloud: %s, err: %s", cloud, err.Error())
				}
				errChan <- fmt.Errorf("[STOP] failed to stop instances on cloud: %s", cloud)
			}
		}(cloud, instances)
	}
}

func stopDatabases(providers map[types.CloudType]func() types.CloudProvider, databasesPerCloud map[types.CloudType][]*types.Database, wg *sync.WaitGroup, errChan chan<- error) {
	for cloud, databases := range databasesPerCloud {
		go func(cloud types.CloudType, databases []*types.Database) {
			defer wg.Done()
			log.Infof("[STOP] Stop %d databases on %s: %s", len(databases), cloud, strings.Join(getDatabaseNames(databases), ","))
			if errs := providers[cloud]().StopDatabases(types.NewDatabaseContainer(databases)); len(errs) != 0 {
				for _, err := range errs {
					log.Errorf("[STOP] Failed to stop databases on cloud: %s, err: %s", cloud, err.Error())
				}
				errChan <- fmt.Errorf("[STOP] failed to stop databases on cloud: %s", cloud)
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
