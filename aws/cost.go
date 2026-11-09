package aws

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/costexplorer"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

const (
	costMetric             = "UnblendedCost"
	costExplorerDateLayout = "2006-01-02"
	ec2ComputeService      = "Amazon Elastic Compute Cloud - Compute"
	// GetCostAndUsageWithResources only supports the last 14 days. Owner totals still use the full budget period.
	maxResourceLevelCostHistory = 14 * 24 * time.Hour
	tagGroupKeySeparator        = "$"
)

var _ types.CostProvider = awsProvider{}

type costExplorerClient interface {
	GetCostAndUsage(*costexplorer.GetCostAndUsageInput) (*costexplorer.GetCostAndUsageOutput, error)
	GetCostAndUsageWithResources(*costexplorer.GetCostAndUsageWithResourcesInput) (*costexplorer.GetCostAndUsageWithResourcesOutput, error)
}

// GetIgnoredCosts returns the spend of the ignore-labelled resources per owner label, and the spend of
// every EC2 instance, the only resource type Cost Explorer reports per resource
func (p awsProvider) GetIgnoredCosts(since, until time.Time) (*types.IgnoredCosts, error) {
	if p.govCloud {
		return nil, errors.New("[AWS_GOV] Cost Explorer is not available in GovCloud")
	}
	client, err := newCostExplorerClient()
	if err != nil {
		return nil, fmt.Errorf("[AWS] Failed to create the Cost Explorer client: %w", err)
	}
	return getIgnoredCosts(client, p.cfg.IgnoreLabel, p.cfg.OwnerLabel, since, until)
}

func newCostExplorerClient() (*costexplorer.CostExplorer, error) {
	awsSession, err := newSession(func(config *aws.Config) {
		config.Region = aws.String(DEFAULT_REGION)
	})
	if err != nil {
		return nil, err
	}
	return costexplorer.New(awsSession), nil
}

func getIgnoredCosts(client costExplorerClient, ignoreLabel, ownerLabel string, since, until time.Time) (*types.IgnoredCosts, error) {
	byOwner, err := getIgnoredCostsByOwner(client, ignoreLabel, ownerLabel, costPeriod(since, until))
	if err != nil {
		return nil, fmt.Errorf("[AWS] Failed to fetch the ignored costs: %w", err)
	}
	byResource, err := getInstanceCosts(client, costPeriod(latest(since, until.Add(-maxResourceLevelCostHistory)), until))
	if err != nil {
		log.Warnf("[AWS] Failed to fetch the cost of the instances, they are listed without cost, err: %s", err.Error())
		byResource = map[string]float64{}
	}
	return &types.IgnoredCosts{ByOwner: byOwner, ByResource: byResource}, nil
}

func getIgnoredCostsByOwner(client costExplorerClient, ignoreLabel, ownerLabel string, period *costexplorer.DateInterval) (map[string]float64, error) {
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod:  period,
		Granularity: aws.String(costexplorer.GranularityMonthly),
		Metrics:     aws.StringSlice([]string{costMetric}),
		GroupBy: []*costexplorer.GroupDefinition{
			{Type: aws.String(costexplorer.GroupDefinitionTypeTag), Key: aws.String(ignoreLabel)},
			{Type: aws.String(costexplorer.GroupDefinitionTypeTag), Key: aws.String(ownerLabel)},
		},
	}
	byOwner := map[string]float64{}
	for {
		output, err := client.GetCostAndUsage(input)
		if err != nil {
			return nil, err
		}
		for _, group := range groupsOf(output.ResultsByTime) {
			if len(group.Keys) != 2 || tagValue(group.Keys[0], ignoreLabel) == "" {
				continue
			}
			if owner := tagValue(group.Keys[1], ownerLabel); owner != "" {
				byOwner[owner] += amountOf(group)
			}
		}
		if aws.StringValue(output.NextPageToken) == "" {
			return byOwner, nil
		}
		input.NextPageToken = output.NextPageToken
	}
}

func getInstanceCosts(client costExplorerClient, period *costexplorer.DateInterval) (map[string]float64, error) {
	input := &costexplorer.GetCostAndUsageWithResourcesInput{
		TimePeriod:  period,
		Granularity: aws.String(costexplorer.GranularityMonthly),
		Metrics:     aws.StringSlice([]string{costMetric}),
		Filter: &costexplorer.Expression{Dimensions: &costexplorer.DimensionValues{
			Key:    aws.String(costexplorer.DimensionService),
			Values: aws.StringSlice([]string{ec2ComputeService}),
		}},
		GroupBy: []*costexplorer.GroupDefinition{
			{Type: aws.String(costexplorer.GroupDefinitionTypeDimension), Key: aws.String(costexplorer.DimensionResourceId)},
		},
	}
	byResource := map[string]float64{}
	for {
		output, err := client.GetCostAndUsageWithResources(input)
		if err != nil {
			return nil, err
		}
		for _, group := range groupsOf(output.ResultsByTime) {
			if len(group.Keys) == 1 {
				byResource[strings.ToLower(aws.StringValue(group.Keys[0]))] += amountOf(group)
			}
		}
		if aws.StringValue(output.NextPageToken) == "" {
			return byResource, nil
		}
		input.NextPageToken = output.NextPageToken
	}
}

func costPeriod(since, until time.Time) *costexplorer.DateInterval {
	return &costexplorer.DateInterval{
		Start: aws.String(since.UTC().Format(costExplorerDateLayout)),
		End:   aws.String(until.UTC().Format(costExplorerDateLayout)),
	}
}

func latest(first, second time.Time) time.Time {
	if first.After(second) {
		return first
	}
	return second
}

func groupsOf(results []*costexplorer.ResultByTime) []*costexplorer.Group {
	var groups []*costexplorer.Group
	for _, result := range results {
		groups = append(groups, result.Groups...)
	}
	return groups
}

// tagValue reads a tag group key, which Cost Explorer returns as "<tag>$<value>" with an empty value for untagged costs
func tagValue(groupKey *string, tag string) string {
	return strings.TrimPrefix(aws.StringValue(groupKey), tag+tagGroupKeySeparator)
}

func amountOf(group *costexplorer.Group) float64 {
	metric, found := group.Metrics[costMetric]
	if !found {
		return 0
	}
	amount, err := strconv.ParseFloat(aws.StringValue(metric.Amount), 64)
	if err != nil {
		log.Warnf("[AWS] Unparsable cost amount %q, counted as 0", aws.StringValue(metric.Amount))
		return 0
	}
	return amount
}
