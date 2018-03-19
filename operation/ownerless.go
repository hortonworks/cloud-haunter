package operation

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	context.Operations[types.OWNERLESS] = Ownerless{}
}

type Ownerless struct {
}

func (o Ownerless) Execute(clouds []types.CloudType) {
	for _, cloud := range clouds {
		provider, ok := context.CloudProviders[cloud]
		if !ok {
			panic("Cloud provider not supported")
		}
		ownerLessInstances, err := provider.TerminateRunningInstances()
		if err != nil {
			continue
		}
		for _, instance := range ownerLessInstances {
			log.Infof("[%s] Instance %s was running without Owner tag", cloud.String(), instance.Name)
		}

	}
}
