package action

import (
	"errors"
	"testing"

	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

// TestStopRoutesInstancesAndDatabases verifies that stoppable items are grouped
// by kind and dispatched to the matching provider method, while items that
// cannot be stopped are ignored. stop asserts item.(*types.Instance), so the
// items must be pointers. It also exercises the public NewStop constructor.
func TestStopRoutesInstancesAndDatabases(t *testing.T) {
	t.Parallel()
	mock := newMockProvider()
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})

	err := NewStop(cfg).Execute(types.Instances, nil, []types.CloudItem{
		&types.Instance{CloudType: types.AWS, Name: "i1"},
		&types.Instance{CloudType: types.AWS, Name: "i2"},
		&types.Database{CloudType: types.AWS, Name: "db1"},
		&types.Stack{CloudType: types.AWS, Name: "not-stoppable"},
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, mock.stopInstances, "StopInstances called once for the cloud")
	assert.Equal(t, 1, mock.stopDatabases, "StopDatabases called once for the cloud")
	assert.Len(t, mock.instances.Get(types.AWS), 2, "both instances passed")
	assert.Len(t, mock.databases.Get(types.AWS), 1, "the database passed")
}

// TestStopMultipleClouds verifies each cloud's provider is invoked for its own
// instances.
func TestStopMultipleClouds(t *testing.T) {
	t.Parallel()
	aws := newMockProvider()
	gcp := newMockProvider()
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: aws, types.GCP: gcp})

	err := stopAction{cfg}.Execute(types.Instances, nil, []types.CloudItem{
		&types.Instance{CloudType: types.AWS},
		&types.Instance{CloudType: types.GCP},
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, aws.stopInstances)
	assert.Equal(t, 1, gcp.stopInstances)
}

// TestStopIgnoresNonStoppableItems verifies no provider call is made when no
// item is an instance or database.
func TestStopIgnoresNonStoppableItems(t *testing.T) {
	t.Parallel()
	mock := newMockProvider()
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})

	err := stopAction{cfg}.Execute(types.Instances, nil, []types.CloudItem{
		&types.Stack{CloudType: types.AWS},
		&types.Disk{CloudType: types.AWS},
	})

	assert.NoError(t, err)
	assert.Equal(t, 0, mock.stopInstances)
	assert.Equal(t, 0, mock.stopDatabases)
}

// TestStopReturnsErrorOnProviderFailure verifies a provider error is aggregated
// and returned instead of crashing the process (previously a goroutine panic).
func TestStopReturnsErrorOnProviderFailure(t *testing.T) {
	t.Parallel()
	mock := newMockProvider()
	mock.opErr = []error{errors.New("boom")}
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})

	err := stopAction{cfg}.Execute(types.Instances, nil, []types.CloudItem{
		&types.Instance{CloudType: types.AWS},
	})

	assert.Error(t, err)
}

// TestStopReturnsErrorOnDatabaseProviderFailure covers the database branch of
// the provider-error path (distinct goroutine from instances).
func TestStopReturnsErrorOnDatabaseProviderFailure(t *testing.T) {
	t.Parallel()
	mock := newMockProvider()
	mock.opErr = []error{errors.New("boom")}
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})

	err := stopAction{cfg}.Execute(types.Instances, nil, []types.CloudItem{
		&types.Database{CloudType: types.AWS},
	})

	assert.Error(t, err)
}
