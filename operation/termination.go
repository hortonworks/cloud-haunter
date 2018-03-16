package operation

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	context.Operations[types.TERMINATION] = Termination{}
}

type Termination struct {
}

func (o Termination) Execute(clouds []types.CloudType) {
	for _, cloud := range clouds {
		provider, ok := context.CloudProviders[cloud]
		if !ok {
			panic("Cloud provider not supported")
		}
		ownerLessInstances := provider.TerminateRunningInstances()
		for _, instance := range ownerLessInstances {
			log.Infof("[%s] Instance %s is running without Owner tag", cloud.String(), instance.Name)
		}

	}
}
