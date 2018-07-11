package types

type ActionType string

func (ot ActionType) String() string {
	return string(ot)
}

const (
	LogAction          = ActionType("log")
	NotificationAction = ActionType("notification")
	TerminationAction  = ActionType("termination")
)

type Action interface {
	Execute(*OpType, []CloudItem)
}
