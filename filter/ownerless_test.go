package filter

import (
	"testing"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

func TestOwnerlessFilter(t *testing.T) {
	t.Parallel()
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

	filteredItems, err := NewOwnerless(&config.Config{}).Execute(items)

	assert.NoError(t, err)
	assert.Len(t, filteredItems, 2)
}
