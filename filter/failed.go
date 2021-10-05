package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Filters[types.FailedFilter] = failed{}
}

type failed struct {
}

func (f failed) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[FAILED] Filtering instances (%d): [%s]", len(items), items)
	return filter("FAILED", items, types.ExclusiveFilter, func(item types.CloudItem) bool {
		switch item.GetItem().(type) {
		case types.Stack:
			if item.GetItem().(types.Stack).State != types.Failed {
				log.Debugf("[FAILED] Filter stack, because it's not in FAILED state: %s", item.GetName())
				return false
			}
		default:
			log.Fatalf("[FAILED] Filter does not apply for cloud item: %s", item.GetName())
		}
		return true
	})
}
