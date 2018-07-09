package azure

import (
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2017-12-01/compute"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"

	"github.com/stretchr/testify/assert"
)

type callInfo struct {
	invocations []interface{}
}

func Test_givenGetCreationTimeFromTagsReturnsCreationTime_whenNewInstance_thenCreatedWillContainCreationTime(t *testing.T) {
	expectedTime := time.Date(2018, 5, 25, 11, 23, 23, 0, time.Local)
	callInfo, stubGetCreationTimeFromTagsFunc := getStubGetCreationTimeFromTags(expectedTime)
	tags := types.Tags{}
	stubConvertTagsFunc := func(map[string]*string) types.Tags {
		return tags
	}

	instance := newInstance(*getInstance(), stubGetCreationTimeFromTagsFunc, stubConvertTagsFunc)

	assert.Equal(t, instance.Created, expectedTime)
	assert.Equal(t, len(callInfo.invocations), 1)
	assert.Equal(t, callInfo.invocations[0].(types.Tags), tags)
}

func Test_givenTimestampIsInTags_whenGetCreationTimeFromTags_thenReturnsConvertedTimestamp(t *testing.T) {
	testValues := struct {
		timeAsUnixTimeStamp string
		timeAsTime          time.Time
	}{
		timeAsUnixTimeStamp: "1527240203",
		timeAsTime:          time.Date(2018, 5, 25, 11, 23, 23, 0, time.Local),
	}
	tags := types.Tags{context.CreationTimeLabel: testValues.timeAsUnixTimeStamp}
	callInfo, stubConverterFunc := getStubConvertTimeUnixByTime(testValues.timeAsTime)

	getCreationTimeFromTags(tags, stubConverterFunc)

	assert.Equal(t, len(callInfo.invocations), 1)
	assert.Equal(t, callInfo.invocations[0].(string), testValues.timeAsUnixTimeStamp)
}

func Test_givenTimestampNotInTags_whenGetCreationTimeFromTags_thenReturnsEpochZeroTime(t *testing.T) {
	callInfo, stubConverterFunc := getStubConvertTimeUnixEpochZero()

	getCreationTimeFromTags(types.Tags{}, stubConverterFunc)

	assert.Equal(t, len(callInfo.invocations), 1)
	assert.Equal(t, callInfo.invocations[0].(string), "0")
}

func getInstance() *compute.VirtualMachine {
	var instanceName = "testInstance"
	var instanceID = "id-1234"
	var instanceLocation = "testLocation"
	return &compute.VirtualMachine{Name: &instanceName, ID: &instanceID, Location: &instanceLocation}
}

func getStubGetCreationTimeFromTags(expectedTime time.Time) (*callInfo, getCreationTimeFromTagsFuncSignature) {
	callInfo := callInfo{invocations: make([]interface{}, 0, 3)}
	return &callInfo, func(tags types.Tags, timeConverterFunc func(string) time.Time) time.Time {
		callInfo.invocations = append(callInfo.invocations, tags)
		return expectedTime
	}
}

func getStubConvertTimeUnixByTime(timeAsTime time.Time) (*callInfo, func(string) time.Time) {
	callInfo := callInfo{invocations: make([]interface{}, 0, 3)}
	return &callInfo, func(unixTimestamp string) time.Time {
		callInfo.invocations = append(callInfo.invocations, unixTimestamp)
		return timeAsTime
	}
}

func getStubConvertTimeUnixEpochZero() (*callInfo, func(string) time.Time) {
	callInfo := callInfo{invocations: make([]interface{}, 0, 3)}
	return &callInfo, func(unixTimestamp string) time.Time {
		callInfo.invocations = append(callInfo.invocations, unixTimestamp)
		return time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local)
	}
}
