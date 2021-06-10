package hipchat

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"

	ctx "github.com/blentz/cloud-haunter/context"
	"github.com/blentz/cloud-haunter/types"
	"github.com/blentz/cloud-haunter/utils"
	log "github.com/sirupsen/logrus"
	"github.com/tbruyelle/hipchat-go/hipchat"
)

var dispatcher = hipchatDispatcher{}

type hipchatDispatcher struct {
	client *hipchat.Client
	room   string
}

func init() {
	token := os.Getenv("HIPCHAT_TOKEN")
	server := os.Getenv("HIPCHAT_SERVER")
	room := os.Getenv("HIPCHAT_ROOM")
	if len(token) == 0 || len(server) == 0 || len(room) == 0 {
		if len(token+server+room) != 0 {
			log.Warn("[HIPCHAT] HIPCHAT_TOKEN or HIPCHAT_SERVER or HIPCHAT_ROOM environment variables are missing")
		}
		return
	}
	if serverURL, err := url.Parse(server); err != nil {
		log.Errorf("[HIPCHAT] invalid url: %s, err: %s", server, err.Error())
	} else {
		log.Infof("[HIPCHAT] register hipchat to send notifications to server: %s and room: %s", serverURL.Host, room)
		dispatcher.init(token, room, serverURL)
		ctx.Dispatchers["HIPCHAT"] = dispatcher
	}
}

func (d *hipchatDispatcher) init(token, room string, serverURL *url.URL) {
	d.room = room
	d.client = hipchat.NewClient(token)
	d.client.BaseURL = serverURL
}

func (d hipchatDispatcher) GetName() string {
	return "HipChat"
}

func (d hipchatDispatcher) Send(op types.OpType, filters []types.FilterType, items []types.CloudItem) error {
	message := d.generateMessage(op, filters, items)
	log.Debugf("[HIPCHAT] Generated message is: %s", message)
	if ctx.DryRun {
		log.Info("[HIPCHAT] Skipping notification on dry run session")
	} else {
		return send(d.room, message, d.client.Room)
	}
	return nil
}

func (d *hipchatDispatcher) generateMessage(op types.OpType, filters []types.FilterType, items []types.CloudItem) string {
	var buffer bytes.Buffer
	buffer.WriteString("/code\n")
	buffer.WriteString(fmt.Sprintf("Operation: %s Filters: %s Accounts: %s\n", op, utils.GetFilterNames(filters), utils.GetCloudAccountNames()))
	for _, item := range items {
		displayTime := item.GetCreated().Format("2006-01-02 15:04:05")
		switch item.GetItem().(type) {
		case types.Instance:
			inst := item.GetItem().(types.Instance)
			msg := fmt.Sprintf("[%s] %s: %s type: %s created: %s owner: %s region: %s", item.GetCloudType(), item.GetType(), item.GetName(), inst.InstanceType, displayTime, item.GetOwner(), inst.Region)
			if len(inst.Metadata) > 0 {
				msg += fmt.Sprintf(" metadata: %s", inst.Metadata)
			}
			msg += "\n"
			buffer.WriteString(msg)
		case types.Database:
			db := item.GetItem().(types.Database)
			msg := fmt.Sprintf("[%s] %s: %s type: %s created: %s region: %s", item.GetCloudType(), item.GetType(), item.GetName(), db.InstanceType, displayTime, db.Region)
			if len(db.Metadata) > 0 {
				msg += fmt.Sprintf(" metadata: %s", db.Metadata)
			}
			msg += "\n"
			buffer.WriteString(msg)
		default:
			buffer.WriteString(fmt.Sprintf("[%s] %s: %s created: %s owner: %s\n", item.GetCloudType(), item.GetType(), item.GetName(), displayTime, item.GetOwner()))
		}
	}
	return buffer.String()
}

type notificationClient interface {
	Notification(string, *hipchat.NotificationRequest) (*http.Response, error)
}

func send(room, message string, client notificationClient) error {
	_, err := client.Notification(room, &hipchat.NotificationRequest{
		Message:       message,
		Color:         "green",
		MessageFormat: "text",
	})
	return err
}
