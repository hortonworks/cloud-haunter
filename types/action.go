package types

type ActionType string

func (ot ActionType) String() string {
	return string(ot)
}

const (
	LOG          = ActionType("log")
	NOTIFICATION = ActionType("notification")
)

type Action interface {
	Execute(string, []*Instance)
}
