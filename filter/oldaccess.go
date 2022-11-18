package operation

import (
	"os"
	"time"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

var defaultAvailablePeriod = 120 * 24 * time.Hour

type oldAccess struct {
	availablePeriod time.Duration
}

func init() {
	availableEnv := os.Getenv("ACCESS_AVAILABLE_PERIOD")
	var availablePeriod time.Duration
	if len(availableEnv) > 0 {
		duration, err := time.ParseDuration(availableEnv)
		if err != nil {
			log.Errorf("[OLDACCESS] err: %s", err)
			return
		}
		availablePeriod = duration
	} else {
		availablePeriod = defaultAvailablePeriod
	}
	log.Infof("[OLDACCESS] running period set to: %s", availablePeriod)
	ctx.Filters[types.OldAccessFilter] = oldAccess{availablePeriod}
}

func (f oldAccess) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("[OLDACCESS] Filtering items (%d): [%s]", len(items), items)
	return filter("OLDACCESS", items, types.ExclusiveFilter, func(item types.CloudItem) bool {
		switch item.GetItem().(type) {
		case types.Access:
			match := item.GetCreated().Add(f.availablePeriod).Before(time.Now())
			log.Debugf("[OLDACCESS] Access: %s match: %v", item.GetName(), match)
			return match
		default:
			log.Fatalf("[OLDACCESS] Filter does not apply for cloud item: %s", item.GetName())
		}
		return true
	})
}
