package azure

import (
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2017-12-01/compute"
	"github.com/Azure/go-autorest/autorest"
	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"

	"github.com/stretchr/testify/assert"
)

type callInfo struct {
	invocations []interface{}
}

func TestProviderInit(t *testing.T) {
	provider := azureProvider{}

	authorizer := func() (autorest.Authorizer, error) {
		return autorest.NullAuthorizer{}, nil
	}

	provider.init("AZURE_SUBSCRIPTION_ID", authorizer)

	assert.Equal(t, "AZURE_SUBSCRIPTION_ID", provider.subscriptionID)
	assert.NotNil(t, provider.vmClient)
	assert.NotNil(t, provider.vmClient.Authorizer)
}

func TestConvertVmsToInstances(t *testing.T) {
	instances, _ := convertVmsToInstances(mockHasValues{})

	assert.Equal(t, 1, len(instances))
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
	tags := types.Tags{ctx.CreationTimeLabel: testValues.timeAsUnixTimeStamp}
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
	instanceName := "testInstance"
	instanceID := "id-1234"
	instanceLocation := "testLocation"
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

type mockHasValues struct {
}

func (m mockHasValues) Values() []compute.VirtualMachine {
	return []compute.VirtualMachine{*getInstance()}
}
