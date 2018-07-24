package gcp

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"

	"context"
	"strconv"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
)

var provider = gcpProvider{}

type gcpProvider struct {
	projectID     string
	computeClient *compute.Service
	iamClient     *iam.Service
}

func init() {
	projectID := os.Getenv("GOOGLE_PROJECT_ID")
	if len(projectID) == 0 {
		log.Warn("[GCP] GOOGLE_PROJECT_ID environment variable is missing")
		return
	}
	ctx.CloudProviders[types.GCP] = func() types.CloudProvider {
		if len(provider.projectID) == 0 {
			log.Infof("[GCP] Trying to prepare")
			computeClient, iamClient, err := initClients()
			if err != nil {
				panic("[GCP] Failed to authenticate, err: " + err.Error())
			}
			if err := provider.init(projectID, computeClient, iamClient); err != nil {
				panic("[GCP] Failed to initialize provider, err: " + err.Error())
			}
			log.Info("[GCP] Successfully prepared")
		}
		return provider
	}
}

func initClients() (computeClient *http.Client, iamClient *http.Client, err error) {
	computeClient, err = google.DefaultClient(context.Background(), compute.CloudPlatformScope)
	if err != nil {
		return
	}
	iamClient, err = google.DefaultClient(context.Background(), iam.CloudPlatformScope)
	if err != nil {
		return
	}
	return
}

func (p *gcpProvider) init(projectID string, computeHTTPClient *http.Client, iamHTTPClient *http.Client) error {
	p.projectID = projectID
	computeClient, err := compute.New(computeHTTPClient)
	if err != nil {
		return errors.New("Failed to initialize compute client, err: " + err.Error())
	}
	p.computeClient = computeClient
	iamClient, err := iam.New(iamHTTPClient)
	if err != nil {
		return errors.New("Failed to initialize iam client, err: " + err.Error())
	}
	p.iamClient = iamClient
	return nil
}

func (p gcpProvider) GetAccountName() string {
	return p.projectID
}

func (p gcpProvider) GetInstances() ([]*types.Instance, error) {
	log.Debug("[GCP] Fetching instanes")
	return getInstances(p.computeClient.Instances.AggregatedList(p.projectID))
}

