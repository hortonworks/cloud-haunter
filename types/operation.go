package types

type OpType string

func (ot OpType) String() string {
	return string(ot)
}

const (
	LONGRUNNING = OpType("longrunning")
	OWNERLESS   = OpType("ownerless")
	OLDACCESS   = OpType("oldaccess")
)

type Operation interface {
	Execute([]CloudType) []CloudItem
}
