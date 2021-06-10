package operation

import (
	ctx "github.com/blentz/cloud-haunter/context"
	"github.com/blentz/cloud-haunter/types"
	"github.com/blentz/cloud-haunter/utils"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Filters[types.OwnerlessFilter] = ownerless{}
}

type ownerless struct {
}

func (o ownerless) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[OWNERLESS] Filtering instances without tag %s (%d): [%s]", ctx.OwnerLabel, len(items), items)
	return filter("OWNERLESS", items, types.ExclusiveFilter, func(item types.CloudItem) bool {
		switch item.GetItem().(type) {
		case types.Instance:
			inst := item.(*types.Instance)
			match := !utils.IsAnyMatch(inst.Tags, ctx.OwnerLabel)
			log.Debugf("[OWNERLESS] Instance: %s match: %v", inst.Name, match)
			return match
		case types.Stack:
			stack := item.(*types.Stack)
			match := !utils.IsAnyMatch(stack.Tags, ctx.OwnerLabel)
			log.Debugf("[OWNERLESS] Stack: %s match: %v", stack.Name, match)
			return match
		default:
			log.Fatalf("[OWNERLESS] Filter does not apply for cloud item: %s", item.GetName())
		}
		return true
	})
}