func (p gcpProvider) TerminateInstances(instances []*types.Instance) error {
	return errors.New("[GCP] Termination not supported")
	// 	log.Debug("[GCP] Terminating instanes")
	// instanceGroups, err := p.computeClient.InstanceGroupManagers.AggregatedList(p.projectId).Do()
	// if err != nil {
	// 	log.Errorf("[GCP] Failed to fetch instance groups, err: %s", err.Error())
	// 	return err
	// }

	// instancesToDelete := []*types.Instance{}
	// instanceGroupsToDelete := map[*compute.InstanceGroupManager]bool{}

	// for _, inst := range instances {
	// 		log.Debugf("[GCP] Terminating instane: %s", inst.GetName())
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

	// 	log.Debugf("[GCP] Instance groups to terminate (%d) : [%s]", len(instanceGroupsToDelete), instanceGroupsToDelete)
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
	// 	log.Debugf("[GCP] Instances to terminate (%d): [%s]", len(instancesToDelete), instancesToDelete)
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

func (p gcpProvider) StopInstances([]*types.Instance) error {
	return errors.New("[GCP] Stop not supported")
}

func (p gcpProvider) GetAccesses() ([]*types.Access, error) {
	log.Debug("[GCP] Fetching service accounts")
	return getAccesses(p.iamClient.Projects.ServiceAccounts.List("projects/"+p.projectID), func(name string) keysListAggregator {
		return p.iamClient.Projects.ServiceAccounts.Keys.List(name)
	})
}

func (p gcpProvider) GetDatabases() ([]*types.Database, error) {
	return nil, errors.New("[GCP] Get databases is not supported")
}

type instancesListAggregator interface {
	Do(...googleapi.CallOption) (*compute.InstanceAggregatedList, error)
}

func getInstances(aggregator instancesListAggregator) ([]*types.Instance, error) {
	instances := make([]*types.Instance, 0)
	instanceList, err := aggregator.Do()
	if err != nil {
		log.Errorf("[GCP] Failed to fetch the running instances, err: %s", err.Error())
		return nil, err
	}
	log.Debugf("[GCP] Processing instances (%d): [%s]", len(instanceList.Items), instanceList.Items)
	for _, items := range instanceList.Items {
		for _, inst := range items.Instances {
			instances = append(instances, newInstance(inst))
		}
	}
	return instances, nil
}

type serviceAccountsListAggregator interface {
	Do(opts ...googleapi.CallOption) (*iam.ListServiceAccountsResponse, error)
}

type keysListAggregator interface {
	Do(opts ...googleapi.CallOption) (*iam.ListServiceAccountKeysResponse, error)
}

func getAccesses(serviceAccountAggregator serviceAccountsListAggregator, getKeysAggregator func(string) keysListAggregator) ([]*types.Access, error) {
	accounts, err := serviceAccountAggregator.Do()
	if err != nil {
		return nil, err
	}
	log.Debugf("[GCP] Processing service accounts (%d): [%s]", len(accounts.Accounts), accounts.Accounts)
	now := time.Now()
	var accesses []*types.Access
	for _, account := range accounts.Accounts {
		log.Debugf("[GCP] Fetching keys for: %s", account.Name)
		keys, err := getKeysAggregator(account.Name).Do()
		if err != nil {
			return nil, err
		}
		log.Debugf("[GCP] Processing keys of %s (%d): [%s]", account.Name, len(keys.Keys), keys.Keys)
		for _, key := range keys.Keys {
			validBefore, err := utils.ConvertTimeRFC3339(key.ValidBeforeTime)
			if err != nil {
				return nil, err
			} else if now.After(validBefore) {
				log.Debugf("[GCP] Key already expired: %s", key.Name)
				continue
			}
			validAfter, err := utils.ConvertTimeRFC3339(key.ValidAfterTime)
			if err != nil {
				return nil, err
			}
			accesses = append(accesses, &types.Access{
				CloudType: types.GCP,
				Name:      key.Name,
				Owner:     account.Email,
				Created:   validAfter,
			})
		}
	}
	return accesses, nil
}

// func getRegions(p gcpProvider) ([]string, error) {
// 		log.Debug("[GCP] Fetching regions")
// 	regionList, err := p.computeClient.Regions.List(p.projectId).Do()
// 	if err != nil {
// 		return nil, err
// 	}
// 		log.Debugf("[GCP] Processing regions (%d): [%s]", len(regionList.Items), regionList.Items)
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
		Name:         inst.Name,
		ID:           strconv.Itoa(int(inst.Id)),
		Created:      created,
		CloudType:    types.GCP,
		Tags:         inst.Labels,
		Owner:        inst.Labels[ctx.GcpOwnerLabel],
		Metadata:     map[string]string{"zone": getZone(inst.Zone)},
		Region:       getRegionFromZoneURL(&inst.Zone),
		InstanceType: inst.MachineType[strings.LastIndex(inst.MachineType, "/")+1:],
		State:        getInstanceState(inst),
	}
}

// Possible values:
//   "PROVISIONING"
//   "RUNNING"
//   "STAGING"
//   "STOPPED"
//   "STOPPING"
//   "SUSPENDED"
//   "SUSPENDING"
//   "TERMINATED"
func getInstanceState(instance *compute.Instance) types.State {
	switch instance.Status {
	case "PROVISIONING", "RUNNING", "STAGING":
		return types.Running
	case "STOPPED", "STOPPING", "SUSPENDED", "SUSPENDING":
		return types.Stopped
	case "TERMINATED":
		return types.Terminated
	default:
		return types.Unknown
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
