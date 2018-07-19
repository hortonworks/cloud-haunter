package operation

import (
	"testing"
	"time"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

func TestLongRunningInit(t *testing.T) {
	assert.NotNil(t, ctx.Filters[types.LongRunning])
}

func TestLongRunningFilter(t *testing.T) {
	now := time.Now()
	items := []types.CloudItem{
		&types.Instance{
			CloudType: types.AWS,
			Name:      "short running",
			Created:   now.Add(-defaultRunningPeriod).Add(1 * time.Second),
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "long running",
			Created:   now.Add(-defaultRunningPeriod).Add(-1 * time.Second),
		},
	}

	filteredItems := longRunning{defaultRunningPeriod}.Execute(items)

	assert.Equal(t, 1, len(filteredItems))
}
