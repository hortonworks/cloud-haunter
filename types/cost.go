package types

import "time"

const (
	// ResourceGroupIgnoredMetadata is set to "true" on the items whose resource group carries the ignore label
	ResourceGroupIgnoredMetadata = "ResourceGroupIgnored"

	// ResourceGroupOwnerMetadata holds the owner label of the item's resource group
	ResourceGroupOwnerMetadata = "ResourceGroupOwner"
)

// IgnoredCosts is the spend of the ignore-labelled resources of one cloud account over a period, per raw owner label value.
// ByResource is keyed by lower-cased resource ID and may hold resources without the label, it is only looked up for reported items.
type IgnoredCosts struct {
	ByOwner    map[string]float64
	ByResource map[string]float64
}

// CostProvider is implemented by the cloud providers that can report the spend of the ignore-labelled resources
// over the days from since up to, but not including, until, both at midnight UTC
type CostProvider interface {
	GetIgnoredCosts(since, until time.Time) (*IgnoredCosts, error)
}

// CostedItem is a cloud item with its spend over the budget period, nil when the cloud does not report it per resource
type CostedItem struct {
	Item CloudItem
	Cost *float64
}

// OwnerSpend is the ignore-labelled spend of one owner across every cloud, with the owner's long-running items
type OwnerSpend struct {
	Owner string
	Spend float64
	Items []CostedItem
}

// BudgetReport lists the owners whose ignore-labelled spend over the period exceeds the limit
type BudgetReport struct {
	Period time.Duration
	Limit  float64
	Owners []OwnerSpend
}

// BudgetDispatcher is implemented by the dispatchers that can deliver a budget report
type BudgetDispatcher interface {
	SendBudgetReport(report *BudgetReport) error
}

// GetItemID returns the cloud ID of the instances, databases and disks, empty for any other item
func GetItemID(item CloudItem) string {
	switch typed := item.GetItem().(type) {
	case Instance:
		return typed.ID
	case Database:
		return typed.ID
	case Disk:
		return typed.ID
	default:
		return ""
	}
}

// GetItemMetadata returns the metadata of the instances, databases and disks, nil for any other item
func GetItemMetadata(item CloudItem) map[string]string {
	switch typed := item.GetItem().(type) {
	case Instance:
		return typed.Metadata
	case Database:
		return typed.Metadata
	case Disk:
		return typed.Metadata
	default:
		return nil
	}
}
