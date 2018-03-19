package gcp

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"

	ctx "context"
	"os"
	"strconv"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

var (
	IgnoreLabel   string
	OwnerLabel    string
	projectId     string
	computeClient *compute.Service
)

func init() {
	projectId = os.Getenv("GOOGLE_PROJECT_ID")
	if len(projectId) > 0 {
		log.Infof("[GCP] Trying to register as provider")
		httpClient, err := google.DefaultClient(ctx.Background(), compute.CloudPlatformScope)
		if err != nil {
			log.Errorf("[GCP] Failed to authenticate, err: %s", err.Error())
			return
		}
		computeClient, err = compute.New(httpClient)
		if err != nil {
			log.Errorf("[GCP] Failed to authenticate, err: %s", err.Error())
			return
		}
		context.CloudProviders[types.GCP] = new(GcpProvider)
		log.Info("[GCP] Successfully registered as provider")
	} else {
		log.Warn("[GCP] GOOGLE_PROJECT_ID environment variable is missing")
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
			if !isAnyMatch(inst.Labels, IgnoreLabel) {
				instances = append(instances, newInstance(inst))
			}
		}
	}
	return instances, nil
}

func (a GcpProvider) TerminateRunningInstances() ([]*types.Instance, error) {
	instances := make([]*types.Instance, 0)
	instanceList, err := computeClient.Instances.AggregatedList(projectId).Filter("status eq RUNNING").Do()
	if err != nil {
		log.Errorf("[GCP] Failed to fetch the running instances, err: %s", err.Error())
		return nil, err
	}

	instanceGroups, err := computeClient.InstanceGroupManagers.AggregatedList(projectId).Do()
	if err != nil {
		log.Errorf("[GCP] Failed to fetch instance groups, err: %s", err.Error())
		return nil, err
	}

	instancesToDelete := []*compute.Instance{}
	instanceGroupsToDelete := map[*compute.InstanceGroupManager]bool{}

	for _, items := range instanceList.Items {
		for _, inst := range items.Instances {
			if isAnyMatch(inst.Labels, OwnerLabel, IgnoreLabel) {
				continue
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
			instances = append(instances, newInstance(inst))
		}
	}

	for group, _ := range instanceGroupsToDelete {
		zone := getZone(group.Zone)
		log.Infof("[GCP] Deleting instance group %s in zone %s", group.Name, zone)
		if !context.DryRun {
			_, err := computeClient.InstanceGroupManagers.Delete(projectId, zone, group.Name).Do()
			if err != nil {
				log.Errorf("[GCP] Failed to delete instance group %s, err: %s", group.Name, err.Error())
			}
		}
	}
	for _, inst := range instancesToDelete {
		zone := getZone(inst.Zone)
		log.Infof("[GCP] Deleting instance %s in zone %s", inst.Name, zone)
		if !context.DryRun {
			_, err := computeClient.Instances.Delete(projectId, zone, inst.Name).Do()
			if err != nil {
				log.Errorf("[GCP] Failed to delete instance %s, err: %s", inst.Name, err.Error())
			}
		}
	}
	return instances, nil
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

func isAnyMatch(labels map[string]string, ignores ...string) bool {
	for _, label := range ignores {
		if _, ok := labels[label]; ok {
			return true
		}
	}
	return false
}

func convertTime(stringTime string) time.Time {
	convertedTime, err := time.Parse(time.RFC3339, stringTime)
	if err != nil {
		log.Warnf("[GCP] cannot convert time: %s, err: %s", stringTime, err.Error())
	}
	return convertedTime
}

func newInstance(inst *compute.Instance) *types.Instance {
	return &types.Instance{
		Name:      inst.Name,
		Id:        strconv.Itoa(int(inst.Id)),
		Created:   convertTime(inst.CreationTimestamp),
		CloudType: types.GCP,
	}
}
