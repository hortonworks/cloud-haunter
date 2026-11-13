package action

import (
	"errors"
	"testing"
	"time"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/testutil"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBudgetPeriod = 7 * 24 * time.Hour

var testNow = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

type budgetDispatcher struct {
	mockDispatcher
	reports []*types.BudgetReport
	err     error
}

func (d *budgetDispatcher) SendBudgetReport(report *types.BudgetReport) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.reports = append(d.reports, report)
	return d.err
}

func budgetCfg(providers map[types.CloudType]types.CloudProvider, dispatchers map[string]types.Dispatcher) *config.Config {
	return &config.Config{
		BudgetPeriod:   testBudgetPeriod,
		BudgetLimit:    100,
		CloudProviders: testutil.CostRegistry(providers),
		Dispatchers:    dispatchers,
	}
}

func costs(byOwner, byResource map[string]float64) *testutil.FakeCostProvider {
	return &testutil.FakeCostProvider{IgnoredCosts: &types.IgnoredCosts{ByOwner: byOwner, ByResource: byResource}}
}

func runBudgetNotification(t *testing.T, cfg *config.Config, items ...types.CloudItem) error {
	t.Helper()
	return budgetNotificationAction{cfg: cfg, now: func() time.Time { return testNow }}.Execute(types.BillableResources, nil, items)
}

func reportedOwners(report *types.BudgetReport) map[string]float64 {
	spendByOwner := map[string]float64{}
	for _, owner := range report.Owners {
		spendByOwner[owner.Owner] = owner.Spend
	}
	return spendByOwner
}

func TestBudgetNotificationRemindsOwnersOverTheLimitWithLongRunningItems(t *testing.T) {
	t.Parallel()
	dispatcher := &budgetDispatcher{}
	aws := costs(map[string]float64{"over": 60, "under": 99, "no-items": 500}, nil)
	azure := costs(map[string]float64{"Over@example.com": 50}, nil)
	cfg := budgetCfg(map[types.CloudType]types.CloudProvider{types.AWS: aws, types.AZURE: azure},
		map[string]types.Dispatcher{"budget": dispatcher})

	err := runBudgetNotification(t, cfg,
		&types.Instance{ID: "i-1", Name: "over-instance", Owner: "over", CloudType: types.AWS},
		&types.Database{ID: "db-1", Name: "under-database", Owner: "under", CloudType: types.AWS})

	require.NoError(t, err)
	require.Len(t, dispatcher.reports, 1)
	assert.Equal(t, map[string]float64{"over": 110}, reportedOwners(dispatcher.reports[0]))
	assert.Equal(t, testBudgetPeriod, dispatcher.reports[0].Period)
	assert.Equal(t, 100.0, dispatcher.reports[0].Limit)
	assert.Equal(t, []testutil.CostWindow{{Since: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), Until: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)}}, aws.CostWindows)
	assert.Equal(t, aws.CostWindows, azure.CostWindows)
}

func TestBudgetWindowRoundsThePeriodUpToWholeUTCDays(t *testing.T) {
	t.Parallel()
	action := budgetNotificationAction{cfg: &config.Config{BudgetPeriod: 36 * time.Hour}, now: func() time.Time { return testNow }}

	since, until := action.budgetWindow()

	assert.Equal(t, time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC), since)
	assert.Equal(t, time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), until)
}

func TestBudgetNotificationAttachesAndSortsItemCosts(t *testing.T) {
	t.Parallel()
	dispatcher := &budgetDispatcher{}
	provider := costs(map[string]float64{"owner": 200}, map[string]float64{"/subscriptions/s/vm-cheap": 5, "/subscriptions/s/vm-pricey": 90})
	cfg := budgetCfg(map[types.CloudType]types.CloudProvider{types.AZURE: provider}, map[string]types.Dispatcher{"budget": dispatcher})

	err := runBudgetNotification(t, cfg,
		&types.Disk{ID: "disk-without-cost", Name: "disk", Owner: "owner"},
		&types.Instance{ID: "/subscriptions/S/VM-CHEAP", Name: "cheap", Owner: "owner"},
		&types.Instance{ID: "/subscriptions/s/vm-pricey", Name: "pricey", Owner: "owner"})

	require.NoError(t, err)
	items := dispatcher.reports[0].Owners[0].Items
	require.Len(t, items, 3)
	assert.Equal(t, "pricey", items[0].Item.GetName())
	assert.Equal(t, 90.0, *items[0].Cost)
	assert.Equal(t, "cheap", items[1].Item.GetName())
	assert.Equal(t, 5.0, *items[1].Cost)
	assert.Equal(t, "disk", items[2].Item.GetName())
	assert.Nil(t, items[2].Cost)
}

