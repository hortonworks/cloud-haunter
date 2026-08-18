package action

import (
	"errors"
	"testing"

	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

// TestTerminationByType verifies that Execute dispatches each cloud-item type to
// the matching provider method.
func TestTerminationByType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		item  types.CloudItem
		count func(*mockProvider) int
	}{
		{"instance", &types.Instance{CloudType: types.AWS}, func(p *mockProvider) int { return p.terminateInstances }},
		{"stack", &types.Stack{CloudType: types.AWS}, func(p *mockProvider) int { return p.terminateStacks }},
		{"disk", &types.Disk{CloudType: types.AWS}, func(p *mockProvider) int { return p.deleteDisks }},
		{"image", &types.Image{CloudType: types.AWS}, func(p *mockProvider) int { return p.deleteImages }},
		{"alert", &types.Alert{CloudType: types.AWS}, func(p *mockProvider) int { return p.deleteAlerts }},
		{"database", &types.Database{CloudType: types.AWS}, func(p *mockProvider) int { return p.deleteDatabases }},
		{"resource", &types.Resource{CloudType: types.AWS}, func(p *mockProvider) int { return p.terminateResources }},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock := newMockProvider()
			cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})

			err := terminationAction{cfg}.Execute(types.Instances, nil, []types.CloudItem{tc.item})

			assert.NoError(t, err)
			assert.Equal(t, 1, tc.count(mock), "expected the matching provider method to be called once")
		})
	}
}

// TestTerminationOnlyRegisteredClouds verifies that items belonging to a cloud
// that has no registered provider are not terminated.
func TestTerminationOnlyRegisteredClouds(t *testing.T) {
	t.Parallel()
	mock := newMockProvider()
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})

	err := NewTermination(cfg).Execute(types.Instances, nil, []types.CloudItem{
		&types.Instance{CloudType: types.AWS, Name: "aws-1"},
		&types.Instance{CloudType: types.AWS, Name: "aws-2"},
		&types.Instance{CloudType: types.GCP, Name: "gcp-1"},
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, mock.terminateInstances, "AWS provider terminates once")
	assert.Len(t, mock.instances.Get(types.AWS), 2, "both AWS instances are passed")
}

// TestTerminationReturnsErrorOnUnsupportedType exercises the previously
// panic-only path: an item type termination doesn't handle now yields an error.
func TestTerminationReturnsErrorOnUnsupportedType(t *testing.T) {
	t.Parallel()
	mock := newMockProvider()
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})

	err := terminationAction{cfg}.Execute(types.Instances, nil, []types.CloudItem{
		&types.Storage{CloudType: types.AWS},
	})

	assert.Error(t, err)
}

// TestTerminationReturnsErrorOnProviderFailure verifies a provider error is
// aggregated and returned instead of crashing the process.
func TestTerminationReturnsErrorOnProviderFailure(t *testing.T) {
	t.Parallel()
	mock := newMockProvider()
	mock.opErr = []error{errors.New("boom")}
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})

	err := terminationAction{cfg}.Execute(types.Instances, nil, []types.CloudItem{
		&types.Instance{CloudType: types.AWS},
	})

	assert.Error(t, err)
}
