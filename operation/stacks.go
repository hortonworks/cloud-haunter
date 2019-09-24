package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Operations[types.Stacks] = stacks{}
}

type stacks struct {
}

func (o stacks) Execute(clouds []types.CloudType) []types.CloudItem {
	log.Debugf("[GET_STACKS] Collecting stacks on: [%s]", clouds)
	itemsChan, errChan := collectStacks(clouds)
	return wait(itemsChan, errChan, "[GET_STACKS] Failed to collect stacks")
}
