package gcp

import (
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/hortonworks/cloud-cost-reducer/utils"

	ctx "context"
	"os"
	"strconv"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

var (
	projectId     string
	computeClient *compute.Service
)

func init() {
	context.CloudProviders[types.GCP] = func() types.CloudProvider {
		prepare()
		return new(GcpProvider)
	}
}

func prepare() {
	if computeClient == nil {
		projectId = os.Getenv("GOOGLE_PROJECT_ID")
		if len(projectId) == 0 {
			panic("[GCP] GOOGLE_PROJECT_ID environment variable is missing")
		}
		log.Infof("[GCP] Trying to prepare")
		httpClient, err := google.DefaultClient(ctx.Background(), compute.CloudPlatformScope)
		if err != nil {
			panic("[GCP] Failed to authenticate, err: " + err.Error())
		}
		computeClient, err = compute.New(httpClient)
		if err != nil {
			panic("[GCP] Failed to authenticate, err: " + err.Error())
		}

		log.Info("[GCP] Successfully prepared")
	}
}

type GcpProvider struct {
}

func (p *GcpProvider) GetRunningInstances() ([]*types.Instance, error) {
	instances := make([]*types.Instance, 0)
	instanceList, err := computeClient.Instances.AggregatedList(projectId).Filter("status eq RUNNING").Do()
	if err != nil {
		log.Errorf("[GCP] Failed to fetch the running instances, err: %s", err.Error())
		return nil, err
	}
	for _, items := range instanceList.Items {
		for _, inst := range items.Instances {
			instances = append(instances, newInstance(inst))
		}
	}
	return instances, nil
}

func (a GcpProvider) TerminateInstances(instances []*types.Instance) error {
	instanceGroups, err := computeClient.InstanceGroupManagers.AggregatedList(projectId).Do()
	if err != nil {
		log.Errorf("[GCP] Failed to fetch instance groups, err: %s", err.Error())
		return err
	}

	instancesToDelete := []*types.Instance{}
	instanceGroupsToDelete := map[*compute.InstanceGroupManager]bool{}

	for _, inst := range instances {
		groupFound := false
		for _, i := range instanceGroups.Items {
			for _, group := range i.InstanceGroupManagers {
				if _, ok := instanceGroupsToDelete[group]; !ok && strings.Index(inst.Name, group.BaseInstanceName+"-") == 0 {
					instanceGroupsToDelete[group], groupFound = true, true
				}
			}
		}
		if !groupFound {
			instancesToDelete = append(instancesToDelete, inst)
		}
	}

	wg := sync.WaitGroup{}
	wg.Add(len(instanceGroupsToDelete))
	for g, _ := range instanceGroupsToDelete {
		go func(group *compute.InstanceGroupManager) {
			defer wg.Done()

			zone := getZone(group.Zone)
			if !context.DryRun {
				log.Infof("[GCP] Deleting instance group %s in zone %s", group.Name, zone)
				_, err := computeClient.InstanceGroupManagers.Delete(projectId, zone, group.Name).Do()
				if err != nil {
					log.Errorf("[GCP] Failed to delete instance group %s, err: %s", group.Name, err.Error())
				}
			}
		}(g)
	}
	wg.Add(len(instancesToDelete))
	for _, i := range instancesToDelete {
		go func(inst *types.Instance) {
			defer wg.Done()

			zone := inst.Metadata["zone"].(string)
			if !context.DryRun {
				log.Infof("[GCP] Deleting instance %s in zone %s", inst.Name, zone)
				_, err := computeClient.Instances.Delete(projectId, zone, inst.Name).Do()
				if err != nil {
					log.Errorf("[GCP] Failed to delete instance %s, err: %s", inst.Name, err.Error())
				}
			}
		}(i)
	}
	wg.Wait()
	return nil
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

func getZone(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

func newInstance(inst *compute.Instance) *types.Instance {
	created, err := utils.ConvertTimeRFC3339(inst.CreationTimestamp)
	if err != nil {
		log.Warnf("[GCP] cannot convert time: %s, err: %s", inst.CreationTimestamp, err.Error())
	}
	return &types.Instance{
		Name:      inst.Name,
		Id:        strconv.Itoa(int(inst.Id)),
		Created:   created,
		CloudType: types.GCP,
		Tags:      inst.Labels,
		Metadata:  map[string]interface{}{"zone": getZone(inst.Zone)},
	}
}
