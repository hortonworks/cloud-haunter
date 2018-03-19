package action

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	context.Actions[types.LOG] = new(LogAction)
}

type LogAction struct {
}

func (a LogAction) Execute(op string, instances []*types.Instance) {
	capitalOp := strings.ToUpper(op)
	for _, inst := range instances {
		log.Infof("[%s] %t", capitalOp, inst)
	}
}
