package types

import "time"

type DiskContainer struct {
	disks []*Disk
}

func (c *DiskContainer) Get(cloudType CloudType) []*Disk {
	items := []*Disk{}
	for _, item := range c.disks {
		if item.CloudType == cloudType {
			items = append(items, item)
		}
	}
	return items
}

func NewDiskContainer(disks []*Disk) *DiskContainer {
	return &DiskContainer{disks}
}

// Disk represents the root and attached disks for the instances
type Disk struct {
	ID        string    `json:"Id"`
	Name      string    `json:"Name"`
	Created   time.Time `json:"Created"`
	State     State     `json:"State"`
	Owner     string    `json:"Owner"`
	CloudType CloudType `json:"CloudType"`
	Region    string    `json:"Region"`
	Size      int64     `json:"Size"`
	Type      string    `json:"Type"`
}

// GetName returns the name of the disk
func (d Disk) GetName() string {
	return d.Name
}

// GetOwner returns the owner of the disk
func (d Disk) GetOwner() string {
	return d.Owner
}

// GetCloudType returns the type of the cloud
func (d Disk) GetCloudType() CloudType {
	return d.CloudType
}

// GetCreated returns the creation time of the disk
func (d Disk) GetCreated() time.Time {
	return d.Created
}

// GetItem returns the disk struct itself
func (d Disk) GetItem() interface{} {
	return d
}

// GetType returns the disk's string representation
func (d Disk) GetType() string {
	return "disk"
}
