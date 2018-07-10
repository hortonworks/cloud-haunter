package action

import (
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	context.Actions[types.NOTIFICATION_ACTION] = new(notificationAction)
}

type notificationAction struct {
}

func (a notificationAction) Execute(op *types.OpType, items []types.CloudItem) {
	log.Infof("[NOTIFICATION] Sending %d items for %d dispatchers", len(items), len(ctx.Dispatchers))
	if context.DryRun {
		log.Debugf("[NOTIFICATION] Sending notifications (%d) for items: [%s]", len(items), items)
	}
	if len(items) > 0 {
		wg := sync.WaitGroup{}
		wg.Add(len(ctx.Dispatchers))
		for n, d := range ctx.Dispatchers {
			if context.DryRun {
				log.Debugf("[NOTIFICATION] Using %s to dispatch dispatch notifications", d.GetName())
			}
			go func(name string, dispatcher types.Dispatcher) {
				defer wg.Done()

				if err := dispatcher.Send(op, items); err != nil {
					log.Errorf("[%s] Failed to send message, err: %s", name, err.Error())
				}
			}(n, d)
		}
		wg.Wait()
	}
}
