package operation

import (
	"testing"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

func TestOwnerlessInit(t *testing.T) {
	assert.NotNil(t, ctx.Filters[types.OwnerlessFilter])
}

func TestOwnerlessFilter(t *testing.T) {
	items := []types.CloudItem{
		&types.Instance{
			CloudType: types.AWS,
			Name:      "has owner",
			Owner:     "owner",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "missing owner",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "empty owner",
			Owner:     "",
		},
	}

	filteredItems := ownerless{}.Execute(items)

	assert.Len(t, filteredItems, 2)
}
