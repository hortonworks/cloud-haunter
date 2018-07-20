package types

const (
	LongRunningFilter = FilterType("longrunning")
	OwnerlessFilter   = FilterType("ownerless")
	OldAccessFilter   = FilterType("oldaccess")
	StoppedFilter     = FilterType("stopped")
)

type FilterType string

type Filter interface {
	Execute([]CloudItem) []CloudItem
}

func (f FilterType) String() string {
	return string(f)
}
