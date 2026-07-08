package types

const (
	// LongRunningFilter filters the cloud items that are created/running after a certain time
	LongRunningFilter = FilterType("longrunning")

	// OwnerlessFilter filters the cloud items that do not have the 'Owner' tag
	OwnerlessFilter = FilterType("ownerless")

	// OldAccessFilter filters the cloud access objects that are created a long time
	OldAccessFilter = FilterType("oldaccess")

	// StoppedFilter filters the cloud items that's state is stopped
	StoppedFilter = FilterType("stopped")

	// RunningFilter filters the cloud items that's state is running
	RunningFilter = FilterType("running")

	// FailedFilter filters the cloud items that's state is failed
	FailedFilter = FilterType("failed")

	// UnusedFilter filters the items that are not used
	UnusedFilter = FilterType("unused")

	// MatchFilter filters the items that match the include criteria of the filter config
	MatchFilter = FilterType("match")

	// NoMatchFilter filters the items that do not match the include criteria of the filter config
	NoMatchFilter = FilterType("nomatch")

	// InclusiveFilter filter type that will return only the matching entries from the filter's inclusive configuration
	InclusiveFilter = FilterConfigType("inclusive")

	// ExclusiveFilter filter type that will filter the matching entries from the filter's exclusive configuration
	ExclusiveFilter = FilterConfigType("exclusive")
)

// FilterConfigType inclusive or exclusive filter type
type FilterConfigType string

// IsInclusive returns true if it's an inclusive filter
func (f FilterConfigType) IsInclusive() bool {
	return f == InclusiveFilter
}

// FilterType returns the type of the filter in string format
type FilterType string

// Filter interface that can be chained
type Filter interface {
	Execute([]CloudItem) []CloudItem
}

func (f FilterType) String() string {
	return string(f)
}
