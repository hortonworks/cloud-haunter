package action

import (
	"sync"

	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
)

func init() {
	ctx.Actions[types.NotificationAction] = new(notificationAction)
}

type notificationAction struct {
}

func (a notificationAction) Execute(op types.OpType, filters []types.FilterType, items []types.CloudItem) {
	log.Infof("[NOTIFICATION] Sending %d items for %d dispatchers", len(items), len(ctx.Dispatchers))
	log.Debugf("[NOTIFICATION] Sending notifications (%d) for items: [%s]", len(items), items)
	if len(items) > 0 {
		wg := sync.WaitGroup{}
		wg.Add(len(ctx.Dispatchers))
		for n, d := range ctx.Dispatchers {
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
}
