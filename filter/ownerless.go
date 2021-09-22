package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
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
		match := len(item.GetOwner()) == 0 || item.GetOwner() == "???"
		log.Infof("[OWNERLESS] %T: %s match: %v", item, item.GetName(), match)
		return match
	})
}
