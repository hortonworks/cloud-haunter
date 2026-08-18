package operation

import (
	"errors"
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

// TestOperationsReturnEmptyOnProviderError verifies that for every operation a
// provider error is logged and yields an empty result rather than panicking.
func TestOperationsReturnEmptyOnProviderError(t *testing.T) {
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
	}

	for name, newOp := range ops {
		name, newOp := name, newOp
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := providerCfg(map[types.CloudType]types.CloudProvider{
				types.DUMMY: &mockProvider{err: errors.New("boom")},
			})

			items, err := newOp(cfg).Execute([]types.CloudType{types.DUMMY})
			assert.NoError(t, err)
			assert.Empty(t, items)
		})
	}
}

func TestNewReadImages(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, NewReadImages())
}
