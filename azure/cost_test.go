package azure

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	resourcesfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/fake"
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSubscription = "subscription"
	teamVM           = "/subscriptions/subscription/resourceGroups/Team-RG/providers/Microsoft.Compute/virtualMachines/vm"
	teamDisk         = "/subscriptions/subscription/resourceGroups/Team-RG/providers/Microsoft.Compute/disks/disk"
	otherVM          = "/subscriptions/subscription/resourceGroups/other-rg/providers/Microsoft.Compute/virtualMachines/labelled"
)

type fakeAzure struct {
	resourceGroups []*armresources.ResourceGroup
	resources      []*armresources.GenericResourceExpanded
	costColumns    []*armcostmanagement.QueryColumn
	costRows       [][]any
	costPages      []*armcostmanagement.QueryProperties
	costErrors     map[int]int

	resourceFilter *string
	costScope      string
	costQueries    []armcostmanagement.QueryDefinition
	costRequests   []*http.Request
}

type costTransport func(*http.Request) (*http.Response, error)

func (f costTransport) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func fakeTransport(transport policy.Transporter) *arm.ClientOptions {
	return &arm.ClientOptions{ClientOptions: azcore.ClientOptions{Transport: transport}}
}

func (f *fakeAzure) provider(t *testing.T) azureProvider {
	t.Helper()
	credential := &azfake.TokenCredential{}
	groupServer := resourcesfake.ResourceGroupsServer{
		NewListPager: func(*armresources.ResourceGroupsClientListOptions) (resp azfake.PagerResponder[armresources.ResourceGroupsClientListResponse]) {
			resp.AddPage(http.StatusOK, armresources.ResourceGroupsClientListResponse{
				ResourceGroupListResult: armresources.ResourceGroupListResult{Value: f.resourceGroups}}, nil)
			return
		},
	}
	resourceServer := resourcesfake.Server{
		NewListPager: func(options *armresources.ClientListOptions) (resp azfake.PagerResponder[armresources.ClientListResponse]) {
			f.resourceFilter = options.Filter
			resp.AddPage(http.StatusOK, armresources.ClientListResponse{
				ResourceListResult: armresources.ResourceListResult{Value: f.resources}}, nil)
			return
		},
	}
	queryTransport := costTransport(func(req *http.Request) (*http.Response, error) {
		var query armcostmanagement.QueryDefinition
		require.NoError(t, json.NewDecoder(req.Body).Decode(&query))
		f.costScope = strings.TrimPrefix(strings.TrimSuffix(req.URL.Path, "/providers/Microsoft.CostManagement/query"), "/")
		f.costQueries = append(f.costQueries, query)
		f.costRequests = append(f.costRequests, req)
		page := len(f.costRequests) - 1
		properties := &armcostmanagement.QueryProperties{Columns: f.costColumns, Rows: f.costRows}
		if f.costPages != nil {
			require.Less(t, page, len(f.costPages))
			properties = f.costPages[page]
		}
		body, err := json.Marshal(armcostmanagement.QueryResult{Properties: properties})
		require.NoError(t, err)
		status := http.StatusOK
		if errorStatus, found := f.costErrors[page]; found {
			status = errorStatus
			body = []byte(`{"error":{"code":"BadRequest","message":"cost query failed"}}`)
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(bytes.NewReader(body)), Request: req}, nil
	})

	groupClient, err := armresources.NewResourceGroupsClient(testSubscription, credential, fakeTransport(resourcesfake.NewResourceGroupsServerTransport(&groupServer)))
	require.NoError(t, err)
	resourceClient, err := armresources.NewClient(testSubscription, credential, fakeTransport(resourcesfake.NewServerTransport(&resourceServer)))
	require.NoError(t, err)
	costOptions := fakeTransport(queryTransport)
	costOptions.Retry.MaxRetries = -1
	costClient, err := newCostQueryClient(credential, costOptions)
	require.NoError(t, err)
	return azureProvider{
		cfg:            &config.Config{IgnoreLabel: "ignore", OwnerLabel: "owner"},
		subscriptionID: testSubscription,
		rgClient:       groupClient,
		resourceClient: resourceClient,
		costClient:     costClient,
	}
}

func resourceGroup(name string, tags map[string]string) *armresources.ResourceGroup {
	return &armresources.ResourceGroup{Name: to.Ptr(name), Tags: toPtrMap(tags)}
}

func toPtrMap(values map[string]string) map[string]*string {
	pointers := map[string]*string{}
	for key, value := range values {
		pointers[key] = to.Ptr(value)
	}
	return pointers
}

func costColumns(names ...string) []*armcostmanagement.QueryColumn {
	columns := make([]*armcostmanagement.QueryColumn, 0, len(names))
	for _, name := range names {
		columns = append(columns, &armcostmanagement.QueryColumn{Name: to.Ptr(name)})
	}
	return columns
}

