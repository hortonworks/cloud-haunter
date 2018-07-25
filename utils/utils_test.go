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
	ignores, _ := LoadIgnores("testdata/ignores.yml")

	assert.Equal(t, []string{"awsName"}, ignores.Access.Aws.Names)
	assert.Equal(t, []string{"azureName"}, ignores.Access.Azure.Names)
	assert.Equal(t, []string{"gcpName"}, ignores.Access.Gcp.Names)

	assert.Equal(t, []string{"awsOwner"}, ignores.Access.Aws.Owners)
	assert.Equal(t, []string{"azureOwner"}, ignores.Access.Azure.Owners)
	assert.Equal(t, []string{"gcpOwner"}, ignores.Access.Gcp.Owners)

	assert.Equal(t, []string{"awsLabel"}, ignores.Instance.Aws.Labels)
	assert.Equal(t, []string{"azureLabel"}, ignores.Instance.Azure.Labels)
	assert.Equal(t, []string{"gcpLabel"}, ignores.Instance.Gcp.Labels)

	assert.Equal(t, []string{"awsName"}, ignores.Instance.Aws.Names)
	assert.Equal(t, []string{"azureName"}, ignores.Instance.Azure.Names)
	assert.Equal(t, []string{"gcpName"}, ignores.Instance.Gcp.Names)

	assert.Equal(t, []string{"awsOwner"}, ignores.Instance.Aws.Owners)
	assert.Equal(t, []string{"azureOwner"}, ignores.Instance.Azure.Owners)
	assert.Equal(t, []string{"gcpOwner"}, ignores.Instance.Gcp.Owners)
}

func TestSplitListToMap(t *testing.T) {
	assert.Equal(t, map[string]bool{"a": true, "b": true, "A": true, "B": true}, SplitListToMap("a, b"))
}

func TestSplitListToMapEmpty(t *testing.T) {
	assert.Equal(t, map[string]bool{}, SplitListToMap(""))
}
