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

func (m *instancesMessage) HTMLMessage() string {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("<b>Operation: %s</b><ul>", m.op.String()))
	for _, inst := range m.Instances {
		owner := inst.Owner
		if len(owner) == 0 {
			owner = "???"
		}
		buffer.WriteString(fmt.Sprintf("<li>[%s] instance name: <b>%s</b> created: %s owner: <b>%s</b></li>", inst.CloudType, inst.Name, inst.Created, owner))
	}
	buffer.WriteString("</ul>")
	return buffer.String()
}
