package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cost(value float64) *float64 {
	return &value
}

func testReport(items ...types.CostedItem) *types.BudgetReport {
	return &types.BudgetReport{
		Period: 7 * day,
		Limit:  100,
		Owners: []types.OwnerSpend{
			{Owner: "mapped", Spend: 282.5, Items: items},
			{Owner: "unmapped", Spend: 120, Items: []types.CostedItem{{Item: &types.Disk{Name: "disk", Size: 128, Region: "region", CloudType: types.AWS}}}},
		},
	}
}

func testDispatcher(webhook string, dryRun bool) slackDispatcher {
	dispatcher := slackDispatcher{
		cfg:           &config.Config{IgnoreLabel: "ignore-label", DryRun: dryRun},
		ownerSlackIDs: map[string]string{"mapped": "U123"},
	}
	dispatcher.init(webhook)
	return dispatcher
}

func TestGenerateBudgetMessageMentionsOwnersAndListsTheirItems(t *testing.T) {
	created := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	report := testReport(
		types.CostedItem{Item: &types.Instance{Name: "vm", InstanceType: "large", Region: "region", CloudType: types.AZURE, Created: created}, Cost: cost(45.2)},
		types.CostedItem{Item: &types.Database{Name: "db", InstanceType: "small", Region: "region", CloudType: types.AZURE, Created: created}},
	)

	message := testDispatcher("", false).generateBudgetMessage(report)

	assert.Contains(t, message.Text, "$100.00 in the last 7 days")
	assert.Contains(t, message.Text, "`ignore-label`")
	assert.True(t, strings.HasSuffix(message.Text, "\ncc <@U123> unmapped"), message.Text)
	require.Len(t, message.Attachments, 2)
	assert.Equal(t, "<@U123>: *$282.50* in the last 7 days", message.Attachments[0].Pretext)
	assert.Equal(t, "• *[AZURE]* instance `vm` (large, region, running since 2026-10-01): $45.20\n"+
		"• *[AZURE]* database `db` (small, region, running since 2026-10-01): cost not reported per resource", message.Attachments[0].Text)
	assert.Equal(t, "unmapped: *$120.00* in the last 7 days", message.Attachments[1].Pretext)
	assert.Contains(t, message.Attachments[1].Text, "disk `disk` (128 GB, region")
}

func TestGenerateBudgetMessageCapsTheItemsPerOwner(t *testing.T) {
	items := make([]types.CostedItem, 0, maxItemsPerOwner+3)
	for index := range maxItemsPerOwner + 3 {
		items = append(items, types.CostedItem{Item: &types.Instance{Name: fmt.Sprintf("vm-%d", index)}})
	}

	message := testDispatcher("", false).generateBudgetMessage(testReport(items...))

	lines := strings.Split(message.Attachments[0].Text, "\n")
	assert.Len(t, lines, maxItemsPerOwner+1)
	assert.Equal(t, "…and 3 more", lines[maxItemsPerOwner])
}

func TestFormatPeriod(t *testing.T) {
	assert.Equal(t, "7 days", formatPeriod(7*day))
	assert.Equal(t, "36h0m0s", formatPeriod(36*time.Hour))
}

func TestLoadOwnerSlackIDsNormalizesOwners(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owners.yml")
	require.NoError(t, os.WriteFile(path, []byte("JDoe: U1\njane@example.com: U2\n"), 0600))

	ownerSlackIDs, err := loadOwnerSlackIDs(path)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"jdoe": "U1", "jane": "U2"}, ownerSlackIDs)
}

func TestLoadOwnerSlackIDsWithoutFileHasNoMentions(t *testing.T) {
	ownerSlackIDs, err := loadOwnerSlackIDs("")

	require.NoError(t, err)
	assert.Empty(t, ownerSlackIDs)
}

func TestLoadOwnerSlackIDsFailsOnMissingFile(t *testing.T) {
	_, err := loadOwnerSlackIDs(filepath.Join(t.TempDir(), "missing.yml"))

	assert.Error(t, err)
}

func TestSendBudgetReportPostsToTheWebhook(t *testing.T) {
	var posted slackMessage
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewDecoder(request.Body).Decode(&posted)
	}))
	defer server.Close()

	err := testDispatcher(server.URL, false).SendBudgetReport(testReport())

	require.NoError(t, err)
	assert.Len(t, posted.Attachments, 2)
}

func TestSendBudgetReportFailsWhenTheWebhookRejectsTheMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte("invalid_payload"))
	}))
	defer server.Close()

	err := testDispatcher(server.URL, false).SendBudgetReport(testReport())

	assert.ErrorContains(t, err, "invalid_payload")
}

func TestSendBudgetReportOnlyLogsOnDryRun(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()

	err := testDispatcher(server.URL, true).SendBudgetReport(testReport())

	require.NoError(t, err)
	assert.Zero(t, calls.Load())
}
