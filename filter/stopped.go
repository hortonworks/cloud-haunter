package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Filters[types.StoppedFilter] = stopped{}
}

type stopped struct {
}

func (f stopped) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[STOPPED] Filtering items (%d): [%s]", len(items), items)
	return filter("RUNNING", items, types.ExclusiveFilter, func(item types.CloudItem) bool {
		switch item.GetItem().(type) {
		case types.Instance:
			if item.GetItem().(types.Instance).State != types.Stopped {
				log.Debugf("[STOPPED] Filter instance, because it's not in STOPPED state: %s", item.GetName())
				return false
			}
		default:
			log.Fatalf("[STOPPED] Filter does not apply for cloud item: %s", item.GetName())
		}
		return true
	})
}
