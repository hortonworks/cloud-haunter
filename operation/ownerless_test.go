package operation

import (
	"testing"

	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/stretchr/testify/assert"
)

func TestOwnerlessInit(t *testing.T) {
	assert.NotNil(t, context.Operations[types.OWNERLESS])
}

func TestOwnerlessFilter(t *testing.T) {
	items := []types.CloudItem{
		&types.Instance{
			CloudType: types.AWS,
			Name:      "wrong owner",
			Tags:      types.Tags{"owner": "owner"},
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "missing owner",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "has owner",
			Tags:      types.Tags{context.AwsOwnerLabel: "owner"},
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "ignored",
			Tags:      types.Tags{context.AwsIgnoreLabel: "true"},
		},
	}

	filteredItems := ownerless{}.filter(items)

	assert.Equal(t, 2, len(filteredItems))
}
