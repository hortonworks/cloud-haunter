package gcp

import (
	"errors"
	"net/http"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/hortonworks/cloud-cost-reducer/utils"

	ctx "context"
	"strconv"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

var provider = gcpProvider{}

type gcpProvider struct {
	projectID     string
	computeClient *compute.Service
}

func init() {
	projectID := os.Getenv("GOOGLE_PROJECT_ID")
	if len(projectID) == 0 {
		log.Warn("[GCP] GOOGLE_PROJECT_ID environment variable is missing")
		return
	}
	context.CloudProviders[types.GCP] = func() types.CloudProvider {
		if provider.computeClient == nil {
			log.Infof("[GCP] Trying to prepare")
			httpClient, err := google.DefaultClient(ctx.Background(), compute.CloudPlatformScope)
			if err != nil {
				panic("[GCP] Failed to authenticate, err: " + err.Error())
			}
			if err := provider.init(projectID, httpClient); err != nil {
				panic("[GCP] Failed to initialize provider, err: " + err.Error())
			}
			log.Info("[GCP] Successfully prepared")
		}
		return provider
	}
}

func (p *gcpProvider) init(projectID string, httpClient *http.Client) error {
	computeClient, err := compute.New(httpClient)
	if err != nil {
		return errors.New("Failed to authenticate, err: " + err.Error())
	}
	p.projectID = projectID
	p.computeClient = computeClient
	return nil
}

func (p gcpProvider) GetRunningInstances() ([]*types.Instance, error) {
	if context.DryRun {
		log.Debug("[GCP] Fetching instanes")
	}
	return getInstances(p.computeClient.Instances.AggregatedList(p.projectID).Filter("status eq RUNNING"))
}

func (p gcpProvider) TerminateInstances(instances []*types.Instance) error {
	return errors.New("[GCP] Termination not supported")
	// if context.DryRun {
	// 	log.Debug("[GCP] Terminating instanes")
	// }
	// instanceGroups, err := p.computeClient.InstanceGroupManagers.AggregatedList(p.projectId).Do()
	// if err != nil {
	// 	log.Errorf("[GCP] Failed to fetch instance groups, err: %s", err.Error())
	// 	return err
	// }

	// instancesToDelete := []*types.Instance{}
	// instanceGroupsToDelete := map[*compute.InstanceGroupManager]bool{}

	// for _, inst := range instances {
	// 	if context.DryRun {
	// 		log.Debugf("[GCP] Terminating instane: %s", inst.GetName())
	// 	}
	// 	groupFound := false
	// 	for _, i := range instanceGroups.Items {
	// 		for _, group := range i.InstanceGroupManagers {
	// 			if _, ok := instanceGroupsToDelete[group]; !ok && strings.Index(inst.Name, group.BaseInstanceName+"-") == 0 {
	// 				instanceGroupsToDelete[group], groupFound = true, true
	// 			}
	// 		}
	// 	}
	// 	if !groupFound {
	// 		instancesToDelete = append(instancesToDelete, inst)
	// 	}
	// }

	// if context.DryRun {
	// 	log.Debugf("[GCP] Instance groups to terminate (%d) : [%s]", len(instanceGroupsToDelete), instanceGroupsToDelete)
	// }
	// wg := sync.WaitGroup{}
	// wg.Add(len(instanceGroupsToDelete))
	// for g := range instanceGroupsToDelete {
	// 	go func(group *compute.InstanceGroupManager) {
	// 		defer wg.Done()

	// 		zone := getZone(group.Zone)
	// 		log.Infof("[GCP] Deleting instance group %s in zone %s", group.Name, zone)
	// 		if context.DryRun {
	// 			log.Info("[GCP] Skipping group termination on dry run session")
	// 		} else {
	// 			_, err := p.computeClient.InstanceGroupManagers.Delete(p.projectId, zone, group.Name).Do()
	// 			if err != nil {
	// 				log.Errorf("[GCP] Failed to delete instance group %s, err: %s", group.Name, err.Error())
	// 			}
	// 		}
	// 	}(g)
	// }
	// if context.DryRun {
	// 	log.Debugf("[GCP] Instances to terminate (%d): [%s]", len(instancesToDelete), instancesToDelete)
	// }
	// wg.Add(len(instancesToDelete))
	// for _, i := range instancesToDelete {
	// 	go func(inst *types.Instance) {
	// 		defer wg.Done()

	// 		zone := inst.Metadata["zone"].(string)
	// 		log.Infof("[GCP] Deleting instance %s in zone %s", inst.Name, zone)
	// 		if context.DryRun {
	// 			log.Info("[GCP] Skipping instance termination on dry run session")
	// 		} else {
	// 			_, err := p.computeClient.Instances.Delete(p.projectId, zone, inst.Name).Do()
	// 			if err != nil {
	// 				log.Errorf("[GCP] Failed to delete instance %s, err: %s", inst.Name, err.Error())
	// 			}
	// 		}
	// 	}(i)
	// }
	// wg.Wait()
	// return nil
}

func (p gcpProvider) GetAccesses() ([]*types.Access, error) {
	return nil, errors.New("[GCP] Access not supported")
}

type aggregator interface {
	Do(...googleapi.CallOption) (*compute.InstanceAggregatedList, error)
}

func getInstances(aggregator aggregator) ([]*types.Instance, error) {
	instances := make([]*types.Instance, 0)
	instanceList, err := aggregator.Do()
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

// func getRegions(p gcpProvider) ([]string, error) {
// 	if context.DryRun {
// 		log.Debug("[GCP] Fetching regions")
// 	}
// 	regionList, err := p.computeClient.Regions.List(p.projectId).Do()
// 	if err != nil {
// 		return nil, err
// 	}
// 	if context.DryRun {
// 		log.Debugf("[GCP] Processing regions (%d): [%s]", len(regionList.Items), regionList.Items)
// 	}
// 	regions := make([]string, 0)
// 	for _, region := range regionList.Items {
// 		regions = append(regions, region.Name)
// 	}
// 	log.Infof("[GCP] Available regions: %v", regions)
// 	return regions, nil
// }

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
		Region:    getRegionFromZoneURL(&inst.Zone),
	}
}

func getZone(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

func getRegionFromZoneURL(zoneURL *string) string {
	zoneURLParts := strings.Split(*zoneURL, "/")
	zone := zoneURLParts[len(zoneURLParts)-1]
	return zone[:len(zone)-2]
}
