package types

import (
	"time"
)

type CloudType string

const (
	AWS   = CloudType("AWS")
	GCP   = CloudType("GCP")
	AZURE = CloudType("AZURE")
)

var CloudProviders = make(map[CloudType]CloudProvider)

type CloudProvider interface {
	GetRunningInstances() []*Instance
}

type Instance struct {
	Id      string    `json:"Id"`
	Name    string    `json:"Name"`
	Created time.Time `json:"Created"`
	Tags    Tags      `json:"Tags"`
}

type Tags map[string]string
