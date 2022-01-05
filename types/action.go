package types

// ActionType type of the action
type ActionType string

func (ot ActionType) String() string {
	return string(ot)
}

const (
	// LogAction will log the cloud items to the console
	LogAction = ActionType("log")

	// Json will print the output in processable json format
	Json = ActionType("json")

	// StopAction will stop the cloud item if the item itself supports such operation
	StopAction = ActionType("stop")

	// NotificationAction will send a notification through the dispatcher interface
	NotificationAction = ActionType("notification")

	// TerminationAction terminates the cloud item if the item supports such operation
	TerminationAction = ActionType("termination")

	// CleanupAction cleans up the cloud item  if the item supports such operation
	CleanupAction = ActionType("cleanup")
)

// Action to execute on the cloud items
type Action interface {
	Execute(OpType, []FilterType, []CloudItem)
}
