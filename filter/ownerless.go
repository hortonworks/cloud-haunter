package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Filters[types.OwnerlessFilter] = ownerless{}
}

type ownerless struct {
}

func (o ownerless) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[OWNERLESS] Filtering instances (%d): [%s]", len(items), items)
	return filter("OWNERLESS", items, types.ExclusiveFilter, func(item types.CloudItem) bool {
		if !isInstance(item) && !isStack(item) {
			log.Fatalf("[OWNERLESS] Filter does not apply for cloud item: %s", item.GetName())
			return true
		}
		var match bool
		inst, ok := item.(*types.Instance)
		if ok {
			match = !utils.IsAnyMatch(inst.Tags, ctx.OwnerLabel)
			log.Debugf("[OWNERLESS] Instance: %s match: %v", inst.Name, match)
		} else {
			stack := item.(*types.Stack)
			match = !utils.IsAnyMatch(stack.Tags, ctx.OwnerLabel)
			log.Debugf("[OWNERLESS] Stack: %s match: %v", stack.Name, match)
		}
		return match
	})
}
