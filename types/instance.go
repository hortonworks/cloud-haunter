package types

import "time"

type Instance struct {
	Id           string                 `json:"Id"`
	Name         string                 `json:"Name"`
	Created      time.Time              `json:"Created"`
	Tags         Tags                   `json:"Tags"`
	Owner        string                 `json:"Owner"`
	CloudType    CloudType              `json:"CloudType"`
	InstanceType string                 `json:"InstanceType"`
	State        State                  `json:"State"`
	Metadata     map[string]interface{} `json:"Metadata"`
	Region       string                 `json:"Region"`
}

type Tags map[string]string

func (i Instance) GetName() string {
	return i.Name
}

func (i Instance) GetOwner() string {
	if len(i.Owner) == 0 {
		return "???"
	}
	return i.Owner
}

func (i Instance) GetCloudType() CloudType {
	return i.CloudType
}

func (i Instance) GetCreated() time.Time {
	return i.Created
}

func (i Instance) GetItem() interface{} {
	return i
}

func (i Instance) GetType() string {
	return "instance"
}
