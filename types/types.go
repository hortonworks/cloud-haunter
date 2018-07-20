package types

import (
	"time"
)

const (
	Running    = State("running")
	Stopped    = State("stopped")
	Terminated = State("terminated")
	Unknown    = State("unknown")
)

type State string

type S struct {
	S string
}

type I64 struct {
	I int64
}

type CloudItem interface {
	GetName() string
	GetOwner() string
	GetCloudType() CloudType
	GetCreated() time.Time
	GetItem() interface{}
	GetType() string
}

type Dispatcher interface {
	GetName() string
	Send(op OpType, filters []FilterType, items []CloudItem) error
}

type EnvResolver func(string) string
