package types

import "time"

type ResourceType string

func (rt ResourceType) String() string {
	return string(rt)
}

const (
	LoadBalancer = ResourceType("LoadBalancer")
	Vpc          = ResourceType("Vpc")
	Subnet       = ResourceType("Subnet")
)

type ResourceContainer struct {
	resources []*Resource
}

func (c *ResourceContainer) Get(cloudType CloudType) []*Resource {
	var items []*Resource
	for _, item := range c.resources {
		if item.CloudType == cloudType {
			items = append(items, item)
		}
	}
	return items
}

func NewResourceContainer(resources []*Resource) *ResourceContainer {
	return &ResourceContainer{resources}
}

// Resource represents a collection of resources (Load balancers, etc...)
type Resource struct {
	ID           string       `json:"Id"`
	Name         string       `json:"Name"`
	Created      time.Time    `json:"Created"`
	Tags         Tags         `json:"Tags"`
	Owner        string       `json:"Owner"`
	CloudType    CloudType    `json:"CloudType"`
	Region       string       `json:"Region"`
	ResourceType ResourceType `json:"ResourceType"`
}

func (s Resource) GetName() string {
	return s.Name
}

// GetOwner returns the 'Owner' tag's value of the resource. If there is not tag present then returns '???'
func (s Resource) GetOwner() string {
	if len(s.Owner) == 0 {
		return "???"
	}
	return s.Owner
}

// GetCloudType returns the type of the cloud (AWS/AZURE/GCP)
func (s Resource) GetCloudType() CloudType {
	return s.CloudType
}

// GetCreated returns the creation time of the resource
func (s Resource) GetCreated() time.Time {
	return s.Created
}

// GetItem returns the cloud resource object itself
func (s Resource) GetItem() interface{} {
	return s
}

// GetType returns the type representation of the resource
func (s Resource) GetType() string {
	return "resource"
}

func (s Resource) GetTags() Tags {
	return s.Tags
}

// GetResourceType returns the type of the resource (LoadBalancer, etc...)
func (s Resource) GetResourceType() ResourceType {
	return s.ResourceType
}
