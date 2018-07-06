package types

import (
	"time"
)

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
	Send(op *OpType, items []CloudItem) error
}
