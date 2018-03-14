package operation

import (
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
		provider.TerminateRunningInstances()
	}
}
