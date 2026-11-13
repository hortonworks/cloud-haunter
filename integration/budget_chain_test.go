package integration

import (
	"testing"
	"time"

	"github.com/hortonworks/cloud-haunter/action"
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/filter"
	"github.com/hortonworks/cloud-haunter/operation"
	"github.com/hortonworks/cloud-haunter/testutil"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturingBudgetDispatcher struct {
	reports []*types.BudgetReport
}

func (d *capturingBudgetDispatcher) GetName() string { return "capturing" }

func (d *capturingBudgetDispatcher) Send(types.OpType, []types.FilterType, []types.CloudItem) error {
	return nil
}

func (d *capturingBudgetDispatcher) SendBudgetReport(report *types.BudgetReport) error {
	d.reports = append(d.reports, report)
	return nil
}

// TestChainBudgetReminder runs the daily reminder job end to end:
// getBillableResources -f ignored,longrunning,nomatch -i -a budget-notification
func TestChainBudgetReminder(t *testing.T) {
	t.Parallel()
	old := time.Now().Add(-10 * 24 * time.Hour)
	recent := time.Now().Add(-time.Hour)
	ignore := types.Tags{config.DefaultIgnoreLabel: "true"}
	inheritsIgnore := map[string]string{types.ResourceGroupIgnoredMetadata: "true", types.ResourceGroupOwnerMetadata: "group-owner"}

	awsFake := &testutil.FakeCostProvider{
		FakeProvider: testutil.FakeProvider{
			Instances: []*types.Instance{
				{ID: "i-old", Name: "old-ignored", Owner: "spender", Tags: ignore, State: types.Running, Created: old, CloudType: types.AWS},
				{ID: "i-new", Name: "recent-ignored", Owner: "spender", Tags: ignore, State: types.Running, Created: recent, CloudType: types.AWS},
				{ID: "i-stopped", Name: "stopped-ignored", Owner: "spender", Tags: ignore, State: types.Stopped, Created: old, CloudType: types.AWS},
				{ID: "i-plain", Name: "not-ignored", Owner: "spender", State: types.Running, Created: old, CloudType: types.AWS},
				{ID: "i-infra", Name: "infra", Owner: "infra-owner", Tags: ignore, State: types.Running, Created: old, CloudType: types.AWS},
			},
			Disks: []*types.Disk{
				{ID: "vol-old", Name: "unused-ignored-disk", Owner: "spender", Tags: ignore, State: types.Unused, Created: old, CloudType: types.AWS},
			},
		},
		IgnoredCosts: &types.IgnoredCosts{
			ByOwner:    map[string]float64{"spender": 80, "infra-owner": 900, "thrifty": 5},
			ByResource: map[string]float64{"i-old": 42},
		},
	}
	azureFake := &testutil.FakeCostProvider{
		FakeProvider: testutil.FakeProvider{
			Databases: []*types.Database{
				{ID: "/db", Name: "group-ignored-db", State: types.Running, Created: old, Metadata: inheritsIgnore, CloudType: types.AZURE},
			},
		},
		IgnoredCosts: &types.IgnoredCosts{ByOwner: map[string]float64{"Spender@example.com": 30, "group-owner": 150}},
	}
	dispatcher := &capturingBudgetDispatcher{}
	cfg := &config.Config{
		IgnoreLabel:         config.DefaultIgnoreLabel,
		IgnoreLabelDisabled: true,
		LongRunningPeriod:   7 * 24 * time.Hour,
		BudgetPeriod:        7 * 24 * time.Hour,
		BudgetLimit:         100,
		FilterConfig:        loadInlineConfig(t, "excludeInstance:\n  aws:\n    owners:\n      - infra-owner\n"),
		CloudProviders:      testutil.CostRegistry(map[types.CloudType]types.CloudProvider{types.AWS: awsFake, types.AZURE: azureFake}),
		Dispatchers:         map[string]types.Dispatcher{"capturing": dispatcher},
	}
	filters := []types.Filter{filter.NewIgnored(cfg), filter.NewLongRunning(cfg), filter.NewNoMatch(cfg)}
	filterNames := []types.FilterType{types.IgnoredFilter, types.LongRunningFilter, types.NoMatchFilter}

	runChain(t, operation.NewBillableResources(cfg), filters, filterNames, action.NewBudgetNotification(cfg), []types.CloudType{types.AWS, types.AZURE})

	require.Len(t, dispatcher.reports, 1)
	owners := dispatcher.reports[0].Owners
	require.Len(t, owners, 2)
	assert.Equal(t, "group-owner", owners[0].Owner)
	assert.Equal(t, []string{"group-ignored-db"}, costedNames(owners[0].Items))
	assert.Equal(t, utils.NormalizeOwner("spender"), owners[1].Owner)
	assert.Equal(t, 110.0, owners[1].Spend)
	assert.Equal(t, []string{"old-ignored", "unused-ignored-disk"}, costedNames(owners[1].Items))
	assert.Equal(t, 42.0, *owners[1].Items[0].Cost)
}

func costedNames(items []types.CostedItem) []string {
	itemNames := make([]string, 0, len(items))
	for _, item := range items {
		itemNames = append(itemNames, item.Item.GetName())
	}
	return itemNames
}
