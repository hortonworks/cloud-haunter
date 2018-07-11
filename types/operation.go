package types

type OpType string

func (ot OpType) String() string {
	return string(ot)
}

const (
	LongRunning = OpType("longrunning")
	Ownerless   = OpType("ownerless")
	OldAccess   = OpType("oldaccess")
)

type Operation interface {
	Execute([]CloudType) []CloudItem
}