func filterValues(filter *armcostmanagement.QueryFilter) []string {
	values := make([]string, 0, len(filter.Dimensions.Values))
	for _, value := range filter.Dimensions.Values {
		values = append(values, *value)
	}
	return values
}

func TestGetIgnoredCostsAttributesSpendToResourceOrGroupOwner(t *testing.T) {
	azure := &fakeAzure{
		resourceGroups: []*armresources.ResourceGroup{
			resourceGroup("Team-RG", map[string]string{"ignore": "true", "owner": "group-owner"}),
			resourceGroup("other-rg", map[string]string{"owner": "someone"}),
		},
		resources:   []*armresources.GenericResourceExpanded{{ID: to.Ptr(teamVM)}, {ID: to.Ptr(otherVM)}},
		costColumns: costColumns("Cost", "ResourceId", "TagKey", "TagValue", "Currency"),
		costRows: [][]any{
			{10.5, "/subscriptions/subscription/resourcegroups/team-rg/providers/microsoft.compute/virtualmachines/vm", "owner", "vm-owner", "USD"},
			{4.0, "/subscriptions/subscription/resourcegroups/team-rg/providers/microsoft.compute/disks/disk", "", "", "USD"},
			{2.0, "/subscriptions/subscription/resourcegroups/other-rg/providers/microsoft.compute/virtualmachines/labelled", "", "", "USD"},
		},
	}
	since := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)

	costs, err := azure.provider(t).GetIgnoredCosts(since, until)

	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"vm-owner": 10.5, "group-owner": 4}, costs.ByOwner)
	assert.Equal(t, map[string]float64{
		"/subscriptions/subscription/resourcegroups/team-rg/providers/microsoft.compute/virtualmachines/vm":        10.5,
		"/subscriptions/subscription/resourcegroups/team-rg/providers/microsoft.compute/disks/disk":                4,
		"/subscriptions/subscription/resourcegroups/other-rg/providers/microsoft.compute/virtualmachines/labelled": 2,
	}, costs.ByResource)

	assert.Equal(t, "tagName eq 'ignore'", *azure.resourceFilter)
	assert.Equal(t, "subscriptions/subscription", azure.costScope)
	require.Len(t, azure.costQueries, 1)
	query := azure.costQueries[0]
	assert.Equal(t, since, query.TimePeriod.From.UTC())
	assert.Equal(t, time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC), query.TimePeriod.To.UTC())
	assert.Equal(t, "owner", *query.Dataset.Grouping[1].Name)
	require.Len(t, query.Dataset.Filter.Or, 2)
	assert.Equal(t, []string{"Team-RG"}, filterValues(query.Dataset.Filter.Or[0]))
	assert.Equal(t, []string{otherVM}, filterValues(query.Dataset.Filter.Or[1]))
}

func TestGetIgnoredCostsSumsAllPagesWithTheOriginalQuery(t *testing.T) {
	const queryURL = "https://management.azure.com/subscriptions/subscription/providers/Microsoft.CostManagement/query"
	firstNextLink := queryURL + "?api-version=2021-10-01&$skiptoken=first%2Btoken%3D"
	secondNextLink := queryURL + "?api-version=2021-10-01&$skiptoken=second%2Ftoken%3D"
	azure := &fakeAzure{
		resourceGroups: []*armresources.ResourceGroup{resourceGroup("Team-RG", map[string]string{"ignore": "true", "owner": "group-owner"})},
		costPages: []*armcostmanagement.QueryProperties{
			{Columns: costColumns("Cost", "ResourceId", "TagValue"), Rows: [][]any{{10.0, teamVM, "vm-owner"}}, NextLink: to.Ptr(firstNextLink)},
			{Columns: costColumns("ResourceId", "TagValue", "Cost"), Rows: [][]any{{strings.ToLower(teamVM), "vm-owner", 5.0}, {teamDisk, "", 4.0}}, NextLink: to.Ptr(secondNextLink)},
			{Columns: costColumns("TagValue", "Cost", "ResourceId"), Rows: [][]any{{"vm-owner", 1.5, teamVM}}, NextLink: to.Ptr("")},
		},
	}
	since := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)

	costs, err := azure.provider(t).GetIgnoredCosts(since, until)

	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"vm-owner": 16.5, "group-owner": 4}, costs.ByOwner)
	assert.Equal(t, map[string]float64{strings.ToLower(teamVM): 16.5, strings.ToLower(teamDisk): 4}, costs.ByResource)
	require.Len(t, azure.costRequests, 3)
	assert.Equal(t, "2022-10-01", azure.costRequests[0].URL.Query().Get("api-version"))
	assert.Empty(t, azure.costRequests[0].URL.Query().Get("$skiptoken"))
	assert.Equal(t, "2021-10-01", azure.costRequests[1].URL.Query().Get("api-version"))
	assert.Equal(t, "first+token=", azure.costRequests[1].URL.Query().Get("$skiptoken"))
	assert.Equal(t, "second/token=", azure.costRequests[2].URL.Query().Get("$skiptoken"))
	for index, request := range azure.costRequests {
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "/subscriptions/subscription/providers/Microsoft.CostManagement/query", request.URL.Path)
		assert.Equal(t, "application/json", request.Header.Get("Content-Type"))
		assert.Equal(t, azure.costQueries[0], azure.costQueries[index])
	}
}

