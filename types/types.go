package types

import (
	"time"
)

const (
	// Creating state of the cloud item
	Creating = State("creating")

	// Deleting state of the cloud item
	Deleting = State("deleting")

	// Stopping state of the cloud item
	Error = State("error")

	// InUse state of the cloud item
	InUse = State("in-use")

	// Failed state of the cloud item
	Failed = State("failed")

	// Running state of the cloud item
	Running = State("running")

	// Stopping state of the cloud item
	Starting = State("starting")

	// Stopped state of the cloud item
	Stopped = State("stopped")

	// Stopping state of the cloud item
	Stopping = State("stopping")

	// Terminated state of the cloud item
	Terminated = State("terminated")

	// Updating state of the cloud item
	Updating = State("updating")
>>>>>>> d8b89d6 (add support for gcp dataproc clusters)

	// Unknown state of the cloud item
	Unknown = State("unknown")

	// Unused state of the cloud item
	Unused = State("unused")
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
	GetTags() Tags
}

// Dispatcher interface used to send the messages with
type Dispatcher interface {
	GetName() string
	Send(op OpType, filters []FilterType, items []CloudItem) error
}
