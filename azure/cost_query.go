package azure

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement/v2"
)

// costQueryClient adds paging to the Cost Management query API, which the SDK's QueryClient does not expose.
// It uses the ARM pipeline so that authentication and throttling retries also apply to subsequent pages.
type costQueryClient struct {
	client *arm.Client
}

func newCostQueryClient(credential azcore.TokenCredential, options *arm.ClientOptions) (*costQueryClient, error) {
	client, err := arm.NewClient("github.com/hortonworks/cloud-haunter/azure", "v0.0.0", credential, options)
	if err != nil {
		return nil, err
	}
	return &costQueryClient{client: client}, nil
}

func (c *costQueryClient) NewUsagePager(scope string, query armcostmanagement.QueryDefinition) *runtime.Pager[armcostmanagement.QueryResult] {
	firstURL := runtime.JoinPaths(c.client.Endpoint(), scope, "providers/Microsoft.CostManagement/query") + "?api-version=2022-10-01"
	createRequest := func(ctx context.Context, url string) (*policy.Request, error) {
		req, err := runtime.NewRequest(ctx, http.MethodPost, url)
		if err != nil {
			return nil, err
		}
		req.Raw().Header.Set("Accept", "application/json")
		if err := runtime.MarshalAsJSON(req, query); err != nil {
			return nil, err
		}
		return req, nil
	}
	return runtime.NewPager(runtime.PagingHandler[armcostmanagement.QueryResult]{
		More: func(page armcostmanagement.QueryResult) bool {
			return page.Properties != nil && page.Properties.NextLink != nil && *page.Properties.NextLink != ""
		},
		Fetcher: func(ctx context.Context, page *armcostmanagement.QueryResult) (armcostmanagement.QueryResult, error) {
			nextLink := ""
			if page != nil && page.Properties != nil && page.Properties.NextLink != nil {
				nextLink = *page.Properties.NextLink
			}
			// Continuations are POST requests with the same query body, not the usual ARM list GET.
			response, err := runtime.FetcherForNextLink(ctx, c.client.Pipeline(), nextLink, func(ctx context.Context) (*policy.Request, error) {
				return createRequest(ctx, firstURL)
			}, &runtime.FetcherForNextLinkOptions{NextReq: createRequest, StatusCodes: []int{http.StatusNoContent}})
			if err != nil {
				return armcostmanagement.QueryResult{}, err
			}
			var result armcostmanagement.QueryResult
			if err := runtime.UnmarshalAsJSON(response, &result); err != nil {
				return armcostmanagement.QueryResult{}, err
			}
			return result, nil
		},
		Tracer: c.client.Tracer(),
	})
}
