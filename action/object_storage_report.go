package action

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Actions[types.StorageReportAction] = new(StorageReportAction)
}

type StorageReportAction struct {
}

func (a StorageReportAction) Execute(op types.OpType, filter []types.FilterType, items []types.CloudItem) {
	for _, item := range items {
		log.Infof("[%s] %s", item.GetCloudType(), item.GetName())
	}
}
