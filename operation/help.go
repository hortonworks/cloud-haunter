package operation

import (
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	context.Operations[types.HELP] = Help{}
}

type Help struct {
}

func (o Help) Execute(clouds []types.CloudType) {
	println("Supported operations:")
	for ot := range context.Operations {
		println("\t-o=" + ot.String())
	}
	println("Supported cloud providers:")
	for ct := range context.CloudProviders {
		println("\t-c=" + ct.String())
	}
	println("Dry run:\n\t-d")
}
