package utils

import (
	"testing"
	"time"

	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

func TestIsAnyMatchFound(t *testing.T) {
	tags := map[string]string{"firsttag": "", "testtag": ""}
	assert.True(t, IsAnyMatch(tags, "testtag"))
}

func TestIsAnyMatchNotFound(t *testing.T) {
	tags := map[string]string{"firsttag": "", "sectag": ""}
	assert.False(t, IsAnyMatch(tags, "testtag"))
}

func TestIsAnyStartsWithFound(t *testing.T) {
	tags := map[string]string{"firsttag": "", "testtag": ""}
	assert.True(t, IsAnyStartsWith(tags, "te"))
}

func TestIsAnyStartsWithNotFound(t *testing.T) {
	tags := map[string]string{"firsttag": "", "testtag": ""}
	assert.False(t, IsAnyStartsWith(tags, "x"))
}

func Test_givenValidUnixTimeStamp_whenConvertTimeUnix_thenReturnsConvertedToTime(t *testing.T) {

	var unixTimestamp = "1527244797"
	convertedTime := ConvertTimeUnix(unixTimestamp)

	assert := assert.New(t)
	assert.Equal(convertedTime.In(time.UTC), time.Date(2018, 5, 25, 10, 39, 57, 0, time.UTC))
}

func Test_givenNotAnIntUnixTimeStamp_whenConvertTimeUnix_thenReturnsEpochZeroTime(t *testing.T) {

	var unixTimestamp = "not an int number"
	convertedTime := ConvertTimeUnix(unixTimestamp)
	epochZero := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

	assert := assert.New(t)
	assert.Equal(convertedTime, epochZero.Local())
}

func TestConvertTags(t *testing.T) {
	tags := map[string]*string{"firsttag": &(&types.S{S: ""}).S}
	assert.Equal(t, types.Tags{"firsttag": ""}, ConvertTags(tags))
}

func TestLoadIgnores(t *testing.T) {
	filterConfig, _ := LoadFilterConfig("testdata/filterConfig.yml")

	assert.Equal(t, []string{"awsName"}, filterConfig.GetFilterValues(types.ExcludeAccess, types.AWS, types.Name))
	assert.Equal(t, []string{"azureName"}, filterConfig.GetFilterValues(types.ExcludeAccess, types.AZURE, types.Name))
	assert.Equal(t, []string{"gcpName"}, filterConfig.GetFilterValues(types.ExcludeAccess, types.GCP, types.Name))

	assert.Equal(t, []string{"awsOwner"}, filterConfig.GetFilterValues(types.ExcludeAccess, types.AWS, types.Owner))
	assert.Equal(t, []string{"azureOwner"}, filterConfig.GetFilterValues(types.ExcludeAccess, types.AZURE, types.Owner))
	assert.Equal(t, []string{"gcpOwner"}, filterConfig.GetFilterValues(types.ExcludeAccess, types.GCP, types.Owner))

	assert.Equal(t, []string{"awsLabel"}, filterConfig.GetFilterValues(types.ExcludeInstance, types.AWS, types.Label))
	assert.Equal(t, []string{"azureLabel"}, filterConfig.GetFilterValues(types.ExcludeInstance, types.AZURE, types.Label))
	assert.Equal(t, []string{"gcpLabel"}, filterConfig.GetFilterValues(types.ExcludeInstance, types.GCP, types.Label))

	assert.Equal(t, []string{"awsName"}, filterConfig.GetFilterValues(types.ExcludeInstance, types.AWS, types.Name))
	assert.Equal(t, []string{"azureName"}, filterConfig.GetFilterValues(types.ExcludeInstance, types.AZURE, types.Name))
	assert.Equal(t, []string{"gcpName"}, filterConfig.GetFilterValues(types.ExcludeInstance, types.GCP, types.Name))

	assert.Equal(t, []string{"awsOwner"}, filterConfig.GetFilterValues(types.ExcludeInstance, types.AWS, types.Owner))
	assert.Equal(t, []string{"azureOwner"}, filterConfig.GetFilterValues(types.ExcludeInstance, types.AZURE, types.Owner))
	assert.Equal(t, []string{"gcpOwner"}, filterConfig.GetFilterValues(types.ExcludeInstance, types.GCP, types.Owner))

	assert.Nil(t, filterConfig.GetFilterValues(types.IncludeInstance, types.GCP, types.Owner))
	assert.Nil(t, filterConfig.GetFilterValues(types.IncludeAccess, types.AWS, types.Name))
}

func TestLoadIgnoresV2(t *testing.T) {
	filterConfig, _ := LoadFilterConfigV2("testdata/filterConfigV2.yml")

	assert.Equal(t, []string{"excludeThisValue"}, filterConfig.GetFilterValues(types.ExcludeAccess, types.AWS, types.Name))
	assert.Equal(t, []string{"excludeThisValue"}, filterConfig.GetFilterValues(types.ExcludeAccess, types.AZURE, types.Name))
	assert.Equal(t, []string{"excludeThisValue"}, filterConfig.GetFilterValues(types.ExcludeAccess, types.GCP, types.Name))

	assert.Equal(t, []string{"excludeThisValue"}, filterConfig.GetFilterValues(types.ExcludeInstance, types.AWS, types.Label))
	assert.Equal(t, []string{"excludeThisValue"}, filterConfig.GetFilterValues(types.ExcludeInstance, types.AZURE, types.Label))
	assert.Equal(t, []string{"excludeThisValue"}, filterConfig.GetFilterValues(types.ExcludeInstance, types.GCP, types.Label))

	assert.Equal(t, []string{"includeThisValue"}, filterConfig.GetFilterValues(types.IncludeAccess, types.AWS, types.Owner))
	assert.Equal(t, []string{"includeThisValue"}, filterConfig.GetFilterValues(types.IncludeAccess, types.AZURE, types.Owner))
	assert.Equal(t, []string{"includeThisValue"}, filterConfig.GetFilterValues(types.IncludeAccess, types.GCP, types.Owner))

	assert.Equal(t, []string{"includeThisValue"}, filterConfig.GetFilterValues(types.IncludeInstance, types.AWS, types.Name))
	assert.Equal(t, []string{"includeThisValue"}, filterConfig.GetFilterValues(types.IncludeInstance, types.AZURE, types.Name))
	assert.Equal(t, []string{"includeThisValue"}, filterConfig.GetFilterValues(types.IncludeInstance, types.GCP, types.Name))
}

func TestSplitListToMap(t *testing.T) {
	assert.Equal(t, map[string]bool{"a": true, "b": true, "A": true, "B": true}, SplitListToMap("a, b"))
}

func TestSplitListToMapEmpty(t *testing.T) {
	assert.Equal(t, map[string]bool{}, SplitListToMap(""))
}
