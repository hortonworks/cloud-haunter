package types

type OpType string

func (ot OpType) String() string {
	return string(ot)
}

const (
	HELP        = OpType("help")
	LONGRUNNING = OpType("longrunning")
	OWNERLESS   = OpType("ownerless")
)

type Operation interface {
	Execute([]CloudType)
}
