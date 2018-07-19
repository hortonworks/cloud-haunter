package types

const (
	LongRunning = FilterType("longrunning")
	Ownerless   = FilterType("ownerless")
	OldAccess   = FilterType("oldaccess")
)

type FilterType string

type Filter interface {
	Execute([]CloudItem) []CloudItem
}

func (f FilterType) String() string {
	return string(f)
}
