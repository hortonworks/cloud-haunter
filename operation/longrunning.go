package operation

import (
	"time"

	"bytes"
	"fmt"

	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"os"
)

var runningPeriod = 24 * time.Hour

func init() {
	ctx.Operations[types.LONGRUNNING] = LongRunning{}
	running := os.Getenv("RUNNING_PERIOD")
	if len(running) > 0 {
		if duration, err := time.ParseDuration(running); err != nil {
			log.Warnf("[LONGRUNNING] err: %s", err)
		} else {
			runningPeriod = duration
		}
	}
	log.Infof("[LONGRUNNING] running period set to: %s", runningPeriod)
}

type LongRunning struct {
}

type longRunningInstancesMessage struct {
	instances []*types.Instance
}

func (m *longRunningInstancesMessage) Message() string {
	var buffer bytes.Buffer
	buffer.WriteString("/code\n")
	for _, instance := range m.instances {
		buffer.WriteString(fmt.Sprintf("[%s] instance name: %s created: %s\n", instance.CloudType, instance.Name, instance.Created))
	}
	return buffer.String()
}

func (o LongRunning) Execute(clouds []types.CloudType) {
	var longRunningInstances = make([]*types.Instance, 0)
	for _, cloud := range clouds {
		provider, ok := ctx.CloudProviders[cloud]
		if !ok {
			panic("Cloud provider not supported")
		}
		instances, err := provider.GetRunningInstances()
		if err != nil {
			continue
		}
		longRunningInstances = append(longRunningInstances, getInstancesRunningLongerThan(instances, runningPeriod)...)
	}
	for _, instance := range longRunningInstances {
		log.Infof("[%s] Instance %s is running for more than: %s", instance.CloudType, instance.Name, time.Since(instance.Created))
	}
	if len(longRunningInstances) > 0 {
		message := longRunningInstancesMessage{instances: longRunningInstances}
		for name, dispatcher := range ctx.Dispatchers {
			if err := dispatcher.Send(&message); err != nil {
				log.Errorf("[%s] Failed to send message, err: %s", name, err.Error())
			}
		}
	}
}

func getInstancesRunningLongerThan(instances []*types.Instance, period time.Duration) []*types.Instance {
	filtered := make([]*types.Instance, 0)
	for _, instance := range instances {
		if instance.Created.Add(period).Before(time.Now()) {
			filtered = append(filtered, instance)
		}
	}
	return filtered
}
