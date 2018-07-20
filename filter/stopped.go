package operation

import (
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
)

func init() {
	ctx.Filters[types.StoppedFilter] = stopped{}
}

type stopped struct {
}

func (f stopped) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[STOPPED] Filtering instances (%d): [%s]", len(items), items)
	return filter(items, func(item types.CloudItem) bool {
		if !isInstance(item) {
			log.Debugf("[STOPPED] Filter does not apply for cloud item: %s", item.GetName())
			return true
		}
		if item.GetItem().(types.Instance).State != types.Stopped {
			log.Debugf("[STOPPED] Filter instance, because it's not in STOPPED state: %s", item.GetName())
			return false
		}
		return true
	})
}
