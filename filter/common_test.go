package operation

import (
	"testing"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
	"github.com/stretchr/testify/assert"
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
	filterConfig, _ := utils.LoadFilterConfig("testdata/sample-ignore.yml")

	for _, item := range items {
		isFiltered := isFilterMatch("TEST", item, types.ExclusiveFilter, filterConfig)
		assert.True(t, isFiltered, "Item found: "+item.GetName())
	}
}

func TestIsIncluded(t *testing.T) {
	items := []types.CloudItem{
		&types.Instance{
			CloudType: types.AWS,
			Name:      "includeMeName-0",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "included-by-owner",
			Owner:     "includeMeOwner-0",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "included-by-label",
			Tags:      types.Tags{"includeMeLabel-0": "true"},
		},
		&types.Access{
			CloudType: types.AWS,
			Name:      "includeMeName-0",
		},
		&types.Access{
			CloudType: types.AWS,
			Name:      "included-by-owner",
			Owner:     "includeMeOwner-0",
		},
	}
	filterConfig, _ := utils.LoadFilterConfig("testdata/sample-ignore.yml")

	for _, item := range items {
		isFiltered := isFilterMatch("TEST", item, types.InclusiveFilter, filterConfig)
		assert.True(t, isFiltered, "Item found: "+item.GetName())
	}
}

func TestIncludedAndExcludedWithInclusiveFilter(t *testing.T) {
	items := []types.CloudItem{
		&types.Instance{
			CloudType: types.AZURE,
			Name:      "ignored-by-ignore-label",
			Tags:      types.Tags{ctx.AwsIgnoreLabel: "true"},
		},
		&types.Instance{
			CloudType: types.AZURE,
			Name:      "skipThisName-0",
		},
		&types.Instance{
			CloudType: types.AZURE,
			Name:      "ignored-by-owner",
			Owner:     "skipThisOwner-0",
		},
		&types.Instance{
			CloudType: types.AZURE,
			Name:      "ignored-by-label",
			Tags:      types.Tags{"skipThisLabel-0": "true"},
		},
		&types.Access{
			CloudType: types.AZURE,
			Name:      "skipThisAccess-0",
		},
		&types.Access{
			CloudType: types.AZURE,
			Name:      "ignored-by-owner",
			Owner:     "skipThisOwner-0",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "includeMeName-0",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "included-by-owner",
			Owner:     "includeMeOwner-0",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "included-by-label",
			Tags:      types.Tags{"includeMeLabel-0": "true"},
		},
		&types.Access{
			CloudType: types.AWS,
			Name:      "includeMeName-0",
		},
		&types.Access{
			CloudType: types.AWS,
			Name:      "included-by-owner",
			Owner:     "includeMeOwner-0",
		},
	}
	filterConfig, _ := utils.LoadFilterConfig("testdata/sample-ignore.yml")

	for _, item := range items {
		isFiltered := isFilterMatch("TEST", item, types.InclusiveFilter, filterConfig)
		if item.GetCloudType() == types.AZURE {
			assert.False(t, isFiltered, "Item found: "+item.GetName())
		} else {
			assert.True(t, isFiltered, "Item found: "+item.GetName())
		}
	}
}

