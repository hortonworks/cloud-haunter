package types

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
	GetRunningInstances() ([]*Instance, error)
	GetOwnerLessInstances() ([]*Instance, error)
}
