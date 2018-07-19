package operation

import (
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
)

func init() {
	ctx.Operations[types.Running] = running{}
}

type running struct {
}

func (o running) Execute(clouds []types.CloudType) []types.CloudItem {
	log.Debugf("Collecting long running instances on: [%s]", clouds)
	itemsChan, errChan := collectRunningInstances(clouds)
	return wait(itemsChan, errChan, "[RUNNING] Failed to collect running instances")
}
