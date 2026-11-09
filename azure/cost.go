package azure

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
	log "github.com/sirupsen/logrus"
)

const (
	costColumn              = "Cost"
	resourceIDColumn        = "ResourceId"
	resourceGroupNameColumn = "ResourceGroupName"
	tagValueColumn          = "TagValue"
)

var _ types.CostProvider = azureProvider{}

var throttledCostQueryRetry = policy.RetryOptions{MaxRetries: 5, RetryDelay: 5 * time.Second, MaxRetryDelay: time.Minute}

type ignoredResourceGroup struct {
	name  string
	owner string
}

// ignoredResourceGroups is keyed by the lower-cased resource group name
type ignoredResourceGroups map[string]ignoredResourceGroup

func (p azureProvider) getIgnoredResourceGroups() (ignoredResourceGroups, error) {
	groups := ignoredResourceGroups{}
	pager := p.rgClient.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		for _, group := range page.Value {
			tags := utils.ConvertTags(group.Tags)
			if _, ignored := tags[p.cfg.IgnoreLabel]; ignored {
				groups[strings.ToLower(*group.Name)] = ignoredResourceGroup{name: *group.Name, owner: tags[p.cfg.OwnerLabel]}
			}
		}
	}
	return groups, nil
}

func (p azureProvider) getIgnoredResourceGroupsOrEmpty() ignoredResourceGroups {
	groups, err := p.getIgnoredResourceGroups()
	if err != nil {
		log.Errorf("[AZURE] Failed to fetch the resource groups, items do not inherit the ignore label of their resource group, err: %s", err.Error())
		return ignoredResourceGroups{}
	}
	return groups
}

func markIgnoredResourceGroup(metadata map[string]string, groups ignoredResourceGroups) {
	group, ignored := groups[strings.ToLower(metadata[ResourceGroupName])]
	if !ignored {
		return
	}
	metadata[types.ResourceGroupIgnoredMetadata] = "true"
	if group.owner != "" {
		metadata[types.ResourceGroupOwnerMetadata] = group.owner
	}
}

// GetIgnoredCosts returns the spend of the resources in ignore-labelled resource groups and of the
// ignore-labelled resources elsewhere, attributed to the resource's owner label or else its group's
func (p azureProvider) GetIgnoredCosts(since, until time.Time) (*types.IgnoredCosts, error) {
	ignoredGroups, err := p.getIgnoredResourceGroups()
	if err != nil {
		return nil, fmt.Errorf("[AZURE] Failed to fetch the resource groups: %w", err)
	}
	labelledResourceIDs, err := p.getLabelledResourceIDsOutside(ignoredGroups)
	if err != nil {
		return nil, fmt.Errorf("[AZURE] Failed to fetch the ignore-labelled resources: %w", err)
	}
	costFilter := ignoredCostFilter(ignoredGroups, labelledResourceIDs)
	if costFilter == nil {
		return &types.IgnoredCosts{ByOwner: map[string]float64{}, ByResource: map[string]float64{}}, nil
	}
	log.Infof("[AZURE] Querying the costs of %d ignored resource groups and %d ignore-labelled resources outside them", len(ignoredGroups), len(labelledResourceIDs))
	query := ignoredCostQuery(since, until, p.cfg.OwnerLabel, costFilter)
	costs := &types.IgnoredCosts{ByOwner: map[string]float64{}, ByResource: map[string]float64{}}
	pager := p.costClient.NewUsagePager("subscriptions/"+p.subscriptionID, query)
	for pager.More() {
		response, err := pager.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("[AZURE] Failed to query the costs: %w", err)
		}
		if response.Properties == nil {
			return nil, errors.New("[AZURE] The cost query returned no result")
		}
		pageCosts, err := parseIgnoredCosts(response.Properties, ignoredGroups)
		if err != nil {
			return nil, err
		}
		for owner, cost := range pageCosts.ByOwner {
			costs.ByOwner[owner] += cost
		}
		for resourceID, cost := range pageCosts.ByResource {
			costs.ByResource[resourceID] += cost
		}
	}
	return costs, nil
}

func (p azureProvider) getLabelledResourceIDsOutside(ignoredGroups ignoredResourceGroups) ([]string, error) {
	var resourceIDs []string
	pager := p.resourceClient.NewListPager(&armresources.ClientListOptions{Filter: to.Ptr(fmt.Sprintf("tagName eq '%s'", p.cfg.IgnoreLabel))})
	for pager.More() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		for _, resource := range page.Value {
			if _, inIgnoredGroup := ignoredGroups[strings.ToLower(resourceGroupOf(*resource.ID))]; !inIgnoredGroup {
				resourceIDs = append(resourceIDs, *resource.ID)
			}
		}
	}
	return resourceIDs, nil
}

