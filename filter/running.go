package operation

import (
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
)

func init() {
	ctx.Filters[types.RunningFilter] = running{}
}

type running struct {
}

func (f running) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[RUNNING] Filtering instances (%d): [%s]", len(items), items)
	return filter(items, func(item types.CloudItem) bool {
		if isInstance(item) && item.GetItem().(types.Instance).State != types.Running {
			log.Debugf("[RUNNING] Filter instance, because it's not in RUNNING state: %s", item.GetName())
			return false
		}
		return true
	})
}
