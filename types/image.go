package types

import "time"

type ImageContainer struct {
	images []*Image
}

func (c *ImageContainer) Get(cloudType CloudType) []*Image {
	items := []*Image{}
	for _, item := range c.images {
		if item.CloudType == cloudType {
			items = append(items, item)
		}
	}
	return items
}

func NewImageContainer(images []*Image) *ImageContainer {
	return &ImageContainer{images}
}

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
