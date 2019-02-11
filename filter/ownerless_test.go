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
			Name:      "capital owner",
			Tags:      types.Tags{"oWNER": "owner"},
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "missing owner",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "has owner",
			Tags:      types.Tags{ctx.OwnerLabel: "owner"},
		},
	}

	filteredItems := ownerless{}.Execute(items)

	assert.Equal(t, 1, len(filteredItems))
}
