package slack

import (
	"bytes"
	"fmt"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
)

const (
	// RedColor Hex code of color red
	RedColor = "#FF0000"

	// GreenColor Hex code of color green
	GreenColor = "#008000"
)

type slackDispatcher struct {
	webhook    string
	httpClient *http.Client
}

type slackMessage struct {
	Text        string       `json:"text"`
	Attachments []attachment `json:"attachments"`
}

type attachment struct {
	Color      string   `json:"color"`
	Pretext    string   `json:"pretext"`
	Text       string   `json:"text"`
	MarkdownIn []string `json:"mrkdwn_in"`
	Fields     []field  `json:"fields"`
}

type field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func init() {
	webhook := os.Getenv("SLACK_WEBHOOK_URL")
	if len(webhook) > 0 {
		slack := slackDispatcher{}
		slack.init(webhook)
		ctx.Dispatchers["SLACK"] = slack
		log.Infof("[SLACK] register slack to send notifications")
	}
}

func (d *slackDispatcher) init(webhook string) {
	d.webhook = webhook
	d.httpClient = &http.Client{}
}

func (d slackDispatcher) GetName() string {
	return "Slack"
}

func (d slackDispatcher) Send(op types.OpType, filters []types.FilterType, items []types.CloudItem) error {
	message := d.generateMessage(op, filters, items)
	if ctx.DryRun {
		json, err := utils.CovertJsonToString(message)
		if err != nil {
			return err
		}
		log.Infof("[SLACK] Skipping notification on dry run session, generated message: %s", *json)
	} else {
		return d.send(message)
	}
	return nil
}

func (d slackDispatcher) send(message slackMessage) error {
	json, err := utils.CovertJsonToString(message)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", d.webhook, bytes.NewBuffer([]byte(*json)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if _, err := d.httpClient.Do(req); err != nil {
		return err
	}
	return nil
}

func (d slackDispatcher) generateMessage(op types.OpType, filters []types.FilterType, items []types.CloudItem) slackMessage {
	message := slackMessage{}
	message.Text = fmt.Sprintf("*Operation*: %s *Filters*: %s *Accounts*: %s\n", op, utils.GetFilterNames(filters), utils.GetCloudAccountNames())

	itemsPerOwner := map[string][]types.CloudItem{}
	color := GreenColor
	for _, item := range items {
		owner := item.GetOwner()
		if len(owner) == 0 || owner == "???" {
			itemsPerOwner["unknown"] = append(itemsPerOwner["unknown"], item)
			color = RedColor
		} else {
			itemsPerOwner[owner] = append(itemsPerOwner[owner], item)
		}
	}

	detailsAttach := attachment{
		MarkdownIn: []string{"text", "pretext"},
		Color:      color,
	}

	summaryAttach := attachment{
		MarkdownIn: []string{"text", "pretext"},
		Color:      color,
	}

	var buffer bytes.Buffer

	for owner, items := range itemsPerOwner {
		buffer.WriteString(fmt.Sprintf("\n*Owner*: %s *items*: %d\n", owner, len(items)))
		summaryAttach.Fields = append(summaryAttach.Fields, field{
			Title: fmt.Sprintf("*%s*: %d", owner, len(items)),
			Short: true,
		})

		for _, item := range items {
			displayTime := item.GetCreated().Format("2006-01-02 15:04:05")
			switch item.GetItem().(type) {
			case types.Instance:
				inst := item.GetItem().(types.Instance)
				msg := fmt.Sprintf("*[%s]* *%s*: %s *type*: %s *created*: %s *region*: %s", item.GetCloudType(), item.GetType(), item.GetName(), inst.InstanceType, displayTime, inst.Region)
				if len(inst.Metadata) > 0 {
					msg += fmt.Sprintf(" metadata: %s", inst.Metadata)
				}
				msg += "\n"
				buffer.WriteString(msg)
			case types.Database:
				db := item.GetItem().(types.Database)
				msg := fmt.Sprintf("*[%s]* *%s*: %s *type*: %s *created*: %s *region*: %s", item.GetCloudType(), item.GetType(), item.GetName(), db.InstanceType, displayTime, db.Region)
				if len(db.Metadata) > 0 {
					msg += fmt.Sprintf(" metadata: %s", db.Metadata)
				}
				msg += "\n"
				buffer.WriteString(msg)
			case types.Cluster:
				clust := item.GetItem().(types.Cluster)
				msg := fmt.Sprintf("*[%s]* *%s*: %s *state*: %s *created*: %s *region*: %s", item.GetCloudType(), item.GetType(), item.GetName(), clust.State, displayTime, clust.Region)
				msg += "\n"
				buffer.WriteString(msg)
			default:
				buffer.WriteString(fmt.Sprintf("*[%s]* *%s*: %s\n", item.GetCloudType(), item.GetType(), item.GetName()))
			}
		}

		buffer.WriteString("\n")
	}

	detailsAttach.Text = buffer.String()
	message.Attachments = []attachment{summaryAttach, detailsAttach}

	return message
}
