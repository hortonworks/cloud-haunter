package action

import (
	"bytes"
	"fmt"

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

func (a NotificationAction) Execute(op string, instances []*types.Instance) {
	if len(instances) > 0 {
		message := instancesMessage{instances: instances}
		for name, dispatcher := range ctx.Dispatchers {
			if err := dispatcher.Send(&message); err != nil {
				log.Errorf("[%s] Failed to send message, err: %s", name, err.Error())
			}
		}
	}
}

type instancesMessage struct {
	instances []*types.Instance
}

func (m *instancesMessage) Message() string {
	var buffer bytes.Buffer
	buffer.WriteString("/code\n")
	for _, instance := range m.instances {
		buffer.WriteString(fmt.Sprintf("[%s] instance name: %s created: %s\n", instance.CloudType, instance.Name, instance.Created))
	}
	return buffer.String()
}
