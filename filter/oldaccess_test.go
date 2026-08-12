package filter

import (
	"testing"
	"time"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

// testAccessAvailablePeriod is a fixed period the test builds item ages against;
// ACCESS_AVAILABLE_PERIOD env parsing is covered in the config package.
const testAccessAvailablePeriod = 120 * 24 * time.Hour

func TestOldAccessFilter(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{AccessAvailablePeriod: testAccessAvailablePeriod}
	now := time.Now()
	items := []types.CloudItem{
		&types.Access{
			CloudType: types.AWS,
			Name:      "short live",
			Created:   now.Add(-testAccessAvailablePeriod).Add(1 * time.Second),
		},
		&types.Access{
			CloudType: types.AWS,
			Name:      "long live",
			Created:   now.Add(-testAccessAvailablePeriod).Add(-1 * time.Second),
		},
	}

	filteredItems, err := oldAccess{cfg}.Execute(items)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(filteredItems))
}
