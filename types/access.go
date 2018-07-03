package types

import "time"

type Access struct {
	Name      string    `json:"Name"`
	Owner     string    `json:"Owner"`
	Created   time.Time `json:"Created"`
	CloudType CloudType `json:"CloudType"`
}

func (a Access) GetName() string {
	return a.Name
}

func (a Access) GetOwner() string {
	if len(a.Owner) == 0 {
		return "???"
	}
	return a.Owner
}

func (a Access) GetCloudType() CloudType {
	return a.CloudType
}

func (a Access) GetCreated() time.Time {
	return a.Created
}

func (a Access) GetItem() interface{} {
	return a
}

func (a Access) GetType() string {
	return "access"
}
