package filter

import (
	"testing"
	"time"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

// testLongRunningPeriod is a fixed period the tests build item ages against;
// RUNNING_PERIOD env parsing is covered in the config package.
const testLongRunningPeriod = 24 * time.Hour

// TestLongRunningByType exercises the per-type branches: each supported type in
// its "keep" state, created long enough ago, is retained; wrong-state items are
// dropped.
func TestLongRunningByType(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{LongRunningPeriod: testLongRunningPeriod}
	old := time.Now().Add(-testLongRunningPeriod).Add(-time.Hour)
	recent := time.Now()
	items := []types.CloudItem{
		&types.Instance{Name: "inst", State: types.Running, Created: old},
		&types.Stack{Name: "stack", State: types.Running, Created: old},
		&types.Database{Name: "db", State: types.Running, Created: old},
		&types.Disk{Name: "disk", State: types.Unused, Created: old},
		&types.Alert{Name: "alert", State: types.Unused, Created: old},
		// wrong state for their type -> dropped
		&types.Instance{Name: "inst-stopped", State: types.Stopped, Created: old},
		&types.Stack{Name: "stack-stopped", State: types.Stopped, Created: old},
		&types.Database{Name: "db-stopped", State: types.Stopped, Created: old},
		&types.Disk{Name: "disk-inuse", State: types.InUse, Created: old},
		&types.Alert{Name: "alert-inuse", State: types.InUse, Created: old},
		// correct state but created too recently -> dropped
		&types.Instance{Name: "inst-recent", State: types.Running, Created: recent},
	}

	result, err := longRunning{cfg}.Execute(items)

	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"inst", "stack", "db", "disk", "alert"}, filterNames(result))
}

func TestLongRunningFilterOnResources(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{LongRunningPeriod: testLongRunningPeriod}
	now := time.Now()
	items := []types.CloudItem{
		&types.Resource{
			CloudType:    types.AWS,
			Name:         "short running load balancer",
			Created:      now.Add(-testLongRunningPeriod).Add(1 * time.Second),
			ResourceType: types.LoadBalancer,
		},
		&types.Resource{
			CloudType:    types.AWS,
			Name:         "long running load balancer",
			Created:      now.Add(-testLongRunningPeriod).Add(-1 * time.Second),
			ResourceType: types.LoadBalancer,
		},
		&types.Resource{
			CloudType:    types.AWS,
			Name:         "VPC",
			ResourceType: types.Vpc,
		},
	}

	filteredItems, err := longRunning{cfg}.Execute(items)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(filteredItems))
	assert.NotEqual(t, "short running load balancer", filteredItems[0].GetName())
	assert.NotEqual(t, "short running load balancer", filteredItems[1].GetName())
}
