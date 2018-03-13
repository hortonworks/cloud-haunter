package gcp

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/types"

	"context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"os"
	"strconv"
	"time"
)

var (
	projectId     string
	computeClient *compute.Service
)

func Init() {
	projectId = os.Getenv("GOOGLE_PROJECT_ID")
	if len(projectId) > 0 {
		log.Infof("[GCP] Trying to register as provider")
		httpClient, err := google.DefaultClient(context.Background(), compute.CloudPlatformScope)
		if err != nil {
			log.Errorf("[GCP] Failed to authenticate, err: %s", err.Error())
			return
		}
		computeClient, err = compute.New(httpClient)
		if err != nil {
			log.Errorf("[GCP] Failed to authenticate, err: %s", err.Error())
			return
		}
		types.CloudProviders[types.GCP] = new(GcpProvider)
		log.Info("[GCP] Successfully registered as provider")
	} else {
		log.Warn("[GCP] GOOGLE_PROJECT_ID environment variable is missing")
	}
}

type GcpProvider struct {
}

func getRegions() ([]string, error) {
	regionList, err := computeClient.Regions.List(projectId).Do()
	if err != nil {
		return nil, err
	}

	regions := make([]string, 0)
	for _, region := range regionList.Items {
		regions = append(regions, region.Name)
	}
	log.Infof("[GCP] Available regions: %v", regions)
	return regions, nil
}

func (p *GcpProvider) GetRunningInstances() []*types.Instance {
	instances := make([]*types.Instance, 0)
	instanceList, err := computeClient.Instances.AggregatedList(projectId).Do()
	if err != nil {
		log.Errorf("[GCP] Failed to fetch the running instances, err: %s", err.Error())
		return instances
	}
	for _, items := range instanceList.Items {
		for _, inst := range items.Instances {
			timestamp, _ := strconv.ParseInt(inst.CreationTimestamp, 10, 64)
			instances = append(instances, &types.Instance{
				Name:    inst.Name,
				Id:      strconv.Itoa(int(inst.Id)),
				Created: time.Unix(timestamp, 0),
			})
		}
	}
	return instances
}
