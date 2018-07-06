package gcp

import (
	"errors"
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
	if context.DryRun {
		log.Debug("[GCP] Fetching instanes")
	}
	instances := make([]*types.Instance, 0)
	instanceList, err := computeClient.Instances.AggregatedList(projectId).Filter("status eq RUNNING").Do()
	if err != nil {
		log.Errorf("[GCP] Failed to fetch the running instances, err: %s", err.Error())
		return nil, err
	}
	if context.DryRun {
		log.Debugf("[GCP] Processing instances (%d): [%s]", len(instanceList.Items), instanceList.Items)
	}
	for _, items := range instanceList.Items {
		for _, inst := range items.Instances {
			instances = append(instances, newInstance(inst))
		}
	}
	return instances, nil
}

func (a GcpProvider) TerminateInstances(instances []*types.Instance) error {
	if context.DryRun {
		log.Debug("[GCP] Terminating instanes")
	}
	instanceGroups, err := computeClient.InstanceGroupManagers.AggregatedList(projectId).Do()
	if err != nil {
		log.Errorf("[GCP] Failed to fetch instance groups, err: %s", err.Error())
		return err
	}

	instancesToDelete := []*types.Instance{}
	instanceGroupsToDelete := map[*compute.InstanceGroupManager]bool{}

	for _, inst := range instances {
		if context.DryRun {
			log.Debugf("[GCP] Terminating instane: %s", inst.GetName())
		}
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

	if context.DryRun {
		log.Debugf("[GCP] Instance groups to terminate (%d) : [%s]", len(instanceGroupsToDelete), instanceGroupsToDelete)
	}
	wg := sync.WaitGroup{}
	wg.Add(len(instanceGroupsToDelete))
	for g := range instanceGroupsToDelete {
		go func(group *compute.InstanceGroupManager) {
			defer wg.Done()

			zone := getZone(group.Zone)
			log.Infof("[GCP] Deleting instance group %s in zone %s", group.Name, zone)
			if context.DryRun {
				log.Info("[GCP] Skipping group termination on dry run session")
			} else {
				_, err := computeClient.InstanceGroupManagers.Delete(projectId, zone, group.Name).Do()
				if err != nil {
					log.Errorf("[GCP] Failed to delete instance group %s, err: %s", group.Name, err.Error())
				}
			}
		}(g)
	}
	if context.DryRun {
		log.Debugf("[GCP] Instances to terminate (%d): [%s]", len(instancesToDelete), instancesToDelete)
	}
	wg.Add(len(instancesToDelete))
	for _, i := range instancesToDelete {
		go func(inst *types.Instance) {
			defer wg.Done()

			zone := inst.Metadata["zone"].(string)
			log.Infof("[GCP] Deleting instance %s in zone %s", inst.Name, zone)
			if context.DryRun {
				log.Info("[GCP] Skipping instance termination on dry run session")
			} else {
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

func (a GcpProvider) GetAccesses() ([]*types.Access, error) {
	return nil, errors.New("[GCP] Access not supported")
}

func getRegions() ([]string, error) {
	if context.DryRun {
		log.Debug("[GCP] Fetching regions")
	}
	regionList, err := computeClient.Regions.List(projectId).Do()
	if err != nil {
		return nil, err
	}
	if context.DryRun {
		log.Debugf("[GCP] Processing regions (%d): [%s]", len(regionList.Items), regionList.Items)
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
		Owner:     inst.Labels[context.GcpOwnerLabel],
		Metadata:  map[string]interface{}{"zone": getZone(inst.Zone)},
		Region:    getRegionFromZoneUrl(&inst.Zone),
	}
}

func getRegionFromZoneUrl(zoneUrl *string) string {
	zoneUrlParts := strings.Split(*zoneUrl, "/")
	zone := zoneUrlParts[len(zoneUrlParts)-1]
	return zone[:len(zone)-2]
}
