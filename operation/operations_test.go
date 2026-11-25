package operation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

// TestOperationsCollectItems drives every collecting operation end-to-end:
// Execute -> collect -> provider.GetX -> convert -> wait, asserting the provider
// items surface as CloudItems tagged with the right cloud.
func TestOperationsCollectItems(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		mock    *mockProvider
		newOp   func(*config.Config) types.Operation
		wantLen int
	}{
		{"instances", &mockProvider{instances: []*types.Instance{{Name: "a", CloudType: types.DUMMY}, {Name: "b", CloudType: types.DUMMY}}}, NewInstances, 2},
		{"accesses", &mockProvider{accesses: []*types.Access{{Name: "a", CloudType: types.DUMMY}}}, NewAccess, 1},
		{"databases", &mockProvider{databases: []*types.Database{{Name: "a", CloudType: types.DUMMY}}}, NewDatabases, 1},
		{"disks", &mockProvider{disks: []*types.Disk{{Name: "a", CloudType: types.DUMMY}}}, NewDisks, 1},
		{"images", &mockProvider{images: []*types.Image{{Name: "a", CloudType: types.DUMMY}}}, NewImages, 1},
		{"stacks", &mockProvider{stacks: []*types.Stack{{Name: "a", CloudType: types.DUMMY}}}, NewStacks, 1},
		{"alerts", &mockProvider{alerts: []*types.Alert{{Name: "a", CloudType: types.DUMMY}}}, NewAlerts, 1},
		{"storages", &mockProvider{storages: []*types.Storage{{Name: "a", CloudType: types.DUMMY}}}, NewStorages, 1},
		{"resources", &mockProvider{resources: []*types.Resource{{Name: "a", CloudType: types.DUMMY}}}, NewResources, 1},
		{"billable resources", &mockProvider{
			instances: []*types.Instance{{Name: "a", CloudType: types.DUMMY}},
			databases: []*types.Database{{Name: "b", CloudType: types.DUMMY}},
			disks:     []*types.Disk{{Name: "c", CloudType: types.DUMMY}, {Name: "d", CloudType: types.DUMMY}},
			stacks:    []*types.Stack{{Name: "not billable", CloudType: types.DUMMY}},
		}, NewBillableResources, 4},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := providerCfg(map[types.CloudType]types.CloudProvider{types.DUMMY: tc.mock})

			items, err := tc.newOp(cfg).Execute([]types.CloudType{types.DUMMY})

			assert.NoError(t, err)
			assert.Len(t, items, tc.wantLen)
			for _, item := range items {
				assert.Equal(t, types.DUMMY, item.GetCloudType())
			}
		})
	}
}

// TestInstancesAggregatesAcrossClouds verifies items from every requested cloud
// are merged into a single result.
func TestInstancesAggregatesAcrossClouds(t *testing.T) {
	t.Parallel()
	cfg := providerCfg(map[types.CloudType]types.CloudProvider{
		types.AWS: &mockProvider{instances: []*types.Instance{{Name: "aws-1", CloudType: types.AWS}}},
		types.GCP: &mockProvider{instances: []*types.Instance{{Name: "gcp-1", CloudType: types.GCP}}},
	})

	items, err := NewInstances(cfg).Execute([]types.CloudType{types.AWS, types.GCP})
	assert.NoError(t, err)

	names := []string{}
	for _, item := range items {
		names = append(names, item.GetName())
	}
	assert.ElementsMatch(t, []string{"aws-1", "gcp-1"}, names)
}

// TestOperationsFailOnProviderError verifies that for every operation a provider
// error fails the operation, even when another provider succeeds, so an action is
// never run on an inventory with a provider silently missing from it.
func TestOperationsFailOnProviderError(t *testing.T) {
	t.Parallel()
	ops := map[string]func(*config.Config) types.Operation{
		"instances": NewInstances,
		"accesses":  NewAccess,
		"databases": NewDatabases,
		"disks":     NewDisks,
		"images":    NewImages,
		"stacks":    NewStacks,
		"alerts":    NewAlerts,
		"storages":  NewStorages,
		"resources": NewResources,
		"billable":  NewBillableResources,
	}

	for name, newOp := range ops {
		name, newOp := name, newOp
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := providerCfg(map[types.CloudType]types.CloudProvider{
				types.DUMMY: &mockProvider{err: errors.New("boom")},
				types.AWS:   &mockProvider{},
			})

			items, err := newOp(cfg).Execute([]types.CloudType{types.DUMMY, types.AWS})
			assert.ErrorContains(t, err, "[DUMMY] boom", "the error names the failing cloud")
			assert.Empty(t, items)
		})
	}
}

// TestOperationsSkipUnsupportedProviders verifies that a provider which does not
// support an operation does not fail it: its "not supported" error is skipped and
// the other providers' items are returned.
func TestOperationsSkipUnsupportedProviders(t *testing.T) {
	t.Parallel()
	ops := map[string]func(*config.Config) types.Operation{
		"instances": NewInstances,
		"accesses":  NewAccess,
		"databases": NewDatabases,
		"disks":     NewDisks,
		"images":    NewImages,
		"stacks":    NewStacks,
		"alerts":    NewAlerts,
		"storages":  NewStorages,
		"resources": NewResources,
		"billable":  NewBillableResources,
	}

	for name, newOp := range ops {
		name, newOp := name, newOp
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := providerCfg(map[types.CloudType]types.CloudProvider{
				types.DUMMY: &mockProvider{err: fmt.Errorf("[DUMMY] getting %s: %w", name, types.ErrNotSupported)},
				types.AWS: &mockProvider{
					instances: []*types.Instance{{Name: "aws"}},
					accesses:  []*types.Access{{Name: "aws"}},
					databases: []*types.Database{{Name: "aws"}},
					disks:     []*types.Disk{{Name: "aws"}},
					images:    []*types.Image{{Name: "aws"}},
					stacks:    []*types.Stack{{Name: "aws"}},
					alerts:    []*types.Alert{{Name: "aws"}},
					storages:  []*types.Storage{{Name: "aws"}},
					resources: []*types.Resource{{Name: "aws"}},
				},
			})

			items, err := newOp(cfg).Execute([]types.CloudType{types.DUMMY, types.AWS})

			assert.NoError(t, err)
			assert.NotEmpty(t, items, "the items of the other provider are kept")
			for _, item := range items {
				assert.Equal(t, "aws", item.GetName())
			}
		})
	}
}

// TestBillableResourcesWithCloudNotSupportingDisks covers the documented budget
// reminder run on every cloud, where one cloud (like Azure) supports instances
// and databases, but not disks.
func TestBillableResourcesWithCloudNotSupportingDisks(t *testing.T) {
	t.Parallel()
	cfg := providerCfg(map[types.CloudType]types.CloudProvider{
		types.DUMMY: &mockProvider{
			instances: []*types.Instance{{Name: "dummy-instance"}},
			databases: []*types.Database{{Name: "dummy-database"}},
			disksErr:  fmt.Errorf("[DUMMY] getting disks: %w", types.ErrNotSupported),
		},
		types.AWS: &mockProvider{
			instances: []*types.Instance{{Name: "aws-instance"}},
			disks:     []*types.Disk{{Name: "aws-disk"}},
		},
	})

	items, err := NewBillableResources(cfg).Execute([]types.CloudType{types.DUMMY, types.AWS})

	assert.NoError(t, err)
	var names []string
	for _, item := range items {
		names = append(names, item.GetName())
	}
	assert.ElementsMatch(t, []string{"dummy-instance", "dummy-database", "aws-instance", "aws-disk"}, names)
}

func TestNewReadImages(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, NewReadImages())
}
