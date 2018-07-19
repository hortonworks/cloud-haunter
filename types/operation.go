package types

const (
	Running     = OpType("running")
	CloudAccess = OpType("access")
)

type OpType string

type Operation interface {
	Execute([]CloudType) []CloudItem
}

func (ot OpType) String() string {
	return string(ot)
}
