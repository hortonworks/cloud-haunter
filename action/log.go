package action

import (
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	context.Actions[types.LOG] = new(LogAction)
}

type LogAction struct {
}

func (la LogAction) Execute() {
	println(999999)
}
