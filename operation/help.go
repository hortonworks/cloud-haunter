package operation

import "github.com/hortonworks/cloud-cost-reducer/types"

func init() {
	types.Operations[types.HELP] = Help{}
}

type Help struct {
}

func (o Help) Execute() {
	println("Supported operations:")
	for ot := range types.Operations {
		println("-op=" + ot.String())
	}
}
