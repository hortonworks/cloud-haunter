package operation

import (
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func collectInstances(clouds []types.CloudType,
	getInstances func(types.CloudProvider) ([]*types.Instance, error)) (chan []*types.Instance, chan error) {
	instsChan := make(chan []*types.Instance, 10)
	errChan := make(chan error, 5)
	wg := sync.WaitGroup{}
	wg.Add(len(clouds))
	for _, c := range clouds {
		go func(cloud types.CloudType) {
			defer wg.Done()

			provider := context.CloudProviders[cloud]
			instances, err := getInstances(provider)
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

func waitForInstances(instsChan chan []*types.Instance, errChan chan error, errorMsg string,
	filter func([]*types.Instance) []*types.Instance) []*types.Instance {
	var allInstances = make([]*types.Instance, 0)
	exit := false
	for !exit {
		select {
		case instances, ok := <-instsChan:
			if !ok {
				exit = true
				break
			}
			if filter != nil {
				instances = filter(instances)
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
