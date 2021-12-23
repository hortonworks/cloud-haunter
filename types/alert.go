package types

import "time"

type AlertContainer struct {
	alerts []*Alert
}

func (c *AlertContainer) Get(cloudType CloudType) []*Alert {
	items := []*Alert{}
	for _, item := range c.alerts {
		if item.CloudType == cloudType {
			items = append(items, item)
		}
	}
	return items
}

func NewAlertContainer(alerts []*Alert) *AlertContainer {
	return &AlertContainer{alerts}
}

type Alert struct {
	ID        string            `json:"Id"`
	Name      string            `json:"Name"`
	Created   time.Time         `json:"Created"`
	CloudType CloudType         `json:"CloudType"`
	Region    string            `json:"Region"`
	Owner     string            `json:"Owner"`
	State     State             `json:"State"`
	Tags      Tags              `json:"Tags"`
	Metadata  map[string]string `json:"Metadata"`
}

// GetName returns the name of the alert
func (a Alert) GetName() string {
	return a.Name
}

// GetOwner returns the 'Owner' tag's value of the alert. If there is not tag present then returns '???'
func (a Alert) GetOwner() string {
	if len(a.Owner) == 0 {
		return "???"
	}
	return a.Owner
}

// GetCloudType returns the type of the cloud (AWS/AZURE/GCP)
func (a Alert) GetCloudType() CloudType {
	return a.CloudType
}

// GetCreated returns the creation time of the alert
func (a Alert) GetCreated() time.Time {
	return a.Created
}

// GetItem returns the cloud alert object itself
func (a Alert) GetItem() interface{} {
	return a
}

// GetType returns the type representation of the alert
func (a Alert) GetType() string {
	return "alert"
}

func (a Alert) GetTags() Tags {
	return a.Tags
}
