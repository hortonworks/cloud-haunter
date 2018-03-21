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

func (a NotificationAction) Execute(instances []*types.Instance) {
	if len(instances) > 0 {
		message := instancesMessage{Instances: instances}
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
}

func (m *instancesMessage) Message() string {
	var buffer bytes.Buffer
	buffer.WriteString("/code\n")
	for _, instance := range m.Instances {
		buffer.WriteString(fmt.Sprintf("[%s] instance name: %s created: %s\n", instance.CloudType, instance.Name, instance.Created))
	}
	return buffer.String()
}
