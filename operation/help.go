package operation

import "github.com/hortonworks/cloud-cost-reducer/types"

func init() {
	types.Operations[types.HELP] = Help{}
}

type Help struct {
}

func (o Help) Execute(clouds []types.CloudType) {
	println("Supported operations:")
	for ot := range types.Operations {
		println("-o=" + ot.String())
	}
	println("Supported cloud providers:")
	for ct := range types.CloudProviders {
		println("-c=" + ct.String())
	}
}
