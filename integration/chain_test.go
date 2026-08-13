package integration

import (
	"os"
	"path/filepath"
	"testing"

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

// capturingAction is a types.Action that records the items handed to it, so a
// chain test can assert exactly which items survived the operation and filters.
type capturingAction struct {
	items []types.CloudItem
}

func (a *capturingAction) Execute(_ types.OpType, _ []types.FilterType, items []types.CloudItem) error {
	a.items = items
	return nil
}

// runChain executes the operation -> filters -> action sequence the same way
// main() does, failing the test if any stage errors.
func runChain(t *testing.T, op types.Operation, filters []types.Filter, filterNames []types.FilterType, act types.Action, clouds []types.CloudType) {
	t.Helper()
	items, err := op.Execute(clouds)
	require.NoError(t, err)
	for _, f := range filters {
		items, err = f.Execute(items)
		require.NoError(t, err)
	}
	require.NoError(t, act.Execute(types.Instances, filterNames, items))
}

// names extracts the CloudItem names for order-insensitive assertions (the
// operation collects clouds concurrently, so output order is not deterministic).
func names(items []types.CloudItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.GetName()
	}
	return out
}

// loadInlineConfig writes an inline V1 filter-config YAML to a temp file and
// loads it through the real parser, so cases exercise the actual config code
// path while keeping the fixture co-located with the assertion.
func loadInlineConfig(t *testing.T, yaml string) types.IFilterConfig {
	t.Helper()
	path := filepath.Join(t.TempDir(), "filter-config.yml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0600))
	cfg, err := utils.LoadFilterConfig(path)
	require.NoError(t, err)
	return cfg
}

func inst(name, owner string, cloud types.CloudType, state types.State) *types.Instance {
	return &types.Instance{Name: name, Owner: owner, CloudType: cloud, State: state}
}

// TestChainOperationFilterAction drives the instances operation through one or
// more filters into a capturing action, asserting which items reach the action.
func TestChainOperationFilterAction(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		clouds     []types.CloudType
		seed       map[types.CloudType][]*types.Instance
		getErr     bool
		filterYAML string
		filters    func(cfg *config.Config) ([]types.Filter, []types.FilterType)
		wantNames  []string
	}{
		{
			name:   "running keeps only running instances",
			clouds: []types.CloudType{types.AWS},
			seed: map[types.CloudType][]*types.Instance{types.AWS: {
				inst("run-1", "alice", types.AWS, types.Running),
				inst("stop-1", "alice", types.AWS, types.Stopped),
				inst("run-2", "bob", types.AWS, types.Running),
			}},
			filters: func(cfg *config.Config) ([]types.Filter, []types.FilterType) {
				return []types.Filter{filter.NewRunning(cfg)}, []types.FilterType{types.RunningFilter}
			},
			wantNames: []string{"run-1", "run-2"},
		},
		{
			name:   "ownerless keeps only unowned instances",
			clouds: []types.CloudType{types.AWS},
			seed: map[types.CloudType][]*types.Instance{types.AWS: {
				inst("owned", "alice", types.AWS, types.Running),
				inst("orphan", "", types.AWS, types.Running),
			}},
			filters: func(cfg *config.Config) ([]types.Filter, []types.FilterType) {
				return []types.Filter{filter.NewOwnerless(cfg)}, []types.FilterType{types.OwnerlessFilter}
			},
			wantNames: []string{"orphan"},
		},
		{
			name:   "running then ownerless chained in order",
			clouds: []types.CloudType{types.AWS},
			seed: map[types.CloudType][]*types.Instance{types.AWS: {
				inst("run-owned", "alice", types.AWS, types.Running),
				inst("run-free", "", types.AWS, types.Running),
				inst("stop-free", "", types.AWS, types.Stopped),
			}},
			filters: func(cfg *config.Config) ([]types.Filter, []types.FilterType) {
				return []types.Filter{filter.NewRunning(cfg), filter.NewOwnerless(cfg)},
					[]types.FilterType{types.RunningFilter, types.OwnerlessFilter}
			},
			wantNames: []string{"run-free"},
		},
		{
			name:   "match filter applies inline include config",
			clouds: []types.CloudType{types.AWS},
			seed: map[types.CloudType][]*types.Instance{types.AWS: {
				inst("keep-1", "alice", types.AWS, types.Running),
				inst("drop-1", "alice", types.AWS, types.Running),
			}},
			filterYAML: "" +
				"includeInstance:\n" +
				"  aws:\n" +
				"    names:\n" +
				"      - keep-\n",
			filters: func(cfg *config.Config) ([]types.Filter, []types.FilterType) {
				return []types.Filter{filter.NewMatch(cfg)}, []types.FilterType{types.MatchFilter}
			},
			wantNames: []string{"keep-1"},
		},
		{
			name:   "provider read error yields no items and no failure",
			clouds: []types.CloudType{types.AWS},
			seed:   map[types.CloudType][]*types.Instance{types.AWS: {inst("run-1", "alice", types.AWS, types.Running)}},
			getErr: true,
			filters: func(cfg *config.Config) ([]types.Filter, []types.FilterType) {
				return nil, nil
			},
			wantNames: []string{},
		},
		{
			name:   "running filter across multiple clouds",
			clouds: []types.CloudType{types.AWS, types.AZURE},
			seed: map[types.CloudType][]*types.Instance{
				types.AWS:   {inst("aws-run", "alice", types.AWS, types.Running)},
				types.AZURE: {inst("az-run", "bob", types.AZURE, types.Running), inst("az-stop", "bob", types.AZURE, types.Stopped)},
			},
			filters: func(cfg *config.Config) ([]types.Filter, []types.FilterType) {
				return []types.Filter{filter.NewRunning(cfg)}, []types.FilterType{types.RunningFilter}
			},
			wantNames: []string{"aws-run", "az-run"},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			fakes := map[types.CloudType]*testutil.FakeProvider{}
			for cloud, instances := range c.seed {
				f := &testutil.FakeProvider{Instances: instances}
				if c.getErr {
					f.GetErr = assert.AnError
				}
				fakes[cloud] = f
			}

			cfg := &config.Config{CloudProviders: testutil.Registry(fakes)}
			if c.filterYAML != "" {
				cfg.FilterConfig = loadInlineConfig(t, c.filterYAML)
			}

			filters, filterNames := c.filters(cfg)
			capture := &capturingAction{}
			runChain(t, operation.NewInstances(cfg), filters, filterNames, capture, c.clouds)

			assert.ElementsMatch(t, c.wantNames, names(capture.items))
		})
	}
}

