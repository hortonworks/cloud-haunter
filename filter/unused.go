package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Filters[types.UnusedFilter] = unused{}
}

type unused struct {
}

func (f unused) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[UNUSED] Filtering items (%d): [%s]", len(items), items)
	return filter("UNUSED", items, types.ExclusiveFilter, func(item types.CloudItem) bool {
		switch item.GetItem().(type) {
		case types.Disk:
			if item.GetItem().(types.Disk).State != types.Unused {
				log.Debugf("[UNUSED] Filter disk, because it's in use: %s", item.GetName())
				return false
			}
		case types.Alert:
			if item.GetItem().(types.Alert).State != types.Unused {
				log.Debugf("[UNUSED] Filter alert, because it's in use: %s", item.GetName())
				return false
			}
		default:
			log.Fatalf("[UNUSED] Filter does not apply for cloud item: %s", item.GetName())
		}
		log.Debugf("[UNUSED] Item was not filtered: %s", item.GetName())
		return true
	})
}
