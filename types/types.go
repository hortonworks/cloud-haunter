package types

import (
	"time"
)

type OpType string

func (ot OpType) String() string {
	return string(ot)
}

const (
	HELP        = OpType("help")
	LONGRUNNING = OpType("longrunning")
)

var Operations = make(map[OpType]Operation)

type Operation interface {
	Execute()
}

type CloudType string

const (
	AWS   = CloudType("AWS")
	GCP   = CloudType("GCP")
	AZURE = CloudType("AZURE")
)

var CloudProviders = make(map[CloudType]CloudProvider)

type CloudProvider interface {
	GetRunningInstances() []*Instance
}

type Instance struct {
	Id      string    `json:"Id"`
	Name    string    `json:"Name"`
	Created time.Time `json:"Created"`
	Tags    Tags      `json:"Tags"`
}

type Tags map[string]string

type S struct {
	S string
}
