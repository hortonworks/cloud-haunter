package types

import (
	"bytes"
	"time"
)

type DatabaseContainer struct {
	databases []*Database
}

func (c *DatabaseContainer) Get(cloudType CloudType) []*Database {
	var items []*Database
	for _, item := range c.databases {
		if item.CloudType == cloudType {
			items = append(items, item)
		}
	}
	return items
}

func (d *DatabaseContainer) String() string {
	var result bytes.Buffer
	for _, db := range d.databases {
		result.WriteString(db.Name + " ")
	}
	return result.String()
}

func NewDatabaseContainer(databases []*Database) *DatabaseContainer {
	return &DatabaseContainer{databases}
}

// Database is a general database struct processed by filters and actions
type Database struct {
	ID           string            `json:"Id"`
	Name         string            `json:"Name"`
	Created      time.Time         `json:"Created"`
	Owner        string            `json:"Owner"`
	Tags         Tags              `json:"Tags"`
	InstanceType string            `json:"InstanceType"`
	State        State             `json:"State"`
	CloudType    CloudType         `json:"CloudType"`
	Region       string            `json:"Region"`
	Metadata     map[string]string `json:"Metadata"`
}

// GetName returns the name of the database
func (d Database) GetName() string {
	return d.Name
}

// GetOwner returns the owner of the database
func (d Database) GetOwner() string {
	return d.Owner
}

// GetCloudType returns the type of the cloud
func (d Database) GetCloudType() CloudType {
	return d.CloudType
}

// GetCreated returns the creation time of the database
func (d Database) GetCreated() time.Time {
	return d.Created
}

// GetItem returns the database struct itself
func (d Database) GetItem() interface{} {
	return d
}

// GetType returns the database string representation
func (d Database) GetType() string {
	return "database"
}

func (d Database) GetTags() Tags {
	return d.Tags
}

func (d Database) String() string {
	return d.Name
}
