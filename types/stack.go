package types

import "time"

// Stack represents a collection of resources (CF, ARM)
type Stack struct {
	ID        string            `json:"Id"`
	Name      string            `json:"Name"`
	Created   time.Time         `json:"Created"`
	Tags      Tags              `json:"Tags"`
	Owner     string            `json:"Owner"`
	CloudType CloudType         `json:"CloudType"`
	State     State             `json:"State"`
	Metadata  map[string]string `json:"Metadata"`
	Region    string            `json:"Region"`
}

func (s Stack) GetName() string {
	return s.Name
}

// GetOwner returns the 'Owner' tag's value of the instance. If there is not tag present then returns '???'
func (s Stack) GetOwner() string {
	if len(s.Owner) == 0 {
		return "???"
	}
	return s.Owner
}

// GetCloudType returns the type of the cloud (AWS/AZURE/GCP)
func (s Stack) GetCloudType() CloudType {
	return s.CloudType
}

// GetCreated returns the creation time of the instance
func (s Stack) GetCreated() time.Time {
	return s.Created
}

// GetItem returns the cloud instance object itself
func (s Stack) GetItem() interface{} {
	return s
}

// GetType returns the type representation of the instance
func (s Stack) GetType() string {
	return "stack"
}