func TestBudgetNotificationFallsBackToTheResourceGroupOwner(t *testing.T) {
	t.Parallel()
	dispatcher := &budgetDispatcher{}
	provider := costs(map[string]float64{"group-owner": 150}, nil)
	cfg := budgetCfg(map[types.CloudType]types.CloudProvider{types.AZURE: provider}, map[string]types.Dispatcher{"budget": dispatcher})

	groupOwner := map[string]string{types.ResourceGroupOwnerMetadata: "Group-Owner"}
	err := runBudgetNotification(t, cfg,
		&types.Instance{Name: "child-instance", Metadata: groupOwner},
		&types.Database{Name: "child-database", Metadata: groupOwner},
		&types.Disk{Name: "child-disk", Metadata: groupOwner},
		&types.Instance{Name: "orphan"})

	require.NoError(t, err)
	require.Len(t, dispatcher.reports, 1)
	owners := dispatcher.reports[0].Owners
	require.Len(t, owners, 1)
	assert.Equal(t, "group-owner", owners[0].Owner)
	assert.Len(t, owners[0].Items, 3)
}

func TestBudgetNotificationOrdersOwnersBySpend(t *testing.T) {
	t.Parallel()
	dispatcher := &budgetDispatcher{}
	provider := costs(map[string]float64{"second": 150, "first": 300, "third": 101}, nil)
	cfg := budgetCfg(map[types.CloudType]types.CloudProvider{types.AWS: provider}, map[string]types.Dispatcher{"budget": dispatcher})

	err := runBudgetNotification(t, cfg,
		&types.Instance{Name: "a", Owner: "third"},
		&types.Instance{Name: "b", Owner: "first"},
		&types.Instance{Name: "c", Owner: "second"})

	require.NoError(t, err)
	var order []string
	for _, owner := range dispatcher.reports[0].Owners {
		order = append(order, owner.Owner)
	}
	assert.Equal(t, []string{"first", "second", "third"}, order)
}

func TestBudgetNotificationSkipsCloudsThatDoNotReportCosts(t *testing.T) {
	t.Parallel()
	dispatcher := &budgetDispatcher{}
	cfg := budgetCfg(map[types.CloudType]types.CloudProvider{
		types.AWS:   &testutil.FakeProvider{},
		types.AZURE: costs(map[string]float64{"owner": 101}, nil),
	}, map[string]types.Dispatcher{"budget": dispatcher})

	err := runBudgetNotification(t, cfg, &types.Instance{Name: "a", Owner: "owner"})

	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"owner": 101}, reportedOwners(dispatcher.reports[0]))
}

func TestBudgetNotificationRemindsTheOtherCloudsOwnersAndFailsWhenACloudCannotReportCosts(t *testing.T) {
	t.Parallel()
	dispatcher := &budgetDispatcher{}
	failing := &testutil.FakeCostProvider{CostErr: errors.New("billing unavailable")}
	cfg := budgetCfg(map[types.CloudType]types.CloudProvider{
		types.GCP:   failing,
		types.AZURE: costs(map[string]float64{"owner": 101}, nil),
	}, map[string]types.Dispatcher{"budget": dispatcher})

	err := runBudgetNotification(t, cfg, &types.Instance{Name: "a", Owner: "owner"})

	assert.EqualError(t, err, "billing unavailable")
	assert.Equal(t, map[string]float64{"owner": 101}, reportedOwners(dispatcher.reports[0]))
	assert.Len(t, failing.CostWindows, 1)
}

func TestBudgetNotificationFailsWhenACloudCannotReportCostsAndNobodyIsOverTheLimit(t *testing.T) {
	t.Parallel()
	dispatcher := &budgetDispatcher{}
	cfg := budgetCfg(map[types.CloudType]types.CloudProvider{types.GCP: &testutil.FakeCostProvider{CostErr: errors.New("billing unavailable")}},
		map[string]types.Dispatcher{"budget": dispatcher})

	err := runBudgetNotification(t, cfg, &types.Instance{Name: "a", Owner: "owner"})

	assert.EqualError(t, err, "billing unavailable")
	assert.Empty(t, dispatcher.reports)
}

