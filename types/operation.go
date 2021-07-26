package types

const (
	// Instances operation to return all the instances
	Instances = OpType("getInstances")

	// CloudAccess operation to return all the cloud access objects
	CloudAccess = OpType("getAccess")

	// Databases operation to return all the databases
	Databases = OpType("getDatabases")

	// Disks operation to return all the disks
	Disks = OpType("getDisks")

	// Images operation to return all the images
	Images = OpType("getImages")

	// ReadImages operation to return all the images from sdin
	ReadImages = OpType("readImages")

	// Stacks operation to return all stack (CF, ARM..)
	Stacks = OpType("getStacks")

	// Alerts operation to return all alerts (e.g. CloudWatch)
	Alerts = OpType("getAlerts")

	// Storages operation to return all storages (S3, storage account..)
	Storages = OpType("getStorages")

	// Clusters operation to return all clusters
	Clusters = OpType("getClusters")
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