func TestGetIgnoredCostsDoesNotReturnPartialTotalsWhenALaterPageFails(t *testing.T) {
	for _, tc := range []struct {
		name       string
		properties *armcostmanagement.QueryProperties
		status     int
		message    string
	}{
		{name: "query error", status: http.StatusBadRequest, message: "Failed to query the costs"},
		{name: "missing result", message: "returned no result"},
		{name: "missing columns", properties: &armcostmanagement.QueryProperties{Columns: costColumns("Cost")}, message: "misses one of the columns"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			azure := &fakeAzure{
				resourceGroups: []*armresources.ResourceGroup{resourceGroup("Team-RG", map[string]string{"ignore": "true"})},
				costPages: []*armcostmanagement.QueryProperties{
					{Columns: costColumns("Cost", "ResourceId", "TagValue"), Rows: [][]any{{10.0, teamVM, "vm-owner"}},
						NextLink: to.Ptr("https://management.azure.com/subscriptions/subscription/providers/Microsoft.CostManagement/query?api-version=2022-10-01&$skiptoken=next")},
					tc.properties,
				},
			}
			if tc.status != 0 {
				azure.costErrors = map[int]int{1: tc.status}
			}

			costs, err := azure.provider(t).GetIgnoredCosts(time.Now(), time.Now())

			assert.ErrorContains(t, err, tc.message)
			assert.Nil(t, costs)
			assert.Len(t, azure.costRequests, 2)
		})
	}
}

func TestGetIgnoredCostsSkipsTheQueryWhenNothingIsIgnored(t *testing.T) {
	azure := &fakeAzure{resourceGroups: []*armresources.ResourceGroup{resourceGroup("plain", map[string]string{"owner": "someone"})}}

	costs, err := azure.provider(t).GetIgnoredCosts(time.Now(), time.Now())

	require.NoError(t, err)
	assert.Empty(t, costs.ByOwner)
	assert.Empty(t, costs.ByResource)
	assert.Empty(t, azure.costQueries)
}

func TestGetIgnoredCostsRejectsAnUnexpectedResultShape(t *testing.T) {
	azure := &fakeAzure{
		resourceGroups: []*armresources.ResourceGroup{resourceGroup("ignored", map[string]string{"ignore": "true"})},
		costColumns:    costColumns("PreTaxCost", "ResourceId", "TagKey", "TagValue"),
	}

	_, err := azure.provider(t).GetIgnoredCosts(time.Now(), time.Now())

	assert.ErrorContains(t, err, "misses one of the columns")
}

func TestIgnoredCostFilterUsesOnlyTheNonEmptyDimensions(t *testing.T) {
	groups := ignoredResourceGroups{"rg": {name: "rg"}}

	assert.Nil(t, ignoredCostFilter(ignoredResourceGroups{}, nil))
	assert.Equal(t, "ResourceGroupName", *ignoredCostFilter(groups, nil).Dimensions.Name)
	assert.Equal(t, "ResourceId", *ignoredCostFilter(ignoredResourceGroups{}, []string{otherVM}).Dimensions.Name)
	assert.Len(t, ignoredCostFilter(groups, []string{otherVM}).Or, 2)
}

func TestMarkIgnoredResourceGroup(t *testing.T) {
	groups := ignoredResourceGroups{
		"team-rg":  {name: "Team-RG", owner: "group-owner"},
		"no-owner": {name: "no-owner"},
	}
	inTeamGroup := map[string]string{ResourceGroupName: "TEAM-RG"}
	inGroupWithoutOwner := map[string]string{ResourceGroupName: "no-owner"}
	inOtherGroup := map[string]string{ResourceGroupName: "other-rg"}

	for _, metadata := range []map[string]string{inTeamGroup, inGroupWithoutOwner, inOtherGroup} {
		markIgnoredResourceGroup(metadata, groups)
	}

	assert.Equal(t, map[string]string{ResourceGroupName: "TEAM-RG",
		types.ResourceGroupIgnoredMetadata: "true", types.ResourceGroupOwnerMetadata: "group-owner"}, inTeamGroup)
	assert.Equal(t, map[string]string{ResourceGroupName: "no-owner", types.ResourceGroupIgnoredMetadata: "true"}, inGroupWithoutOwner)
	assert.Equal(t, map[string]string{ResourceGroupName: "other-rg"}, inOtherGroup)
}

func TestResourceGroupOf(t *testing.T) {
	assert.Equal(t, "Team-RG", resourceGroupOf(teamDisk))
	assert.Equal(t, "", resourceGroupOf("not-an-id"))
}
