package azure

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"

	"github.com/hortonworks/cloud-haunter/types"

	"github.com/stretchr/testify/assert"
)

type callInfo struct {
	invocations []interface{}
}

func TestProviderInit(t *testing.T) {
	provider := azureProvider{}

	credentialProvider := func(*azidentity.EnvironmentCredentialOptions) (*azidentity.EnvironmentCredential, error) {
		return &azidentity.EnvironmentCredential{}, nil
	}

	provider.init("AZURE_SUBSCRIPTION_ID", credentialProvider)

	assert.Equal(t, "AZURE_SUBSCRIPTION_ID", provider.subscriptionID)
	assert.NotNil(t, provider.vmClient)
}

func Test_givenTimestampIsInTags_whenGetCreationTimeFromTags_thenReturnsConvertedTimestamp(t *testing.T) {
	testValues := struct {
		timeAsUnixTimeStamp string
		timeAsTime          time.Time
	}{
		timeAsUnixTimeStamp: "1527240203",
		timeAsTime:          time.Date(2018, 5, 25, 11, 23, 23, 0, time.Local),
	}
	tags := types.Tags{"creation-timestamp": testValues.timeAsUnixTimeStamp}
	callInfo, stubConverterFunc := getStubConvertTimeUnixByTime(testValues.timeAsTime)

	getCreationTimeFromTags(tags, stubConverterFunc)

	assert.Equal(t, len(callInfo.invocations), 1)
	assert.Equal(t, callInfo.invocations[0].(string), testValues.timeAsUnixTimeStamp)
}

