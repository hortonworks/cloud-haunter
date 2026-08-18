package action

import (
	"sync"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewNotification returns the notification action implementation.
func NewNotification(cfg *config.Config) types.Action {
	return notificationAction{cfg}
}

type notificationAction struct {
	cfg *config.Config
}

func (a notificationAction) Execute(op types.OpType, filters []types.FilterType, items []types.CloudItem) error {
	log.Infof("[NOTIFICATION] Sending %d items for %d dispatchers", len(items), len(a.cfg.Dispatchers))
	log.Debugf("[NOTIFICATION] Sending notifications (%d) for items: [%s]", len(items), items)
	if len(items) > 0 {
		wg := sync.WaitGroup{}
		wg.Add(len(a.cfg.Dispatchers))
		for n, d := range a.cfg.Dispatchers {
			log.Debugf("[NOTIFICATION] Using %s to dispatch dispatch notifications", d.GetName())
			go func(name string, dispatcher types.Dispatcher) {
				defer wg.Done()

				if err := dispatcher.Send(op, filters, items); err != nil {
					log.Errorf("[%s] Failed to send message, err: %s", name, err.Error())
				}
			}(n, d)
		}
		wg.Wait()
	}
	return nil
}
