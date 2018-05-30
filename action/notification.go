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

func (a NotificationAction) Execute(op *types.OpType, instances []*types.Instance) {
	if len(instances) > 0 {
		message := instancesMessage{instances, op}
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
	Instances []*types.Instance `json:"Instances"`
	op        *types.OpType
}

func (m *instancesMessage) TextMessage() string {
	var buffer bytes.Buffer
	buffer.WriteString("/code\n")
	for _, instance := range m.Instances {
		owner := instance.Owner
		if len(owner) == 0 {
			owner = "???"
		}
		buffer.WriteString(fmt.Sprintf("[%s] instance name: %s created: %s owner: %s region: %s\n", instance.CloudType, instance.Name, instance.Created, owner, instance.Region))
	}
	return buffer.String()
}
