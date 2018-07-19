package operation

import (
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
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
	ctx.Filters[types.OldAccess] = oldAccess{availablePeriod}
}

func (f oldAccess) Execute(items []types.CloudItem) []types.CloudItem {
	log.Debugf("Filtering accesses (%d): [%s]", len(items), items)
	return filter(items, func(item types.CloudItem) bool {
		match := item.GetCreated().Add(f.availablePeriod).Before(time.Now())
		log.Debugf("Access: %s match: %b", item.GetName(), match)
		return match
	})
}
