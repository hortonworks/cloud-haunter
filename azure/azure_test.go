package azure

import (
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2017-12-01/compute"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/hortonworks/cloud-cost-reducer/utils"

	"github.com/stretchr/testify/assert"
)

type CallInfo struct {
	invocations []interface{}
}

type timeTestValues struct {
	timeAsUnixTimeStamp string
	timeAsTime          time.Time
}

var epochZero = time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local)

func getStubGetCreationTimeFromTags(expectedTime time.Time) (*CallInfo, getCreationTimeFromTagsFuncSignature) {
	callInfo := CallInfo{invocations: make([]interface{}, 0, 3)}
	return &callInfo, func(tags types.Tags, timeConverterFunc func(string) time.Time) time.Time {
		callInfo.invocations = append(callInfo.invocations, tags)
		return expectedTime
	}
}

func getStubConvertTags(tagsToReturn types.Tags) utils.ConvertTagsFuncSignature {
	return func(tagMap map[string]*string) types.Tags {
		return tagsToReturn
	}
}

func Test_givenGetCreationTimeFromTagsReturnsCreationTime_whenNewInstance_thenCreatedWillContainCreationTime(t *testing.T) {
	expectedTime := time.Date(2018, 5, 25, 11, 23, 23, 0, time.Local)
	callInfo, stubGetCreationTimeFromTagsFunc := getStubGetCreationTimeFromTags(expectedTime)
	tags := types.Tags{}
	stubConvertTagsFunc := getStubConvertTags(tags)

	instance := newInstance(*setupInstance(), stubGetCreationTimeFromTagsFunc, stubConvertTagsFunc)

	assert := assert.New(t)
	assert.Equal(instance.Created, expectedTime)
	assert.Equal(len(callInfo.invocations), 1)
	assert.Equal(types.Tags(callInfo.invocations[0].(types.Tags)), tags)
}

func setupInstance() *compute.VirtualMachine {
	var instanceName = "testInstance"
	var instanceID = "id-1234"
	return &compute.VirtualMachine{Name: &instanceName, ID: &instanceID}
}

func getStubConvertTimeUnix(testValues *timeTestValues) (*CallInfo, func(string) time.Time) {
	callInfo := CallInfo{invocations: make([]interface{}, 0, 3)}
	return &callInfo, func(unixTimestamp string) time.Time {
		callInfo.invocations = append(callInfo.invocations, unixTimestamp)
		if unixTimestamp == "0" {
			return epochZero
		}
		if testValues != nil && unixTimestamp == testValues.timeAsUnixTimeStamp {
			return testValues.timeAsTime
		}
		return time.Date(1990, 1, 3, 13, 14, 15, 0, time.Local)
	}
}

func Test_givenTimestampIsInTags_whenGetCreationTimeFromTags_thenReturnsConvertedTimestamp(t *testing.T) {
	testValues := timeTestValues{timeAsUnixTimeStamp: "1527240203", timeAsTime: time.Date(2018, 5, 25, 11, 23, 23, 0, time.Local)}
	tags := createTags(&testValues.timeAsUnixTimeStamp)
	callInfo, stubConverterFunc := getStubConvertTimeUnix(&testValues)

	time := getCreationTimeFromTags(tags, stubConverterFunc)

	assert := assert.New(t)
	assert.Equal(time, testValues.timeAsTime)
	assert.Equal(len(callInfo.invocations), 2)
	assert.Equal(string(callInfo.invocations[0].(string)), "0")
	assert.Equal(string(callInfo.invocations[1].(string)), testValues.timeAsUnixTimeStamp)
}

func Test_givenTimestampNotInTags_whenGetCreationTimeFromTags_thenReturnsEpochZeroTime(t *testing.T) {
	tags := createTags(nil)
	callInfo, stubConverterFunc := getStubConvertTimeUnix(nil)

	time := getCreationTimeFromTags(tags, stubConverterFunc)

	assert := assert.New(t)
	assert.Equal(time, epochZero)
	assert.Equal(len(callInfo.invocations), 1)
	assert.Equal(string(callInfo.invocations[0].(string)), "0")
}

func createTags(unixTimestamp *string) types.Tags {
	tags := make(types.Tags, 5)
	var creationTimeLabel = unixTimestamp
	if unixTimestamp != nil {
		tags[context.CreationTimeLabel] = *creationTimeLabel
	}
	return tags
}
