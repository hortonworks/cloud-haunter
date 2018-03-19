package types

type ActionType string

func (ot ActionType) String() string {
	return string(ot)
}

const (
	LOG = ActionType("log")
)

type Action interface {
	Execute()
}
