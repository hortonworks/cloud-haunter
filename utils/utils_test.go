package utils

import (
	"testing"
	"time"

	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/stretchr/testify/assert"
)

func TestIsMatchFound(t *testing.T) {
	tags := map[string]string{"firsttag": "", "testtag": ""}
	assert.True(t, IsAnyMatch(tags, "testtag"))
}

func TestIsMatchNotFound(t *testing.T) {
	tags := map[string]string{"firsttag": "", "sectag": ""}
	assert.False(t, IsAnyMatch(tags, "testtag"))
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
