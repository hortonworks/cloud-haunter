package operation

import (
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
)

func init() {
	ctx.Filters[types.MatchFilter] = match{}
}

type match struct {
}

func (f match) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[MATCH] Filtering items (%d): [%s]", len(items), items)
	return filter("MATCH", items, types.InclusiveFilter, func(item types.CloudItem) bool {
		return true
	})
}
