package types

import "time"

type Database struct {
	Id           string    `json:"Id"`
	Name         string    `json:"Name"`
	Created      time.Time `json:"Created"`
	Owner        string    `json:"Owner"`
	InstanceType string    `json:"InstanceType"`
	State        State     `json:"State"`
	CloudType    CloudType `json:"CloudType"`
	Region       string    `json:"Region"`
}

func (d Database) GetName() string {
	return d.Name
}

func (d Database) GetOwner() string {
	return d.Owner
}

func (d Database) GetCloudType() CloudType {
	return d.CloudType
}

func (d Database) GetCreated() time.Time {
	return d.Created
}

func (d Database) GetItem() interface{} {
	return d
}

func (d Database) GetType() string {
	return "database"
}
