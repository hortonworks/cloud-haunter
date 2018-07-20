package types

type ActionType string

func (ot ActionType) String() string {
	return string(ot)
}

const (
	LogAction          = ActionType("log")
	StopAction         = ActionType("stop")
	NotificationAction = ActionType("notification")
	TerminationAction  = ActionType("termination")
)

type Action interface {
	Execute(OpType, []FilterType, []CloudItem)
}
