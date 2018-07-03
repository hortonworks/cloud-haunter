package action

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	context.Actions[types.LOG_ACTION] = new(LogAction)
}

type LogAction struct {
}

func (a LogAction) Execute(op *types.OpType, items []types.CloudItem) {
	for _, item := range items {
		out, _ := json.Marshal(item.GetItem())
		log.Infof("[%s] %s", item.GetCloudType(), string(out))
	}
}
