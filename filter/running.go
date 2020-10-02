package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Filters[types.RunningFilter] = running{}
}

type running struct {
}

func (f running) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[RUNNING] Filtering instances (%d): [%s]", len(items), items)
	return filter("RUNNING", items, types.ExclusiveFilter, func(item types.CloudItem) bool {
		switch item.GetItem().(type) {
		case types.Instance:
			if item.GetItem().(types.Instance).State != types.Running {
				log.Debugf("[RUNNING] Filter instance, because it's not in RUNNING state: %s", item.GetName())
				return false
			}
		default:
			log.Fatalf("[RUNNING] Filter does not apply for cloud item: %s", item.GetName())
		}
		return true
	})
}
