package operation

import (
	ctx "github.com/blentz/cloud-haunter/context"
	"github.com/blentz/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
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
