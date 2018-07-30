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

	assert.Equal(t, []string{"awsName"}, filterConfig.ExcludeAccess.Aws.Names)
	assert.Equal(t, []string{"azureName"}, filterConfig.ExcludeAccess.Azure.Names)
	assert.Equal(t, []string{"gcpName"}, filterConfig.ExcludeAccess.Gcp.Names)

	assert.Equal(t, []string{"awsOwner"}, filterConfig.ExcludeAccess.Aws.Owners)
	assert.Equal(t, []string{"azureOwner"}, filterConfig.ExcludeAccess.Azure.Owners)
	assert.Equal(t, []string{"gcpOwner"}, filterConfig.ExcludeAccess.Gcp.Owners)

	assert.Equal(t, []string{"awsLabel"}, filterConfig.ExcludeInstance.Aws.Labels)
	assert.Equal(t, []string{"azureLabel"}, filterConfig.ExcludeInstance.Azure.Labels)
	assert.Equal(t, []string{"gcpLabel"}, filterConfig.ExcludeInstance.Gcp.Labels)

	assert.Equal(t, []string{"awsName"}, filterConfig.ExcludeInstance.Aws.Names)
	assert.Equal(t, []string{"azureName"}, filterConfig.ExcludeInstance.Azure.Names)
	assert.Equal(t, []string{"gcpName"}, filterConfig.ExcludeInstance.Gcp.Names)

	assert.Equal(t, []string{"awsOwner"}, filterConfig.ExcludeInstance.Aws.Owners)
	assert.Equal(t, []string{"azureOwner"}, filterConfig.ExcludeInstance.Azure.Owners)
	assert.Equal(t, []string{"gcpOwner"}, filterConfig.ExcludeInstance.Gcp.Owners)
}

func TestSplitListToMap(t *testing.T) {
	assert.Equal(t, map[string]bool{"a": true, "b": true, "A": true, "B": true}, SplitListToMap("a, b"))
}

func TestSplitListToMapEmpty(t *testing.T) {
	assert.Equal(t, map[string]bool{}, SplitListToMap(""))
}
