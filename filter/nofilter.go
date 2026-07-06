package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Filters[types.NoFilter] = nofilter{}
}

type nofilter struct {
}

func (f nofilter) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[NOFILTER] Filtering items (%d): [%s]", len(items), items)
	return items
}