// TestChainStopActionRoutesFilteredItems verifies the last stage of the chain:
// the stop action routes exactly the filtered instances back to the matching
// provider, per cloud.
func TestChainStopActionRoutesFilteredItems(t *testing.T) {
	t.Parallel()

	awsFake := &testutil.FakeProvider{Instances: []*types.Instance{
		inst("aws-run", "alice", types.AWS, types.Running),
		inst("aws-stop", "alice", types.AWS, types.Stopped),
	}}
	azFake := &testutil.FakeProvider{Instances: []*types.Instance{
		inst("az-run", "bob", types.AZURE, types.Running),
	}}
	cfg := &config.Config{CloudProviders: testutil.Registry(map[types.CloudType]*testutil.FakeProvider{
		types.AWS:   awsFake,
		types.AZURE: azFake,
	})}

	runChain(t,
		operation.NewInstances(cfg),
		[]types.Filter{filter.NewRunning(cfg)},
		[]types.FilterType{types.RunningFilter},
		action.NewStop(cfg),
		[]types.CloudType{types.AWS, types.AZURE},
	)

	// Only the running instances should have been stopped, on their own cloud.
	require.Len(t, awsFake.StopInstancesCalls, 1)
	assert.Equal(t, []string{"aws-run"}, names(instancesToItems(awsFake.StopInstancesCalls[0].Get(types.AWS))))

	require.Len(t, azFake.StopInstancesCalls, 1)
	assert.Equal(t, []string{"az-run"}, names(instancesToItems(azFake.StopInstancesCalls[0].Get(types.AZURE))))
}

// instancesToItems adapts a []*types.Instance to []types.CloudItem so the shared
// names helper can be reused for assertions on recorded provider calls.
func instancesToItems(instances []*types.Instance) []types.CloudItem {
	items := make([]types.CloudItem, len(instances))
	for i, in := range instances {
		items[i] = in
	}
	return items
}
