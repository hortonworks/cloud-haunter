package action

import (
	"bytes"
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func init() {
	context.Actions[types.NOTIFICATION_ACTION] = new(NotificationAction)
}

type NotificationAction struct {
}

func (a NotificationAction) Execute(op *types.OpType, items []types.CloudItem) {
	if len(items) > 0 {
		message := instancesMessage{items, op}
		wg := sync.WaitGroup{}
		wg.Add(len(ctx.Dispatchers))
		for n, d := range ctx.Dispatchers {
			go func(name string, dispatcher types.Dispatcher) {
				defer wg.Done()

				if err := dispatcher.Send(&message); err != nil {
					log.Errorf("[%s] Failed to send message, err: %s", name, err.Error())
				}
			}(n, d)
		}
		wg.Wait()
	}
}

type instancesMessage struct {
	CloudItems []types.CloudItem `json:"CloudItem"`
	op         *types.OpType
}

func (m *instancesMessage) TextMessage(header string) string {
	var buffer bytes.Buffer
	buffer.WriteString(header)
	for _, item := range m.CloudItems {
		switch item.GetItem().(type) {
		case types.Instance:
			var inst types.Instance = item.GetItem().(types.Instance)
			buffer.WriteString(fmt.Sprintf("[%s] instance name: %s created: %s owner: %s region: %s\n", item.GetCloudType(), item.GetName(), item.GetCreated(), item.GetOwner(), inst.Region))
		default:
			buffer.WriteString(fmt.Sprintf("[%s] %s: %s created: %s owner: %s\n", item.GetCloudType(), item.GetType(), item.GetName(), item.GetCreated(), item.GetOwner()))
		}
	}
	return buffer.String()
}
