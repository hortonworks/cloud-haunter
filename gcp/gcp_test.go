package gcp

import (
	"net/http"
	"testing"
	"time"

	ctx "github.com/blentz/cloud-haunter/context"
	"github.com/blentz/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	iam "google.golang.org/api/iam/v1"
)

func TestProviderInit(t *testing.T) {
	provider := gcpProvider{}

	provider.init("project-id", &http.Client{}, &http.Client{}, &http.Client{})

	assert.Equal(t, "project-id", provider.projectID)
	assert.NotNil(t, provider.computeClient)
}

func TestGetInstances(t *testing.T) {
	instances, _ := getInstances(mockInstancesListAggregator{})

	assert.Equal(t, 1, len(instances))
}

func TestGetAccesses(t *testing.T) {
	accesses, _ := getAccesses(mockServiceAccountsListAggregator{}, func(string) keysListAggregator {
		return mockKeysListAggregator{}
	})

	if 1 != len(accesses) {
		t.Fatalf("Accesses length not equals 1 == %d", len(accesses))
	}
	assert.Equal(t, "valid", accesses[0].Name)
}

func TestDeleteImages(t *testing.T) {
	imageChan := make(chan string)
	getMockAggregator := func(ID string) imageDeleteAggregator {
		imageChan <- ID
		return mockImageDeleteAggregator{}
	}
	images := []*types.Image{
		{CloudType: types.GCP, ID: "sequenceiqimage/hdc-hdp--1711170803.tar.gz"},
	}

	go func() {
		defer close(imageChan)

		deleteImages(getMockAggregator, images)
	}()

	assert.Equal(t, "hdc-hdp--1711170803", <-imageChan)
}

func TestNewInstance(t *testing.T) {
	instance := newInstance(newTestInstance())

	assert.Equal(t, "instance", instance.Name)
	assert.Equal(t, "owner", instance.Owner)
	assert.Equal(t, "bbb", instance.Metadata["zone"])
	assert.Equal(t, "b", instance.Region)
}

type mockInstancesListAggregator struct {
}

func (m mockInstancesListAggregator) Do(opts ...googleapi.CallOption) (*compute.InstanceAggregatedList, error) {
	return &compute.InstanceAggregatedList{
		Items: map[string]compute.InstancesScopedList{
			"key": {
				Instances: []*compute.Instance{
					newTestInstance(),
				},
			},
		},
	}, nil
}

type mockImageDeleteAggregator struct {
	optsChan chan (googleapi.CallOption)
}

func (m mockImageDeleteAggregator) Do(opts ...googleapi.CallOption) (*compute.Operation, error) {
	for _, opt := range opts {
		m.optsChan <- opt
	}
	return nil, nil
}

type mockServiceAccountsListAggregator struct {
}

func (m mockServiceAccountsListAggregator) Do(opts ...googleapi.CallOption) (*iam.ListServiceAccountsResponse, error) {
	return &iam.ListServiceAccountsResponse{
		Accounts: []*iam.ServiceAccount{
			{
				Name:  "service-account",
				Email: "service@account.com",
			},
		},
	}, nil
}

type mockKeysListAggregator struct {
}

func (m mockKeysListAggregator) Do(opts ...googleapi.CallOption) (*iam.ListServiceAccountKeysResponse, error) {
	now := time.Now()
	return &iam.ListServiceAccountKeysResponse{
		Keys: []*iam.ServiceAccountKey{
			{
				Name:            "expired",
				ValidBeforeTime: now.Add(-1 * time.Second).Format(time.RFC3339),
			},
			{
				Name:            "valid",
				ValidBeforeTime: now.Add(1 * time.Second).Format(time.RFC3339),
				ValidAfterTime:  now.Format(time.RFC3339),
			},
		},
	}, nil
}

func newTestInstance() *compute.Instance {
	return &compute.Instance{
		Name:              "instance",
		CreationTimestamp: "2006-01-02T15:04:05Z",
		Zone:              "a/bbb",
		Labels:            map[string]string{ctx.OwnerLabel: "owner"},
		MachineType:       "n1-standard-8",
	}
}
