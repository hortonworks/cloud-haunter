package operation

import (
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
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
		default:
			return true
		}
		return true
	})
}
