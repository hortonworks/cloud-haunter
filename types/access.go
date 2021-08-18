package types

import "time"

// Access cloud object used to authenticate against the cloud provider
type Access struct {
	Name      string    `json:"Name"`
	Owner     string    `json:"Owner"`
	Created   time.Time `json:"Created"`
	CloudType CloudType `json:"CloudType"`
	Tags      Tags      `json:"Tags"`
}

// GetName returns the name of the access cloud object
func (a Access) GetName() string {
	return a.Name
}

// GetOwner returns the owner of the access cloud object
func (a Access) GetOwner() string {
	if len(a.Owner) == 0 {
		return "???"
	}
	return a.Owner
}

// GetCloudType returns the type of the cloud
func (a Access) GetCloudType() CloudType {
	return a.CloudType
}

// GetCreated returns the creation date of the access cloud object
func (a Access) GetCreated() time.Time {
	return a.Created
}

// GetItem returns the access cloud object itself
func (a Access) GetItem() interface{} {
	return a
}

// GetType returns the type of the access cloud object
func (a Access) GetType() string {
	return "access"
}

func (a Access) GetTags() Tags {
	return a.Tags
}
