package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var epochZero = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

type timeTestValues struct {
	timeAsUnixTimeStamp string
	timeAsTime          time.Time
}

type CallInfo struct {
	invocations []string
}

func Test_givenNotAnIntUnixTimeStamp_whenConvertTimeUnix_thenReturnsEpochZeroTime(t *testing.T) {

	var unixTimestamp = "not an int number"
	convertedTime := ConvertTimeUnix(unixTimestamp)

	assert := assert.New(t)
	assert.Equal(convertedTime, epochZero.Local())
}
