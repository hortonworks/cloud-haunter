package types

const (
	// Instances operation to return all the instances
	Instances = OpType("getInstances")

	// CloudAccess operation to return all the cloud access objects
	CloudAccess = OpType("getAccess")

	// Databases operation to return all the databases
	Databases = OpType("getDatabases")
)

// OpType type of the operation
type OpType string

// Operation is executed against cloud providers
// The result should be a general cloud item that can be specialized later
type Operation interface {
	Execute([]CloudType) []CloudItem
}

func (ot OpType) String() string {
	return string(ot)
}
