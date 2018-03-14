package types

import "time"

type OpType string

func (ot OpType) String() string {
	return string(ot)
}

const (
	HELP        = OpType("help")
	LONGRUNNING = OpType("longrunning")
	TERMINATION = OpType("termination")
)

type Operation interface {
	Execute([]CloudType)
}

type CloudType string

func (ct CloudType) String() string {
	return string(ct)
}

const (
	AWS   = CloudType("AWS")
	GCP   = CloudType("GCP")
	AZURE = CloudType("AZURE")
)

type CloudProvider interface {
	GetRunningInstances() []*Instance
	TerminateRunningInstances() []*Instance
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