func TestIncludedAndExcludedWithExclusiveFilter(t *testing.T) {
	items := []types.CloudItem{
		&types.Instance{
			CloudType: types.AZURE,
			Name:      "ignored-by-ignore-label",
			Tags:      types.Tags{ctx.AwsIgnoreLabel: "true"},
		},
		&types.Instance{
			CloudType: types.AZURE,
			Name:      "skipThisName-0",
		},
		&types.Instance{
			CloudType: types.AZURE,
			Name:      "ignored-by-owner",
			Owner:     "skipThisOwner-0",
		},
		&types.Instance{
			CloudType: types.AZURE,
			Name:      "ignored-by-label",
			Tags:      types.Tags{"skipThisLabel-0": "true"},
		},
		&types.Access{
			CloudType: types.AZURE,
			Name:      "skipThisAccess-0",
		},
		&types.Access{
			CloudType: types.AZURE,
			Name:      "ignored-by-owner",
			Owner:     "skipThisOwner-0",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "includeMeName-0",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "included-by-owner",
			Owner:     "includeMeOwner-0",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "included-by-label",
			Tags:      types.Tags{"includeMeLabel-0": "true"},
		},
		&types.Access{
			CloudType: types.AWS,
			Name:      "includeMeName-0",
		},
		&types.Access{
			CloudType: types.AWS,
			Name:      "included-by-owner",
			Owner:     "includeMeOwner-0",
		},
	}
	filterConfig, _ := utils.LoadFilterConfig("testdata/sample-ignore.yml")

	for _, item := range items {
		isFiltered := isFilterMatch("TEST", item, types.ExclusiveFilter, filterConfig)
		if item.GetCloudType() == types.AZURE {
			assert.True(t, isFiltered, "Item found: "+item.GetName())
		} else {
			assert.False(t, isFiltered, "Item found: "+item.GetName())
		}
	}
}

func TestFilter(t *testing.T) {
	items := []types.CloudItem{
		&types.Instance{},
	}

	filtered := filter("TEST", items, types.ExclusiveFilter, func(types.CloudItem) bool {
		return true
	})

	assert.Equal(t, len(items), len(filtered))
}

func TestInclusiveFilterWithNoMatch(t *testing.T) {
	filterConfig, _ := utils.LoadFilterConfig("testdata/sample-ignore.yml")
	ctx.FilterConfig = filterConfig

	items := []types.CloudItem{
		&types.Instance{
			Name:      "includeMeNamX",
			CloudType: types.AWS,
		},
	}

	filtered := filter("TEST", items, types.InclusiveFilter, func(types.CloudItem) bool {
		return true
	})

	assert.Equal(t, 0, len(filtered))
}

func TestInclusiveFilter(t *testing.T) {
	filterConfig, _ := utils.LoadFilterConfig("testdata/sample-ignore.yml")
	ctx.FilterConfig = filterConfig

	tags := types.Tags{}
	tags["includeMeLabel"] = "test"
	items := []types.CloudItem{
		&types.Instance{
			Name:      "includeMeNamX",
			CloudType: types.AWS,
		},
		&types.Instance{
			Name:      "includeMeName",
			CloudType: types.AWS,
		},
		&types.Instance{
			Name:      "testName0",
			CloudType: types.AWS,
			Owner:     "includeMeOwner",
		},
		&types.Instance{
			Name:      "testName1",
			CloudType: types.AZURE,
			Owner:     "includeMeOwner",
		},
		&types.Instance{
			Name:      "testName2",
			CloudType: types.AZURE,
			Tags:      tags,
		},
	}

	filtered := filter("TEST", items, types.InclusiveFilter, func(types.CloudItem) bool {
		return true
	})

	assert.Equal(t, 4, len(filtered))
}

func TestExclusiveFilter(t *testing.T) {
	filterConfig, _ := utils.LoadFilterConfig("testdata/sample-ignore.yml")
	ctx.FilterConfig = filterConfig

	tags := types.Tags{}
	tags["skipThisLabel"] = "test"
	items := []types.CloudItem{
		&types.Instance{
			Name:      "skipThisNamX",
			CloudType: types.AWS,
		},
		&types.Instance{
			Name:      "skipThisName",
			CloudType: types.AWS,
		},
		&types.Instance{
			Name:      "testName0",
			CloudType: types.AWS,
			Owner:     "includeMeOwner",
		},
		&types.Instance{
			Name:      "testName1",
			CloudType: types.AZURE,
			Owner:     "includeMeOwner",
		},
		&types.Instance{
			Name:      "testName2",
			CloudType: types.AZURE,
			Tags:      tags,
		},
	}

	filtered := filter("TEST", items, types.ExclusiveFilter, func(types.CloudItem) bool {
		return true
	})

	assert.Equal(t, 3, len(filtered))
}

func TestIsInstance(t *testing.T) {
	assert.True(t, isInstance(types.Instance{}))
}

func TestIsNotInstance(t *testing.T) {
	assert.False(t, isInstance(types.Access{}))
}
