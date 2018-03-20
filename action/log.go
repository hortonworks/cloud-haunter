package action

import (
	"encoding/json"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	context.Actions[types.LOG_ACTION] = new(LogAction)
}

type LogAction struct {
}

func (a LogAction) Execute(op string, instances []*types.Instance) {
	capitalOp := strings.ToUpper(op)
	for _, inst := range instances {
		out, _ := json.Marshal(inst)
		log.Infof("[%s] %s", capitalOp, string(out))
	}
}
