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

	// UnusedFilter filters the items that are not used
	UnusedFilter = FilterType("unused")
)

// FilterType returns the type of the filter in string format
type FilterType string

// Filter interface that can be chained
type Filter interface {
	Execute([]CloudItem) []CloudItem
}

func (f FilterType) String() string {
	return string(f)
}
