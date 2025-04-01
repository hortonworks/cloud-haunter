package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Operations[types.Resources] = resources{}
}

type resources struct {
}

func (o resources) Execute(clouds []types.CloudType) []types.CloudItem {
	log.Debugf("[GET_RESOURCES] Collecting resources on: [%s]", clouds)
	itemsChan, errChan := collectResources(clouds)
	return wait(itemsChan, errChan, "[GET_RESOURCES] Failed to collect resources")
}