func TestAwsItemsWithEmptyIgnoreLabel(t *testing.T) {
	t.Parallel()
	action := budgetNotificationAction{cfg: &config.Config{IgnoreLabel: "ignore"}}
	emptyOnAws := &types.Instance{Name: "empty-on-aws", CloudType: types.AWS, Tags: types.Tags{"ignore": ""}}

	uncounted := action.awsItemsWithEmptyIgnoreLabel([]types.CloudItem{
		emptyOnAws,
		&types.Instance{Name: "valued-on-aws", CloudType: types.AWS, Tags: types.Tags{"ignore": "true"}},
		&types.Instance{Name: "empty-on-azure", CloudType: types.AZURE, Tags: types.Tags{"ignore": ""}},
		&types.Database{Name: "unlabelled-on-aws", CloudType: types.AWS, Tags: types.Tags{}},
	})

	assert.Equal(t, []types.CloudItem{emptyOnAws}, uncounted)
}

func TestBudgetNotificationSendsNothingWhenNobodyIsOverTheLimit(t *testing.T) {
	t.Parallel()
	dispatcher := &budgetDispatcher{}
	cfg := budgetCfg(map[types.CloudType]types.CloudProvider{types.AWS: costs(map[string]float64{"owner": 100}, nil)},
		map[string]types.Dispatcher{"budget": dispatcher})

	err := runBudgetNotification(t, cfg, &types.Instance{Name: "a", Owner: "owner"})

	require.NoError(t, err)
	assert.Empty(t, dispatcher.reports)
}

func TestBudgetNotificationReportsDispatchFailuresAndSkipsPlainDispatchers(t *testing.T) {
	t.Parallel()
	plain := &mockDispatcher{}
	failing := &budgetDispatcher{err: errors.New("webhook rejected")}
	cfg := budgetCfg(map[types.CloudType]types.CloudProvider{types.AWS: costs(map[string]float64{"owner": 101}, nil)},
		map[string]types.Dispatcher{"plain": plain, "failing": failing})

	err := runBudgetNotification(t, cfg, &types.Instance{Name: "a", Owner: "owner"})

	assert.EqualError(t, err, "webhook rejected")
	assert.Len(t, failing.reports, 1)
	assert.Equal(t, 0, plain.calls)
}

func excludedAzureOwners(owners ...string) types.IFilterConfig {
	return types.FilterConfigV2{Filters: []types.FilterConfigV2Filter{{
		Types:      []types.FilterEntityType{types.ExcludeInstance},
		CloudTypes: []types.CloudType{"azure"},
		Properties: []types.FilterConfigProperty{types.Owner},
		Values:     owners,
	}}}
}

func TestBudgetNotificationSkipsExcludedOwnersInheritedFromTheResourceGroup(t *testing.T) {
	t.Parallel()
	dispatcher := &budgetDispatcher{}
	cfg := budgetCfg(map[types.CloudType]types.CloudProvider{types.AZURE: costs(map[string]float64{"infra-team": 500, "spender": 150}, nil)},
		map[string]types.Dispatcher{"budget": dispatcher})
	cfg.FilterConfig = excludedAzureOwners("infra")

	err := runBudgetNotification(t, cfg,
		&types.Database{Name: "infra-database", CloudType: types.AZURE, Metadata: map[string]string{types.ResourceGroupOwnerMetadata: "infra-team"}},
		&types.Instance{Name: "spender-instance", Owner: "spender", CloudType: types.AZURE})

	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"spender": 150}, reportedOwners(dispatcher.reports[0]))
}

func TestBudgetNotificationMatchesExcludedOwnersExactlyWhenAsked(t *testing.T) {
	t.Parallel()
	dispatcher := &budgetDispatcher{}
	cfg := budgetCfg(map[types.CloudType]types.CloudProvider{types.AZURE: costs(map[string]float64{"infra-team": 500}, nil)},
		map[string]types.Dispatcher{"budget": dispatcher})
	cfg.FilterConfig = excludedAzureOwners("infra")
	cfg.ExactMatchOwner = true

	err := runBudgetNotification(t, cfg,
		&types.Database{Name: "infra-database", CloudType: types.AZURE, Metadata: map[string]string{types.ResourceGroupOwnerMetadata: "infra-team"}})

	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"infra-team": 500}, reportedOwners(dispatcher.reports[0]))
}
