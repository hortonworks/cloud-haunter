package hipchat

import (
	"encoding/json"
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

func (h *Hipchat) Send(message types.Message) error {
	if ctx.DryRun {
		log.Infof("[HIPCHAT] dry-run set, no notification sent")
		out, _ := json.Marshal(message)
		log.Infof("[HIPCHAT] %s", string(out))
	} else {
		_, err := client.Room.Notification(room, &hipchat.NotificationRequest{
			Message:       message.Message(),
			Color:         "green",
			MessageFormat: "text",
		})
		if err != nil {
			return err
		}
	}
	return nil
}
