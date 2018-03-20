package types

import "time"

type Instance struct {
	Id        string    `json:"Id"`
	Name      string    `json:"Name"`
	Created   time.Time `json:"Created"`
	Tags      Tags      `json:"Tags"`
	CloudType CloudType `json:"CloudType"`
}

type Tags map[string]string

type S struct {
	S string
}
