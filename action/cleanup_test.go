package action

import (
	"errors"
	"testing"

	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

// TestCleanupInvokesProviderForStorages verifies that storage items are routed
// to the provider's CleanupStorages with the configured retention period, and
// that only items for a registered cloud are processed.
func TestCleanupInvokesProviderForStorages(t *testing.T) {
	t.Parallel()
	mock := newMockProvider()
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})
	cfg.RetentionDays = 42

	err := NewCleanup(cfg).Execute(types.Storages, nil, []types.CloudItem{
		&types.Storage{CloudType: types.AWS, Name: "s1"},
		&types.Storage{CloudType: types.AWS, Name: "s2"},
		&types.Storage{CloudType: types.GCP, Name: "no-provider"},
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, mock.cleanupStorages, "CleanupStorages called once for AWS")
	assert.Equal(t, 42, mock.retentionDays, "configured retention flows to the provider")
	assert.Len(t, mock.storages.Get(types.AWS), 2, "both AWS storages passed")
}

// TestCleanupReturnsErrorOnUnsupportedType exercises the previously panic-only
// path: cleanup only supports storages, so any other item type yields an error.
func TestCleanupReturnsErrorOnUnsupportedType(t *testing.T) {
	t.Parallel()
	mock := newMockProvider()
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})

	err := cleanupAction{cfg}.Execute(types.Storages, nil, []types.CloudItem{
		&types.Instance{CloudType: types.AWS},
	})

	assert.Error(t, err)
}

// TestCleanupReturnsErrorOnProviderFailure verifies a provider error is
// aggregated and returned instead of crashing the process.
func TestCleanupReturnsErrorOnProviderFailure(t *testing.T) {
	t.Parallel()
	mock := newMockProvider()
	mock.opErr = []error{errors.New("boom")}
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})

	err := cleanupAction{cfg}.Execute(types.Storages, nil, []types.CloudItem{
		&types.Storage{CloudType: types.AWS},
	})

	assert.Error(t, err)
}
