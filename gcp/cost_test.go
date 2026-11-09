package gcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/bigquery/v2"
	"google.golang.org/api/option"
)

type fakeBigQuery struct {
	mu        sync.Mutex
	responses []any
	requests  []*http.Request
	queries   []bigquery.QueryRequest
}

func (f *fakeBigQuery) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if request.Method == http.MethodPost {
		var query bigquery.QueryRequest
		_ = json.NewDecoder(request.Body).Decode(&query)
		f.queries = append(f.queries, query)
	}
	f.requests = append(f.requests, request)
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(f.responses[len(f.requests)-1])
}

func (f *fakeBigQuery) service(t *testing.T) *bigquery.Service {
	t.Helper()
	server := httptest.NewServer(f)
	t.Cleanup(server.Close)
	service, err := bigquery.NewService(context.Background(), option.WithEndpoint(server.URL), option.WithoutAuthentication())
	require.NoError(t, err)
	return service
}

func ownerSpendRow(owner any, spend string) *bigquery.TableRow {
	return &bigquery.TableRow{F: []*bigquery.TableCell{{V: owner}, {V: spend}}}
}

func TestGetIgnoredCostsSumsSpendPerOwnerFromTheBillingExport(t *testing.T) {
	jobReference := &bigquery.JobReference{JobId: "job", Location: "US"}
	fake := &fakeBigQuery{responses: []any{
		bigquery.QueryResponse{JobComplete: false, JobReference: jobReference},
		bigquery.GetQueryResultsResponse{JobComplete: true, JobReference: jobReference, PageToken: "second",
			Rows: []*bigquery.TableRow{ownerSpendRow("someone", "120.5"), ownerSpendRow(nil, "40")}},
		bigquery.GetQueryResultsResponse{JobComplete: true, JobReference: jobReference,
			Rows: []*bigquery.TableRow{ownerSpendRow("other", "3.25")}},
	}}
	since := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)

	costs, err := getIgnoredCosts(fake.service(t), "project", "billing-project.export.table", "ignore", "owner", since, until)

	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"someone": 120.5, "other": 3.25}, costs.ByOwner)
	assert.Empty(t, costs.ByResource)
	require.Len(t, fake.queries, 1)
	query := fake.queries[0]
	assert.Contains(t, query.Query, "FROM `billing-project.export.table`")
	assert.False(t, *query.UseLegacySql)
	parameters := map[string]string{}
	for _, parameter := range query.QueryParameters {
		parameters[parameter.Name] = parameter.ParameterValue.Value
	}
	assert.Equal(t, map[string]string{"owner_label": "owner", "ignore_label": "ignore", "project": "project",
		"since": "2026-11-02 00:00:00", "until": "2026-11-09 00:00:00"}, parameters)
	assert.Equal(t, "second", fake.requests[2].URL.Query().Get("pageToken"))
	assert.True(t, strings.HasSuffix(fake.requests[1].URL.Path, "/queries/job"))
	assert.Equal(t, "US", fake.requests[1].URL.Query().Get("location"))
}

func TestGetIgnoredCostsRejectsAnInvalidBillingExportTable(t *testing.T) {
	for _, table := range []string{"", "dataset.table", "project.dataset.table`; DROP", "project.data-set.table"} {
		provider := gcpProvider{cfg: &config.Config{}, billingExportTable: table}

		_, err := provider.GetIgnoredCosts(time.Now(), time.Now())

		assert.ErrorContains(t, err, billingExportTableEnv, table)
	}
}
