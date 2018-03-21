package types

import "time"

type Instance struct {
	Id        string                 `json:"Id"`
	Name      string                 `json:"Name"`
	Created   time.Time              `json:"Created"`
	Tags      Tags                   `json:"Tags"`
	Owner     string                 `json:"Owner"`
	CloudType CloudType              `json:"CloudType"`
	Metadata  map[string]interface{} `json:"Metadata"`
}

type Tags map[string]string

type S struct {
	S string
}
