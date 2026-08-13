package filter

import (
	"testing"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
	"github.com/stretchr/testify/assert"
)

// testConfig builds an isolated config for a single test. config.DefaultIgnoreLabel
// is a read-only default, so referencing it here does not share mutable state.
func testConfig(fc types.IFilterConfig) *config.Config {
	return &config.Config{FilterConfig: fc, IgnoreLabel: config.DefaultIgnoreLabel}
}

func TestIsIgnored(t *testing.T) {
	t.Parallel()
	items := []types.CloudItem{
		&types.Instance{
			CloudType: types.AWS,
			Name:      "ignored-by-ignore-label",
			Tags:      types.Tags{config.DefaultIgnoreLabel: "true"},
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
	cfg := testConfig(filterConfig)

	for _, item := range items {
		isFiltered := isFilterMatch(cfg, "TEST", item, types.ExclusiveFilter)
		assert.True(t, isFiltered, "Item found: "+item.GetName())
	}
}

func TestIsIncluded(t *testing.T) {
	t.Parallel()
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
	cfg := testConfig(filterConfig)

	for _, item := range items {
		isFiltered := isFilterMatch(cfg, "TEST", item, types.InclusiveFilter)
		assert.True(t, isFiltered, "Item found: "+item.GetName())
	}
}

// mixedIncludeExcludeItems returns AZURE items that the sample config marks as
// skip/ignore and AWS items it marks as include, for the inclusive/exclusive
// filter scenarios.
func mixedIncludeExcludeItems() []types.CloudItem {
	return []types.CloudItem{
		&types.Instance{
			CloudType: types.AZURE,
			Name:      "ignored-by-ignore-label",
			Tags:      types.Tags{config.DefaultIgnoreLabel: "true"},
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
}

func TestIncludedAndExcludedWithInclusiveFilter(t *testing.T) {
	t.Parallel()
	items := mixedIncludeExcludeItems()
	filterConfig, _ := utils.LoadFilterConfig("testdata/sample-ignore.yml")
	cfg := testConfig(filterConfig)

	for _, item := range items {
		isFiltered := isFilterMatch(cfg, "TEST", item, types.InclusiveFilter)
		if item.GetCloudType() == types.AZURE {
			assert.False(t, isFiltered, "Item found: "+item.GetName())
		} else {
			assert.True(t, isFiltered, "Item found: "+item.GetName())
		}
	}
}

func TestIncludedAndExcludedWithExclusiveFilter(t *testing.T) {
	t.Parallel()
	items := mixedIncludeExcludeItems()
	filterConfig, _ := utils.LoadFilterConfig("testdata/sample-ignore.yml")
	cfg := testConfig(filterConfig)

	for _, item := range items {
		isFiltered := isFilterMatch(cfg, "TEST", item, types.ExclusiveFilter)
		if item.GetCloudType() == types.AZURE {
			assert.True(t, isFiltered, "Item found: "+item.GetName())
		} else {
			assert.False(t, isFiltered, "Item found: "+item.GetName())
		}
	}
}

func TestFilter(t *testing.T) {
	t.Parallel()
	items := []types.CloudItem{
		&types.Instance{},
	}

	filtered, err := filter(&config.Config{}, "TEST", items, types.ExclusiveFilter, func(types.CloudItem) (bool, error) {
		return true, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, len(items), len(filtered))
}

func TestInclusiveFilterWithNoMatch(t *testing.T) {
	t.Parallel()
	filterConfig, _ := utils.LoadFilterConfig("testdata/sample-ignore.yml")
	cfg := testConfig(filterConfig)

	items := []types.CloudItem{
		&types.Instance{
			Name:      "includeMeNamX",
			CloudType: types.AWS,
		},
	}

	filtered, err := filter(cfg, "TEST", items, types.InclusiveFilter, func(types.CloudItem) (bool, error) {
		return true, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 0, len(filtered))
}

func TestInclusiveFilter(t *testing.T) {
	t.Parallel()
	filterConfig, _ := utils.LoadFilterConfig("testdata/sample-ignore.yml")
	cfg := testConfig(filterConfig)

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

	filtered, err := filter(cfg, "TEST", items, types.InclusiveFilter, func(types.CloudItem) (bool, error) {
		return true, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 4, len(filtered))
}

func TestExclusiveFilter(t *testing.T) {
	t.Parallel()
	filterConfig, _ := utils.LoadFilterConfig("testdata/sample-ignore.yml")
	cfg := testConfig(filterConfig)

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

	filtered, err := filter(cfg, "TEST", items, types.ExclusiveFilter, func(types.CloudItem) (bool, error) {
		return true, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, len(filtered))
}
