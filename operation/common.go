package operation

import (
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func collectRunningInstances(clouds []types.CloudType) (chan []*types.Instance, chan error) {
	instsChan := make(chan []*types.Instance, 10)
	errChan := make(chan error, 5)
	wg := sync.WaitGroup{}
	wg.Add(len(clouds))
	for _, c := range clouds {
		go func(cloud types.CloudType) {
			defer wg.Done()

			provider := context.CloudProviders[cloud]()
			instances, err := provider.GetRunningInstances()
			if err != nil {
				errChan <- err
			}
			instsChan <- instances
		}(c)
	}
	go func() {
		wg.Wait()
		close(instsChan)
		close(errChan)
	}()
	return instsChan, errChan
}

func waitForInstances(instsChan chan []*types.Instance, errChan chan error, errorMsg string) []*types.Instance {
	var allInstances = make([]*types.Instance, 0)
	exit := false
	for !exit {
		select {
		case instances, ok := <-instsChan:
			if !ok {
				exit = true
				break
			}
			allInstances = append(allInstances, instances...)
		case err, ok := <-errChan:
			if !ok {
				exit = true
				break
			}
			log.Errorf(errorMsg+", err: %s", err.Error())
		}
	}
	return allInstances
}

func filter(instances []*types.Instance, isNeeded func(*types.Instance) bool) []*types.Instance {
	filtered := []*types.Instance{}
	for _, inst := range instances {
		if isNeeded(inst) {
			filtered = append(filtered, inst)
		}
	}
	return filtered
}
