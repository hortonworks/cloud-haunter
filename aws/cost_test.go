package aws

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/costexplorer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCostExplorer struct {
	ownerPages    []*costexplorer.GetCostAndUsageOutput
	resourcePages []*costexplorer.GetCostAndUsageWithResourcesOutput
	resourceErr   error

	ownerInputs    []costexplorer.GetCostAndUsageInput
	resourceInputs []costexplorer.GetCostAndUsageWithResourcesInput
}

func (m *mockCostExplorer) GetCostAndUsage(input *costexplorer.GetCostAndUsageInput) (*costexplorer.GetCostAndUsageOutput, error) {
	m.ownerInputs = append(m.ownerInputs, *input)
	return m.ownerPages[len(m.ownerInputs)-1], nil
}

func (m *mockCostExplorer) GetCostAndUsageWithResources(input *costexplorer.GetCostAndUsageWithResourcesInput) (*costexplorer.GetCostAndUsageWithResourcesOutput, error) {
	m.resourceInputs = append(m.resourceInputs, *input)
	if m.resourceErr != nil {
		return nil, m.resourceErr
	}
	return m.resourcePages[len(m.resourceInputs)-1], nil
}

func costGroup(amount string, keys ...string) *costexplorer.Group {
	return &costexplorer.Group{
		Keys:    aws.StringSlice(keys),
		Metrics: map[string]*costexplorer.MetricValue{costMetric: {Amount: aws.String(amount), Unit: aws.String("USD")}},
	}
}

func costResults(groups ...*costexplorer.Group) []*costexplorer.ResultByTime {
	return []*costexplorer.ResultByTime{{Groups: groups}}
}

var (
	testSince = time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	testUntil = time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
)

func TestGetIgnoredCostsSumsTaggedSpendPerOwnerAcrossPages(t *testing.T) {
	explorer := &mockCostExplorer{
		ownerPages: []*costexplorer.GetCostAndUsageOutput{
			{
				ResultsByTime: costResults(
					costGroup("10.5", "ignore$true", "owner$someone"),
					costGroup("99", "ignore$", "owner$someone"),
					costGroup("7", "ignore$true", "owner$"),
				),
				NextPageToken: aws.String("next"),
			},
			{ResultsByTime: costResults(costGroup("1.5", "ignore$yes", "owner$someone"), costGroup("3", "ignore$true", "owner$other"))},
		},
		resourcePages: []*costexplorer.GetCostAndUsageWithResourcesOutput{
			{ResultsByTime: costResults(costGroup("4.25", "i-0ABC"), costGroup("not-a-number", "i-0def"))},
		},
	}

	costs, err := getIgnoredCosts(explorer, "ignore", "owner", testSince, testUntil)

	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"someone": 12, "other": 3}, costs.ByOwner)
	assert.Equal(t, map[string]float64{"i-0abc": 4.25, "i-0def": 0}, costs.ByResource)
	require.Len(t, explorer.ownerInputs, 2)
	assert.Nil(t, explorer.ownerInputs[0].NextPageToken)
	assert.Equal(t, "next", aws.StringValue(explorer.ownerInputs[1].NextPageToken))
	assert.Equal(t, "2026-11-02", aws.StringValue(explorer.ownerInputs[0].TimePeriod.Start))
	assert.Equal(t, "2026-11-09", aws.StringValue(explorer.ownerInputs[0].TimePeriod.End))
	assert.Equal(t, "ignore", aws.StringValue(explorer.ownerInputs[0].GroupBy[0].Key))
	assert.Equal(t, "owner", aws.StringValue(explorer.ownerInputs[0].GroupBy[1].Key))
}

func TestGetIgnoredCostsUsesTheBudgetPeriodWithinTheResourceLevelHistoryLimit(t *testing.T) {
	for _, tc := range []struct {
		name          string
		budgetPeriod  time.Duration
		ownerStart    string
		resourceStart string
	}{
		{name: "shorter than the limit", budgetPeriod: 7 * 24 * time.Hour, ownerStart: "2026-11-02", resourceStart: "2026-11-02"},
		{name: "at the limit", budgetPeriod: 14 * 24 * time.Hour, ownerStart: "2026-10-26", resourceStart: "2026-10-26"},
		{name: "longer than the limit", budgetPeriod: 30 * 24 * time.Hour, ownerStart: "2026-10-10", resourceStart: "2026-10-26"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			explorer := &mockCostExplorer{
				ownerPages:    []*costexplorer.GetCostAndUsageOutput{{}},
				resourcePages: []*costexplorer.GetCostAndUsageWithResourcesOutput{{}},
			}

			_, err := getIgnoredCosts(explorer, "ignore", "owner", testUntil.Add(-tc.budgetPeriod), testUntil)

			require.NoError(t, err)
			assert.Equal(t, tc.ownerStart, aws.StringValue(explorer.ownerInputs[0].TimePeriod.Start))
			assert.Equal(t, tc.resourceStart, aws.StringValue(explorer.resourceInputs[0].TimePeriod.Start))
			assert.Equal(t, "2026-11-09", aws.StringValue(explorer.ownerInputs[0].TimePeriod.End))
			assert.Equal(t, "2026-11-09", aws.StringValue(explorer.resourceInputs[0].TimePeriod.End))
			assert.Equal(t, ec2ComputeService, aws.StringValue(explorer.resourceInputs[0].Filter.Dimensions.Values[0]))
		})
	}
}

func TestGetIgnoredCostsKeepsOwnerSpendWhenInstanceCostsFail(t *testing.T) {
	explorer := &mockCostExplorer{
		ownerPages:  []*costexplorer.GetCostAndUsageOutput{{ResultsByTime: costResults(costGroup("150", "ignore$true", "owner$someone"))}},
		resourceErr: errors.New("resource level data is not enabled"),
	}

	costs, err := getIgnoredCosts(explorer, "ignore", "owner", testSince, testUntil)

	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"someone": 150}, costs.ByOwner)
	assert.Empty(t, costs.ByResource)
}

func TestGetIgnoredCostsIsNotSupportedOnGovCloud(t *testing.T) {
	_, err := awsProvider{govCloud: true}.GetIgnoredCosts(testSince, testUntil)

	assert.ErrorContains(t, err, "GovCloud")
}
