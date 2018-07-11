package gcp

import (
	"net/http"
	"testing"

	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/stretchr/testify/assert"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

func TestProviderInit(t *testing.T) {
	provider := gcpProvider{}

	provider.init("project-id", &http.Client{})

	assert.Equal(t, "project-id", provider.projectID)
	assert.NotNil(t, provider.computeClient)
}

func TestGetInstances(t *testing.T) {
	instances, _ := getInstances(mockAggregator{})

	assert.Equal(t, 1, len(instances))
}

func TestNewInstance(t *testing.T) {
	instance := newInstance(newTestInstance())

	assert.Equal(t, "instance", instance.Name)
	assert.Equal(t, "owner", instance.Owner)
	assert.Equal(t, "bbb", instance.Metadata["zone"])
	assert.Equal(t, "b", instance.Region)
}

type mockAggregator struct {
}

func (m mockAggregator) Do(opts ...googleapi.CallOption) (*compute.InstanceAggregatedList, error) {
	return &compute.InstanceAggregatedList{
		Items: map[string]compute.InstancesScopedList{
			"key": compute.InstancesScopedList{
				Instances: []*compute.Instance{
					newTestInstance(),
				},
			},
		},
	}, nil
}

func newTestInstance() *compute.Instance {
	return &compute.Instance{
		Name:              "instance",
		CreationTimestamp: "2006-01-02T15:04:05Z",
		Zone:              "a/bbb",
		Labels:            map[string]string{ctx.GcpOwnerLabel: "owner"},
	}
}