func Test_givenTimestampIsInLegacyTags_whenGetCreationTimeFromTags_thenReturnsConvertedTimestamp(t *testing.T) {
	testValues := struct {
		timeAsUnixTimeStamp string
		timeAsTime          time.Time
	}{
		timeAsUnixTimeStamp: "1527240203",
		timeAsTime:          time.Date(2018, 5, 25, 11, 23, 23, 0, time.Local),
	}
	tags := types.Tags{"cb-creation-timestamp": testValues.timeAsUnixTimeStamp}
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

func TestGetResourceGroupNameByURL(t *testing.T) {
	resourceGroupName, name := getResourceGroupName("https://<rg_name>.blob.core.windows.net/images/<inst_name>")

	assert.Equal(t, "<rg_name>", resourceGroupName)
	assert.Equal(t, "<inst_name>", name)
}

func TestGetResourceGroupName(t *testing.T) {
	resourceGroupName, name := getResourceGroupName("/subscriptions/<sub_id>/resourceGroups/<rg_name>/providers/Microsoft.Compute/virtualMachines/<inst_name>")

	assert.Equal(t, "<rg_name>", resourceGroupName)
	assert.Equal(t, "<inst_name>", name)
}

func TestGetResourceGroupNameNotFound(t *testing.T) {
	resourceGroupName, name := getResourceGroupName("")

	assert.Equal(t, "", resourceGroupName)
	assert.Equal(t, "", name)
}

func TestGetImageResourceGroupAndName(t *testing.T) {
	resourceGroupName, name := getImageResourceGroupAndName("https://hwxaustraliaeast.blob.core.windows.net/images/cb-hdp--1801311614.vhd")

	assert.Equal(t, "hwxaustraliaeast", resourceGroupName)
	assert.Equal(t, "cb-hdp--1801311614.vhd", name)
}

var deleteRgChan = make(chan string)
var deleteNameChan = make(chan string)

func (ic imagesClient) BeginDelete(ctx context.Context, resourceGroupName string, imageName string, options *armcompute.ImagesClientBeginDeleteOptions) (*runtime.Poller[armcompute.ImagesClientDeleteResponse], error) {
	deleteRgChan <- resourceGroupName
	deleteNameChan <- imageName
	return nil, nil
}

func TestDeleteImages(t *testing.T) {
	imagesToDelete := []azureImage{
		{&types.Image{CloudType: types.AZURE, ID: "https://hwxaustraliaeast.blob.core.windows.net/images/cb-hdp--1801311614.vhd", Region: "Australia East"}, "hwxaustraliaeast", "cb-hdp--1801311614.vhd"},
	}
	existingImages := []armcompute.Image{
		{
			ID:       &(&types.S{S: "/subscriptions/<sub_id>/resourceGroups/hwxaustraliaeast/providers/Microsoft.Compute/Image/cb-hdp--1801311614.vhd"}).S,
			Name:     &(&types.S{S: "different name"}).S,
			Location: &(&types.S{S: "Australia East"}).S,
		},
		{
			ID:       &(&types.S{S: "/subscriptions/<sub_id>/resourceGroups/different_resourcegroup/providers/Microsoft.Compute/Image/cb-hdp--1801311614.vhd"}).S,
			Name:     &(&types.S{S: "cb-hdp--1801311614.vhd"}).S,
			Location: &(&types.S{S: "Australia East"}).S,
		},
		{
			ID:       &(&types.S{S: "/subscriptions/<sub_id>/resourceGroups/hwxaustraliaeast/providers/Microsoft.Compute/Image/cb-hdp--1801311614.vhd"}).S,
			Name:     &(&types.S{S: "cb-hdp--1801311614.vhd"}).S,
			Location: &(&types.S{S: "different location"}).S,
		},
		{
			ID:       &(&types.S{S: "/subscriptions/<sub_id>/resourceGroups/hwxaustraliaeast/providers/Microsoft.Compute/Image/cb-hdp--1801311614.vhd"}).S,
			Name:     &(&types.S{S: "cb-hdp--1801311614.vhd"}).S,
			Location: &(&types.S{S: "Australia East"}).S,
		},
	}

	go func() {
		defer close(deleteRgChan)
		defer close(deleteNameChan)

		deleteImages(imagesClient{}, imagesToDelete, existingImages, false)
	}()

	assert.Equal(t, "hwxaustraliaeast", <-deleteRgChan)
	assert.Equal(t, "cb-hdp--1801311614.vhd", <-deleteNameChan)
}

// deallocateRecorder records the deallocate calls of the fake VM clients. Calls arrive from
// several goroutines, hence the mutex.
type deallocateRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *deallocateRecorder) record(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

type fakeVMDeallocateClient struct{ recorder *deallocateRecorder }

func (c fakeVMDeallocateClient) BeginDeallocate(_ context.Context, resourceGroupName string, vmName string, _ *armcompute.VirtualMachinesClientBeginDeallocateOptions) (*runtime.Poller[armcompute.VirtualMachinesClientDeallocateResponse], error) {
	c.recorder.record("vm:" + resourceGroupName + "/" + vmName)
	return nil, nil
}

type fakeScaleSetVMDeallocateClient struct{ recorder *deallocateRecorder }

func (c fakeScaleSetVMDeallocateClient) BeginDeallocate(_ context.Context, resourceGroupName string, vmScaleSetName string, instanceID string, _ *armcompute.VirtualMachineScaleSetVMsClientBeginDeallocateOptions) (*runtime.Poller[armcompute.VirtualMachineScaleSetVMsClientDeallocateResponse], error) {
	c.recorder.record("scaleSetVM:" + resourceGroupName + "/" + vmScaleSetName + "/" + instanceID)
	return nil, nil
}

func TestStopInstances(t *testing.T) {
	cases := []struct {
		name     string
		dryRun   bool
		expected []string
	}{
		{"deallocates standalone and scale-set VMs", false, []string{"scaleSetVM:rg/scale-set/0", "vm:rg/standalone"}},
		{"dry-run sends no request", true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &deallocateRecorder{}
			instances := []*types.Instance{
				{CloudType: types.AZURE, Name: "standalone", Metadata: map[string]string{ResourceGroupName: "rg"}},
				{CloudType: types.AZURE, Name: "scale-set_0", Metadata: map[string]string{ResourceGroupName: "rg", ScaleSetName: "scale-set"}},
			}

			errs := stopInstances(fakeVMDeallocateClient{recorder}, fakeScaleSetVMDeallocateClient{recorder}, instances, tc.dryRun)

			assert.Empty(t, errs)
			sort.Strings(recorder.calls)
			assert.Equal(t, tc.expected, recorder.calls)
		})
	}
}

func getStubConvertTimeUnixByTime(timeAsTime time.Time) (*callInfo, func(string) time.Time) {
	cInfo := callInfo{invocations: make([]interface{}, 0, 3)}
	return &cInfo, func(unixTimestamp string) time.Time {
		cInfo.invocations = append(cInfo.invocations, unixTimestamp)
		return timeAsTime
	}
}

func getStubConvertTimeUnixEpochZero() (*callInfo, func(string) time.Time) {
	cInfo := callInfo{invocations: make([]interface{}, 0, 3)}
	return &cInfo, func(unixTimestamp string) time.Time {
		cInfo.invocations = append(cInfo.invocations, unixTimestamp)
		return time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local)
	}
}
