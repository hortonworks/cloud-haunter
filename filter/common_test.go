package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsIgnored(t *testing.T) {
	items := []types.CloudItem{
		&types.Instance{
			CloudType: types.AWS,
			Name:      "ignored-by-ignore-label",
			Tags:      types.Tags{ctx.AwsIgnoreLabel: "true"},
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "skipThisName-0",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "ignored-by-owner",
			Owner:     "skipThisOwner-0",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "ignored-by-label",
			Tags:      types.Tags{"skipThisLabel-0": "true"},
		},
		&types.Access{
			CloudType: types.AWS,
			Name:      "skipThisAccess-0",
		},
		&types.Access{
			CloudType: types.AWS,
			Name:      "ignored-by-owner",
			Owner:     "skipThisOwner-0",
		},
	}
	ignores, _ := utils.LoadIgnores("testdata/sample-ignore.yml")

	for _, item := range items {
		isIgnored := isIgnored(item, ignores)
		assert.True(t, isIgnored, "Item found: "+item.GetName())
	}
}

func TestFilter(t *testing.T) {
	items := []types.CloudItem{
		&types.Instance{},
	}

	filtered := filter(items, func(types.CloudItem) bool {
		return true
	})

	assert.Equal(t, len(items), len(filtered))
}
