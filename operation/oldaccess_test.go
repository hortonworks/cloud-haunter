package operation

import (
	"testing"
	"time"

	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/stretchr/testify/assert"
)

func TestOldAccessInit(t *testing.T) {
	assert.NotNil(t, ctx.Operations[types.OLDACCESS])
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

	filteredItems := oldAccess{defaultAvailablePeriod}.filter(items)

	assert.Equal(t, 1, len(filteredItems))
}

func TestConvertToCloudItems(t *testing.T) {
	accesses := []*types.Access{&types.Access{}}

	items := oldAccess{}.convertToCloudItems(accesses)

	assert.Equal(t, len(accesses), len(items))
}
