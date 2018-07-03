package types

type ActionType string

func (ot ActionType) String() string {
	return string(ot)
}

const (
	LOG_ACTION          = ActionType("log")
	NOTIFICATION_ACTION = ActionType("notification")
	TERMINATION_ACTION  = ActionType("termination")
)

type Action interface {
	Execute(*OpType, []CloudItem)
}

type Message interface {
	TextMessage(header string) string
}

type Dispatcher interface {
	Send(message Message) error
}
