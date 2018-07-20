package types

type CloudType string

func (ct CloudType) String() string {
	return string(ct)
}

const (
	AWS   = CloudType("AWS")
	GCP   = CloudType("GCP")
	AZURE = CloudType("AZURE")
	DUMMY = CloudType("DUMMY")
)

type CloudProvider interface {
	GetInstances() ([]*Instance, error)
	StopInstances([]*Instance) error
	TerminateInstances([]*Instance) error
	GetAccesses() ([]*Access, error)
	GetDatabases() ([]*Database, error)
}
