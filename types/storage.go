package types

import "time"

type StorageContainer struct {
	storages []*Storage
}

func (c *StorageContainer) Get(cloudType CloudType) []*Storage {
	items := []*Storage{}
	for _, item := range c.storages {
		if item.CloudType == cloudType {
			items = append(items, item)
		}
	}
	return items
}

func NewStorageContainer(storages []*Storage) *StorageContainer {
	return &StorageContainer{storages}
}

// Storage cloud object used to store files on cloud provider
type Storage struct {
	ID        string            `json:"ID"`
	Name      string            `json:"Name"`
	Owner     string            `json:"Owner"`
	Created   time.Time         `json:"Created"`
	CloudType CloudType         `json:"CloudType"`
	Tags      Tags              `json:"Tags"`
	Region    string            `json:"Region"`
	MetaData  map[string]string `json:"MetaData"`
}

// GetName returns the name of the storage cloud object
func (s Storage) GetName() string {
	return s.Name
}

// GetOwner returns the owner of the storage cloud object
func (s Storage) GetOwner() string {
	if len(s.Owner) == 0 {
		return "???"
	}
	return s.Owner
}

// GetCloudType returns the type of the cloud
func (s Storage) GetCloudType() CloudType {
	return s.CloudType
}

// GetCreated returns the creation date of the storage cloud object
func (s Storage) GetCreated() time.Time {
	return s.Created
}

// GetItem returns the storage cloud object itself
func (s Storage) GetItem() interface{} {
	return s
}

// GetType returns the type of the storage cloud object
func (s Storage) GetType() string {
	return "storage"
}

func (s Storage) GetTags() Tags {
	return s.Tags
}
