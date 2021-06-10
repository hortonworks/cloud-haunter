package operation

import (
	ctx "github.com/blentz/cloud-haunter/context"
	"github.com/blentz/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Filters[types.NoMatchFilter] = noMatch{}
}

type noMatch struct {
}

func (f noMatch) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[NO_MATCH] Filtering items (%d): [%s]", len(items), items)
	return filter("NO_MATCH", items, types.ExclusiveFilter, func(item types.CloudItem) bool {
		return true
	})
}
