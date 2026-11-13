package gcp

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/bigquery/v2"
	"google.golang.org/api/option"

	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

const (
	billingExportTableEnv = "GCP_BILLING_EXPORT_TABLE"
	bigQueryTimeoutMs     = 60000
	bigQueryTimestamp     = "2006-01-02 15:04:05"

	ignoredCostByOwnerQuery = "SELECT\n" +
		"  (SELECT value FROM UNNEST(labels) WHERE key = @owner_label LIMIT 1) AS owner,\n" +
		"  SUM(cost) + SUM(IFNULL((SELECT SUM(credit.amount) FROM UNNEST(credits) AS credit), 0)) AS spend\n" +
		"FROM `%s`\n" +
		"WHERE DATE(_PARTITIONTIME) >= DATE(@since)\n" +
		"  AND usage_start_time >= @since\n" +
		"  AND usage_start_time < @until\n" +
		"  AND project.id = @project\n" +
		"  AND EXISTS(SELECT 1 FROM UNNEST(labels) WHERE key = @ignore_label)\n" +
		"GROUP BY owner"
)

var _ types.CostProvider = gcpProvider{}

var billingExportTablePattern = regexp.MustCompile(`^[a-z0-9.:-]+\.\w+\.\w+$`)

// GetIgnoredCosts returns the spend of the ignore-labelled resources of the project per owner label,
// read from the Cloud Billing export to BigQuery. The export has no per-resource cost for most services.
func (p gcpProvider) GetIgnoredCosts(since, until time.Time) (*types.IgnoredCosts, error) {
	if !billingExportTablePattern.MatchString(p.billingExportTable) {
		return nil, fmt.Errorf("[GCP] %s must name the billing export table as project.dataset.table, got %q", billingExportTableEnv, p.billingExportTable)
	}
	httpClient, err := google.DefaultClient(context.Background(), bigquery.CloudPlatformReadOnlyScope)
	if err != nil {
		return nil, fmt.Errorf("[GCP] Failed to authenticate to BigQuery: %w", err)
	}
	service, err := bigquery.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("[GCP] Failed to create the BigQuery client: %w", err)
	}
	return getIgnoredCosts(service, p.projectID, p.billingExportTable, p.cfg.IgnoreLabel, p.cfg.OwnerLabel, since, until)
}

func getIgnoredCosts(service *bigquery.Service, projectID, billingExportTable, ignoreLabel, ownerLabel string, since, until time.Time) (*types.IgnoredCosts, error) {
	rows, err := runQuery(service, projectID, &bigquery.QueryRequest{
		Query:         fmt.Sprintf(ignoredCostByOwnerQuery, billingExportTable),
		UseLegacySql:  new(bool),
		ParameterMode: "NAMED",
		QueryParameters: []*bigquery.QueryParameter{
			queryParameter("owner_label", "STRING", ownerLabel),
			queryParameter("ignore_label", "STRING", ignoreLabel),
			queryParameter("project", "STRING", projectID),
			queryParameter("since", "TIMESTAMP", since.UTC().Format(bigQueryTimestamp)),
			queryParameter("until", "TIMESTAMP", until.UTC().Format(bigQueryTimestamp)),
		},
		TimeoutMs: bigQueryTimeoutMs,
	})
	if err != nil {
		return nil, fmt.Errorf("[GCP] Failed to query the billing export: %w", err)
	}

	costs := &types.IgnoredCosts{ByOwner: map[string]float64{}, ByResource: map[string]float64{}}
	for _, row := range rows {
		owner, _ := row.F[0].V.(string)
		spendValue, _ := row.F[1].V.(string)
		spend, err := strconv.ParseFloat(spendValue, 64)
		if err != nil {
			log.Warnf("[GCP] Unparsable spend %q of owner %q, counted as 0", spendValue, owner)
		}
		if owner != "" {
			costs.ByOwner[owner] += spend
		}
	}
	return costs, nil
}

func runQuery(service *bigquery.Service, projectID string, request *bigquery.QueryRequest) ([]*bigquery.TableRow, error) {
	response, err := service.Jobs.Query(projectID, request).Do()
	if err != nil {
		return nil, err
	}
	rows := response.Rows
	complete, pageToken := response.JobComplete, response.PageToken
	for !complete || pageToken != "" {
		results, err := service.Jobs.GetQueryResults(projectID, response.JobReference.JobId).
			Location(response.JobReference.Location).
			PageToken(pageToken).
			TimeoutMs(bigQueryTimeoutMs).
			Do()
		if err != nil {
			return nil, err
		}
		if results.JobComplete {
			rows = append(rows, results.Rows...)
		}
		complete, pageToken = results.JobComplete, results.PageToken
	}
	return rows, nil
}

func queryParameter(name, parameterType, value string) *bigquery.QueryParameter {
	return &bigquery.QueryParameter{
		Name:           name,
		ParameterType:  &bigquery.QueryParameterType{Type: parameterType},
		ParameterValue: &bigquery.QueryParameterValue{Value: value},
	}
}
