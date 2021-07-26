package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"net/http"
	"time"
)

type InstanceContainer struct {
	instances []*Instance
}

func (c *InstanceContainer) Get(cloudType CloudType) []*Instance {
	items := []*Instance{}
	for _, item := range c.instances {
		if item.CloudType == cloudType {
			items = append(items, item)
		}
	}
	return items
}

func NewInstanceContainer(imstances []*Instance) *InstanceContainer {
	return &InstanceContainer{imstances}
}

// Instance is a general cloud instance struct processed by filters and actions
type Instance struct {
	ID           string            `json:"Id"`
	Name         string            `json:"Name"`
	Created      time.Time         `json:"Created"`
	Tags         Tags              `json:"Tags"`
	Owner        string            `json:"Owner"`
	CloudType    CloudType         `json:"CloudType"`
	InstanceType string            `json:"InstanceType"`
	State        State             `json:"State"`
	Metadata     map[string]string `json:"Metadata"`
	Region       string            `json:"Region"`
	Ephemeral    bool              `json:"Ephemeral"`
	IpAddress    string            `json:"IpAddress"`
}

// Tags Key-value pairs of the tags on the instances
type Tags map[string]string

// GetName returns the name of the instance
func (i Instance) GetName() string {
	return i.Name
}

// GetOwner returns the 'Owner' tag's value of the instance. If there is not tag present then returns '???'
func (i Instance) GetOwner() string {
	if len(i.Owner) == 0 {
		return "???"
	}
	return i.Owner
}

// GetCloudType returns the type of the cloud (AWS/AZURE/GCP)
func (i Instance) GetCloudType() CloudType {
	return i.CloudType
}

// GetCreated returns the creation time of the instance
func (i Instance) GetCreated() time.Time {
	return i.Created
}

// GetItem returns the cloud instance object itself
func (i Instance) GetItem() interface{} {
	return i
}

// GetType returns the type representation of the instance
func (i Instance) GetType() string {
	return "instance"
}

func (i Instance) GetTags() Tags {
	return i.Tags
}

type JsonResponseBody struct {
	Json map[string]interface{}
}

type RemoteResponse struct {
	Code  int
	Body  string
	Json  JsonResponseBody
	Error bool
}

// GetUrl returns the result of an HTTP request to the instance
func (i Instance) GetUrl(path string, port string) RemoteResponse {
	var uri string
	checkPort := "80"

	if port == "" {
		uri = fmt.Sprintf("http://%s%s", i.IpAddress, path)
	} else {
		uri = fmt.Sprintf("http://%s:%s%s", i.IpAddress, port, path)
		checkPort = port
	}

	if tcp_port_test(i.IpAddress, checkPort) == false {
		return RemoteResponse{999, "", JsonResponseBody{}, true}
	}

	log.Debugf("Making HTTP request to %s", uri)
	resp, err := http.Get(uri)
	if err != nil {
		log.Errorf("Error fetching %s - %s", uri, err)
		return RemoteResponse{999, "", JsonResponseBody{}, true}
	}
	log.Debug("HTTP response: ", resp.Status)

	if resp.ContentLength < 1 {
		log.Debug("HTTP request returned empty response.")
		return RemoteResponse{resp.StatusCode, "", JsonResponseBody{}, true}
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("Error reading response body", err)
	}

	if resp.Header.Get("Content-Type") == "application/json" {
		jsonbody := JsonResponseBody{}
		json.Unmarshal(body, &jsonbody)
		log.Debug("HTTP Body: ", jsonbody)
		return RemoteResponse{resp.StatusCode, "", jsonbody, false}
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return RemoteResponse{resp.StatusCode, buf.String(), JsonResponseBody{}, false}
}

func tcp_port_test(host string, port string) bool {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		log.Debugf("Error making TCP connection to %s -> %s", net.JoinHostPort(host, port), err)
	}
	if conn != nil {
		defer conn.Close()
		log.Debugf("TCP Port %s is open.", net.JoinHostPort(host, port))
		return true
	}
	return false
}
