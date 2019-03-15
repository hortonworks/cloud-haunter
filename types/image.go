package types

import "time"

// Image represents the images on the cloud providers
type Image struct {
	ID        string    `json:"Id"`
	Name      string    `json:"Name"`
	Created   time.Time `json:"Created"`
	CloudType CloudType `json:"CloudType"`
	Region    string    `json:"Region"`
}

// GetName returns the name of the image
func (img Image) GetName() string {
	return img.Name
}

// GetOwner returns the owner of the image
func (img Image) GetOwner() string {
	return ""
}

// GetCloudType returns the type of the cloud
func (img Image) GetCloudType() CloudType {
	return img.CloudType
}

// GetCreated returns the creation time of the image
func (img Image) GetCreated() time.Time {
	return img.Created
}

// GetItem returns the image struct itself
func (img Image) GetItem() interface{} {
	return img
}

// GetType returns the image's string representation
func (img Image) GetType() string {
	return "image"
}