func resourceGroupOf(resourceID string) string {
	parsed, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return ""
	}
	return parsed.ResourceGroupName
}

func ignoredCostFilter(ignoredGroups ignoredResourceGroups, labelledResourceIDs []string) *armcostmanagement.QueryFilter {
	var filters []*armcostmanagement.QueryFilter
	if len(ignoredGroups) > 0 {
		filters = append(filters, dimensionIn(resourceGroupNameColumn, groupNames(ignoredGroups)))
	}
	if len(labelledResourceIDs) > 0 {
		filters = append(filters, dimensionIn(resourceIDColumn, labelledResourceIDs))
	}
	switch len(filters) {
	case 0:
		return nil
	case 1:
		return filters[0]
	default:
		return &armcostmanagement.QueryFilter{Or: filters}
	}
}

func groupNames(groups ignoredResourceGroups) []string {
	names := make([]string, 0, len(groups))
	for _, group := range groups {
		names = append(names, group.name)
	}
	return names
}

func dimensionIn(name string, values []string) *armcostmanagement.QueryFilter {
	return &armcostmanagement.QueryFilter{Dimensions: &armcostmanagement.QueryComparisonExpression{
		Name:     to.Ptr(name),
		Operator: to.Ptr(armcostmanagement.QueryOperatorTypeIn),
		Values:   to.SliceOfPtrs(values...),
	}}
}

func ignoredCostQuery(since, until time.Time, ownerLabel string, filter *armcostmanagement.QueryFilter) armcostmanagement.QueryDefinition {
	return armcostmanagement.QueryDefinition{
		Type:       to.Ptr(armcostmanagement.ExportTypeActualCost),
		Timeframe:  to.Ptr(armcostmanagement.TimeframeTypeCustom),
		TimePeriod: &armcostmanagement.QueryTimePeriod{From: to.Ptr(since), To: to.Ptr(lastDayBefore(until))},
		Dataset: &armcostmanagement.QueryDataset{
			Aggregation: map[string]*armcostmanagement.QueryAggregation{
				"totalCost": {Name: to.Ptr(costColumn), Function: to.Ptr(armcostmanagement.FunctionTypeSum)},
			},
			Grouping: []*armcostmanagement.QueryGrouping{
				{Type: to.Ptr(armcostmanagement.QueryColumnTypeDimension), Name: to.Ptr(resourceIDColumn)},
				{Type: to.Ptr(armcostmanagement.QueryColumnTypeTagKey), Name: to.Ptr(ownerLabel)},
			},
			Filter: filter,
		},
	}
}

// lastDayBefore turns the exclusive end into the inclusive one Cost Management expects, it ignores the time of day of both ends
func lastDayBefore(until time.Time) time.Time {
	return until.AddDate(0, 0, -1)
}

func parseIgnoredCosts(properties *armcostmanagement.QueryProperties, ignoredGroups ignoredResourceGroups) (*types.IgnoredCosts, error) {
	columnIndexes := map[string]int{}
	for index, column := range properties.Columns {
		columnIndexes[*column.Name] = index
	}
	costIndex, hasCost := columnIndexes[costColumn]
	resourceIDIndex, hasResourceID := columnIndexes[resourceIDColumn]
	ownerIndex, hasOwner := columnIndexes[tagValueColumn]
	if !hasCost || !hasResourceID || !hasOwner {
		return nil, fmt.Errorf("[AZURE] The cost query result misses one of the columns %s, %s, %s", costColumn, resourceIDColumn, tagValueColumn)
	}

	costs := &types.IgnoredCosts{ByOwner: map[string]float64{}, ByResource: map[string]float64{}}
	for _, row := range properties.Rows {
		cost, _ := row[costIndex].(float64)
		resourceID, _ := row[resourceIDIndex].(string)
		owner, _ := row[ownerIndex].(string)
		if owner == "" {
			owner = ignoredGroups[strings.ToLower(resourceGroupOf(resourceID))].owner
		}
		costs.ByResource[strings.ToLower(resourceID)] += cost
		if owner != "" {
			costs.ByOwner[owner] += cost
		}
	}
	return costs, nil
}
