package types

import (
	"time"
)

const (
	// Running state of the cloud item
	Running = State("running")

	// Stopped state of the cloud item
	Stopped = State("stopped")

	// Terminated state of the cloud item
	Terminated = State("terminated")

	// Unknown state of the cloud item
	Unknown = State("unknown")
)

// State string representation of the cloud item
type State string

// S string wrapper struct
type S struct {
	S string
}

// I64 int64 wrapper struct
type I64 struct {
	I int64
}

// CloudItem is a general cloud item that is returned by the operation and processed by the filters and actions
type CloudItem interface {
	GetName() string
	GetOwner() string
	GetCloudType() CloudType
	GetCreated() time.Time
	GetItem() interface{}
	GetType() string
}

// Dispatcher interface used to send the messages with
type Dispatcher interface {
	GetName() string
	Send(op OpType, filters []FilterType, items []CloudItem) error
}
