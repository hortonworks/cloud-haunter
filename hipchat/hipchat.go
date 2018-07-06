package hipchat

import (
	"bytes"
	"fmt"
	"net/url"
	"os"

	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/tbruyelle/hipchat-go/hipchat"
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
		client = hipchat.NewClient(token)
		if serverUrl, err := url.Parse(server); err != nil {
			log.Errorf("[HIPCHAT] invalid url: %s, err: %s", server, err.Error())
		} else {
			client.BaseURL = serverUrl
			ctx.Dispatchers["HIPCHAT"] = new(Hipchat)
		}
	}
}

type Hipchat struct {
}

func (h *Hipchat) GetName() string {
	return "HipChat"
}

func (h *Hipchat) Send(op *types.OpType, items []types.CloudItem) error {
	message := h.generateMessage(op, items)
	if ctx.DryRun {
		log.Info("[HIPCHAT] Skipping notification on dry run session")
	} else {
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

func (h *Hipchat) generateMessage(op *types.OpType, items []types.CloudItem) string {
	var buffer bytes.Buffer
	buffer.WriteString("/code\n")
	for _, item := range items {
		switch item.GetItem().(type) {
		case types.Instance:
			var inst types.Instance = item.GetItem().(types.Instance)
			buffer.WriteString(fmt.Sprintf("[%s] instance name: %s created: %s owner: %s region: %s\n", item.GetCloudType(), item.GetName(), item.GetCreated(), item.GetOwner(), inst.Region))
		default:
			buffer.WriteString(fmt.Sprintf("[%s] %s: %s created: %s owner: %s\n", item.GetCloudType(), item.GetType(), item.GetName(), item.GetCreated(), item.GetOwner()))
		}
	}
	if ctx.DryRun {
		log.Debugf("[HIPCHAT] Generated message is: %s", buffer.String())
	}
	return buffer.String()
}
