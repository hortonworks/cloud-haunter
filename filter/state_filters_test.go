package filter

import (
	"testing"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
	"github.com/stretchr/testify/assert"
)

func filterNames(items []types.CloudItem) []string {
	names := []string{}
	for _, item := range items {
		names = append(names, item.GetName())
	}
	return names
}

// TestStateFilters covers the state-based filters: each keeps only the item
// types/states it targets and drops the rest, exercising every case branch.
func TestStateFilters(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		filter   types.Filter
		items    []types.CloudItem
		wantKept []string
	}{
		{
			name:   "running keeps running instances/stacks/databases and unused disks",
			filter: NewRunning(&config.Config{}),
			items: []types.CloudItem{
				&types.Instance{Name: "running-inst", State: types.Running},
				&types.Instance{Name: "stopped-inst", State: types.Stopped},
				&types.Stack{Name: "running-stack", State: types.Running},
				&types.Stack{Name: "stopped-stack", State: types.Stopped},
				&types.Database{Name: "running-db", State: types.Running},
				&types.Database{Name: "stopped-db", State: types.Stopped},
				&types.Disk{Name: "unused-disk", State: types.Unused},
				&types.Disk{Name: "inuse-disk", State: types.InUse},
			},
			wantKept: []string{"running-inst", "running-stack", "running-db", "unused-disk"},
		},
		{
			name:   "stopped keeps stopped instances",
			filter: NewStopped(&config.Config{}),
			items: []types.CloudItem{
				&types.Instance{Name: "stopped", State: types.Stopped},
				&types.Instance{Name: "running", State: types.Running},
			},
			wantKept: []string{"stopped"},
		},
		{
			name:   "failed keeps failed stacks",
			filter: NewFailed(&config.Config{}),
			items: []types.CloudItem{
				&types.Stack{Name: "failed", State: types.Failed},
				&types.Stack{Name: "running", State: types.Running},
			},
			wantKept: []string{"failed"},
		},
		{
			name:   "unused keeps unused disks and alerts",
			filter: NewUnused(&config.Config{}),
			items: []types.CloudItem{
				&types.Disk{Name: "unused-disk", State: types.Unused},
				&types.Disk{Name: "used-disk", State: types.InUse},
				&types.Alert{Name: "unused-alert", State: types.Unused},
				&types.Alert{Name: "used-alert", State: types.InUse},
			},
			wantKept: []string{"unused-disk", "unused-alert"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := tc.filter.Execute(tc.items)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tc.wantKept, filterNames(result))
		})
	}
}

// TestConfigFilters covers the filter-config driven filters: match keeps items
// that match the config (inclusive); nomatch drops them (exclusive).
func TestConfigFilters(t *testing.T) {
	t.Parallel()
	fc, err := utils.LoadFilterConfig("testdata/sample-ignore.yml")
	assert.NoError(t, err)
	cfg := &config.Config{FilterConfig: fc}

	cases := []struct {
		name     string
		filter   types.Filter
		items    []types.CloudItem
		wantKept []string
	}{
		{
			name:   "match keeps items matching the config",
			filter: NewMatch(cfg),
			items: []types.CloudItem{
				&types.Instance{Name: "includeMeName-0", CloudType: types.AWS},
				&types.Instance{Name: "not-in-config", CloudType: types.AWS},
			},
			wantKept: []string{"includeMeName-0"},
		},
		{
			name:   "nomatch drops items matching the config",
			filter: NewNoMatch(cfg),
			items: []types.CloudItem{
				&types.Instance{Name: "skipThisName-0", CloudType: types.AWS},
				&types.Instance{Name: "keep-me", CloudType: types.AWS},
			},
			wantKept: []string{"keep-me"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := tc.filter.Execute(tc.items)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tc.wantKept, filterNames(result))
		})
	}
}

// TestFiltersRejectUnsupportedType verifies each state filter returns an error
// (instead of exiting the process) when given an item type it does not handle.
func TestFiltersRejectUnsupportedType(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cases := []struct {
		name string
		f    types.Filter
		item types.CloudItem
	}{
		{"failed", NewFailed(cfg), &types.Instance{Name: "x"}},
		{"stopped", NewStopped(cfg), &types.Stack{Name: "x"}},
		{"unused", NewUnused(cfg), &types.Instance{Name: "x"}},
		{"running", NewRunning(cfg), &types.Access{Name: "x"}},
		{"longrunning", NewLongRunning(cfg), &types.Access{Name: "x"}},
		{"oldaccess", NewOldAccess(cfg), &types.Instance{Name: "x"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tc.f.Execute([]types.CloudItem{tc.item})
			assert.Error(t, err)
		})
	}
}
