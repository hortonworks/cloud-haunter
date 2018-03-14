package hipchat

import (
	"bytes"
	"fmt"
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/tbruyelle/hipchat-go/hipchat"
	"net/url"
	"os"
)

var (
	client *hipchat.Client
	room   string
)

func init() {
	token := os.Getenv("HIPCHAT_TOKEN")
	server := os.Getenv("HIPCHAT_SERVER")
	room = os.Getenv("HIPCHAT_ROOM")
	if len(token) > 0 && len(server) > 0 && len(room) > 0 {
		log.Infof("[HIPCHAT] register hipchat to send notifications to server: %s and room: %s", server, room)
		ctx.Dispatchers["hipchat"] = new(Hipchat)
		client = hipchat.NewClient(token)
		if serverUrl, err := url.Parse(server); err != nil {
			log.Errorf("[HIPCHAT] invalid url: %s, err: %s", server, err.Error())
		} else {
			client.BaseURL = serverUrl
		}
	}
}

type Hipchat struct {
}

func (h *Hipchat) Send(notification types.Notification) error {
	if ctx.DRY_RUN {
		log.Infof("[HIPCHAT] dry-run set, no notification sent")
	} else {
		var buffer bytes.Buffer
		buffer.WriteString("/code\n")
		for _, instance := range notification.Instances {
			buffer.WriteString(fmt.Sprintf("[%s] instance name: %s created: %s\n", instance.CloudType, instance.Name, instance.Created))
		}
		message := buffer.String()
		_, err := client.Room.Notification(room, &hipchat.NotificationRequest{
			Message:       message,
			Color:         "green",
			MessageFormat: "text",
		})
		if err != nil {
			return err
		}
	}
	return nil
}
