package slack

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var _ types.BudgetDispatcher = slackDispatcher{}

const (
	ownerSlackIDsFileEnv = "SLACK_OWNER_IDS_FILE"
	maxItemsPerOwner     = 10
	day                  = 24 * time.Hour
)

// loadOwnerSlackIDs reads a YAML map of owner label values to Slack member IDs, keyed by normalized owner
func loadOwnerSlackIDs(path string) (map[string]string, error) {
	ownerSlackIDs := map[string]string{}
	if path == "" {
		return ownerSlackIDs, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rawOwnerSlackIDs map[string]string
	if err := yaml.Unmarshal(content, &rawOwnerSlackIDs); err != nil {
		return nil, err
	}
	for owner, slackID := range rawOwnerSlackIDs {
		ownerSlackIDs[utils.NormalizeOwner(owner)] = slackID
	}
	return ownerSlackIDs, nil
}

// SendBudgetReport posts one reminder listing every owner of the report with their long-running ignored items
func (d slackDispatcher) SendBudgetReport(report *types.BudgetReport) error {
	message := d.generateBudgetMessage(report)
	if d.cfg.DryRun {
		json, err := utils.CovertJsonToString(message)
		if err != nil {
			return err
		}
		log.Infof("[SLACK] Skipping budget reminder on dry run session, generated message: %s", *json)
		return nil
	}
	return d.send(message)
}

func (d slackDispatcher) generateBudgetMessage(report *types.BudgetReport) slackMessage {
	message := slackMessage{Text: fmt.Sprintf(
		"*Reminder*: these owners spent more than $%.2f in the last %s on resources labelled `%s`, and some of those resources are still running. "+
			"Stop or delete them, or remove the label if they should be cleaned up.\ncc %s",
		report.Limit, formatPeriod(report.Period), d.cfg.IgnoreLabel, d.mentions(report.Owners))}
	for _, owner := range report.Owners {
		message.Attachments = append(message.Attachments, attachment{
			Color:      RedColor,
			Pretext:    fmt.Sprintf("%s: *$%.2f* in the last %s", d.mention(owner.Owner), owner.Spend, formatPeriod(report.Period)),
			Text:       itemLines(owner.Items),
			MarkdownIn: []string{"text", "pretext"},
		})
	}
	return message
}

// mentions are repeated in the top-level text because Slack does not notify about mentions inside attachments
func (d slackDispatcher) mentions(owners []types.OwnerSpend) string {
	mentions := make([]string, 0, len(owners))
	for _, owner := range owners {
		mentions = append(mentions, d.mention(owner.Owner))
	}
	return strings.Join(mentions, " ")
}

func (d slackDispatcher) mention(owner string) string {
	if slackID, known := d.ownerSlackIDs[owner]; known {
		return fmt.Sprintf("<@%s>", slackID)
	}
	return owner
}

func itemLines(items []types.CostedItem) string {
	lines := make([]string, 0, maxItemsPerOwner+1)
	for _, costedItem := range items[:min(len(items), maxItemsPerOwner)] {
		lines = append(lines, itemLine(costedItem))
	}
	if hidden := len(items) - maxItemsPerOwner; hidden > 0 {
		lines = append(lines, fmt.Sprintf("…and %d more", hidden))
	}
	return strings.Join(lines, "\n")
}

func itemLine(costedItem types.CostedItem) string {
	item := costedItem.Item
	return fmt.Sprintf("• *[%s]* %s `%s` (%s, running since %s): %s",
		item.GetCloudType(), item.GetType(), item.GetName(), itemDetails(item), item.GetCreated().Format("2006-01-02"), formatCost(costedItem.Cost))
}

func itemDetails(item types.CloudItem) string {
	switch typed := item.GetItem().(type) {
	case types.Instance:
		return typed.InstanceType + ", " + typed.Region
	case types.Database:
		return typed.InstanceType + ", " + typed.Region
	case types.Disk:
		return fmt.Sprintf("%d GB, %s", typed.Size, typed.Region)
	default:
		return string(item.GetCloudType())
	}
}

func formatCost(cost *float64) string {
	if cost == nil {
		return "cost not reported per resource"
	}
	return fmt.Sprintf("$%.2f", *cost)
}

func formatPeriod(period time.Duration) string {
	if period%day == 0 {
		return fmt.Sprintf("%d days", period/day)
	}
	return period.String()
}
