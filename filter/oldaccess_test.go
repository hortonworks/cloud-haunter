package operation

import (
	"testing"
	"time"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

func TestOldAccessInit(t *testing.T) {
	assert.NotNil(t, ctx.Filters[types.OldAccessFilter])
}

func TestOldAccessFilter(t *testing.T) {
	now := time.Now()
	items := []types.CloudItem{
		&types.Access{
			CloudType: types.AWS,
			Name:      "short live",
			Created:   now.Add(-defaultAvailablePeriod).Add(1 * time.Second),
		},
		&types.Access{
			CloudType: types.AWS,
			Name:      "long live",
			Created:   now.Add(-defaultAvailablePeriod).Add(-1 * time.Second),
		},
	}

	filteredItems := oldAccess{defaultAvailablePeriod}.Execute(items)

	assert.Equal(t, 1, len(filteredItems))
}
