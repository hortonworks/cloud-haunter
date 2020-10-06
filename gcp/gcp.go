package gcp

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
	log "github.com/sirupsen/logrus"

	"context"
	"strconv"
	"sync"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

var provider = gcpProvider{}

type gcpProvider struct {
	projectID     string
	computeClient *compute.Service
	iamClient     *iam.Service
	sqlClient     *sqladmin.Service
}

func init() {
	projectID := os.Getenv("GOOGLE_PROJECT_ID")
	if len(projectID) == 0 {
		log.Warn("[GCP] GOOGLE_PROJECT_ID environment variable is missing")
		return
	}
	applicationCredentials := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if len(applicationCredentials) == 0 {
		log.Warn("[GCP] GOOGLE_APPLICATION_CREDENTIALS environment variable is missing")
		return
	}
	ctx.CloudProviders[types.GCP] = func() types.CloudProvider {
		if len(provider.projectID) == 0 {
			log.Debug("[GCP] Trying to prepare")
			computeClient, iamClient, sqlClient, err := initClients()
			if err != nil {
				panic("[GCP] Failed to authenticate, err: " + err.Error())
			}
			if err := provider.init(projectID, computeClient, iamClient, sqlClient); err != nil {
				panic("[GCP] Failed to initialize provider, err: " + err.Error())
			}
			log.Info("[GCP] Successfully prepared")
		}
		return provider
	}
}

func initClients() (computeClient *http.Client, iamClient *http.Client, sqlClient *http.Client, err error) {
	computeClient, err = google.DefaultClient(context.Background(), compute.CloudPlatformScope)
	if err != nil {
		return
	}
	iamClient, err = google.DefaultClient(context.Background(), iam.CloudPlatformScope)
	if err != nil {
		return
	}
	sqlClient, err = google.DefaultClient(context.Background(), sqladmin.SqlserviceAdminScope)
	if err != nil {
		return
	}
	return
}

func (p *gcpProvider) init(projectID string, computeHTTPClient *http.Client, iamHTTPClient *http.Client,
	sqlHTTPClient *http.Client) error {

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
	sqlClient, err := sqladmin.New(sqlHTTPClient)
	if err != nil {
		return errors.New("Failed to initialize Sql admin client, err: " + err.Error())
	}
	p.sqlClient = sqlClient
	return nil
}

func (p gcpProvider) GetAccountName() string {
	return p.projectID
}

func (p gcpProvider) GetInstances() ([]*types.Instance, error) {
	log.Debug("[GCP] Fetching instances")
	return getInstances(p.computeClient.Instances.AggregatedList(p.projectID))
}

func (p gcpProvider) GetStacks() ([]*types.Stack, error) {
	return nil, errors.New("[GCP] Get stacks is not supported")
}

func (p gcpProvider) GetDisks() ([]*types.Disk, error) {
	log.Debug("[GCP] Fetching disks")
	return p.getDisks()
}

func (p gcpProvider) DeleteDisks(disks *types.DiskContainer) []error {
	gcpDisks := disks.Get(types.GCP)
	log.Debugf("[GCP] Deleting disks: %v", gcpDisks)

	wg := sync.WaitGroup{}
	wg.Add(len(gcpDisks))
	errChan := make(chan error)

	for _, d := range gcpDisks {
		go func(disk *types.Disk) {
			defer wg.Done()

			if ctx.DryRun {
				log.Infof("[GCP] Dry-run set, disk is not deleted: %s", disk.Name)
			} else {
				zone := disk.Metadata["zone"]
				log.Infof("[GCP] Sending request to delete disk in zone %s : %s", zone, disk.Name)

				if _, err := p.computeClient.Disks.Delete(p.projectID, zone, disk.Name).Do(); err != nil {
					errChan <- err
				}
			}
		}(d)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	return errs
}

func (p gcpProvider) GetImages() ([]*types.Image, error) {
	log.Debug("[GCP] Fetching images")
	return getImages(p.computeClient.Images.List(p.projectID))
}

func (p gcpProvider) DeleteImages(images *types.ImageContainer) []error {
	log.Debug("[GCP] Deleting images")
	getAggregator := func(image string) imageDeleteAggregator {
		return p.computeClient.Images.Delete(p.projectID, image)
	}
	return deleteImages(getAggregator, images.Get(types.GCP))
}

func (p gcpProvider) TerminateInstances(instances *types.InstanceContainer) []error {
	return []error{errors.New("[GCP] Termination is not supported")}
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

func (p gcpProvider) TerminateStacks(*types.StackContainer) []error {
	return []error{errors.New("[GCP] Termination is not supported")}
}

func (p gcpProvider) StopInstances(instances *types.InstanceContainer) []error {
	gcpInstances := instances.Get(types.GCP)
	log.Debugf("[GCP] Stopping instances: %v", gcpInstances)

	wg := sync.WaitGroup{}
	wg.Add(len(gcpInstances))
	errChan := make(chan error)

	for _, i := range gcpInstances {
		go func(instance *types.Instance) {
			defer wg.Done()

			if ctx.DryRun {
				log.Infof("[GCP] Dry-run set, instance is not stopped: %s", instance.Name)
			} else {
				zone := instance.Metadata["zone"]
				log.Infof("[GCP] Sending request to stop instance in zone %s : %s", zone, instance.Name)

				if _, err := p.computeClient.Instances.Stop(p.projectID, zone, instance.Name).Do(); err != nil {
					errChan <- err
				}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	return errs
}

func (p gcpProvider) StopDatabases(*types.DatabaseContainer) []error {
	return []error{errors.New("[GCP] Not implemented")}
}

func (p gcpProvider) GetAccesses() ([]*types.Access, error) {
	log.Debug("[GCP] Fetching service accounts")
	return getAccesses(p.iamClient.Projects.ServiceAccounts.List("projects/"+p.projectID), func(name string) keysListAggregator {
		return p.iamClient.Projects.ServiceAccounts.Keys.List(name)
	})
}

func (p gcpProvider) GetDatabases() ([]*types.Database, error) {
	log.Debug("[GCP] Fetching database instances")
	aggregator := p.sqlClient.Instances.List(p.projectID)
	return p.getDatabases(aggregator)
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
	log.Debugf("[GCP] Processing instances (%d): [%v]", len(instanceList.Items), instanceList.Items)
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

type imageListAggregator interface {
	Do(opts ...googleapi.CallOption) (*compute.ImageList, error)
}

func getImages(listImages imageListAggregator) ([]*types.Image, error) {
	imagesResponse, err := listImages.Do()
	if err != nil {
		return nil, err
	}
	items := imagesResponse.Items
	log.Debugf("[GCP] Processing images (%d): [%v]", len(items), items)
	images := make([]*types.Image, 0)
	for _, image := range items {
		images = append(images, newImage(image))
	}
	return images, nil
}

type imageDeleteAggregator interface {
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)
}

func deleteImages(getAggregator func(string) imageDeleteAggregator, images []*types.Image) []error {
	wg := sync.WaitGroup{}
	wg.Add(len(images))
	errChan := make(chan error)
	sem := make(chan bool, 10)

	for _, image := range images {
		parts := strings.Split(image.ID, "/")
		ID := strings.Replace(parts[len(parts)-1], ".tar.gz", "", 1)
		go func() {
			sem <- true
			defer func() {
				wg.Done()
				<-sem
			}()

			if ctx.DryRun {
				log.Infof("[GCP] Dry-run set, image is not deleted: %s", ID)
			} else {
				log.Infof("[GCP] Delete image: %s", ID)
				_, err := getAggregator(ID).Do()
				if err != nil {
					log.Errorf("[GCP] Unable to delete image: %s because: %s", ID, err.Error())
					errChan <- errors.New(ID)
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	return errs
}

func getAccesses(serviceAccountAggregator serviceAccountsListAggregator, getKeysAggregator func(string) keysListAggregator) ([]*types.Access, error) {
	accounts, err := serviceAccountAggregator.Do()
	if err != nil {
		return nil, err
	}
	log.Debugf("[GCP] Processing service accounts (%d): [%v]", len(accounts.Accounts), accounts.Accounts)
	now := time.Now()
	var accesses []*types.Access
	for _, account := range accounts.Accounts {
		log.Debugf("[GCP] Fetching keys for: %s", account.Name)
		keys, err := getKeysAggregator(account.Name).Do()
		if err != nil {
			return nil, err
		}
		log.Debugf("[GCP] Processing keys of %s (%d): [%v]", account.Name, len(keys.Keys), keys.Keys)
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

func newImage(image *compute.Image) *types.Image {
	created, err := utils.ConvertTimeRFC3339(image.CreationTimestamp)
	if err != nil {
		log.Warnf("[GCP] cannot convert time: %s, err: %s", image.CreationTimestamp, err.Error())
	}
	return &types.Image{
		Name:      image.Name,
		ID:        strconv.Itoa(int(image.Id)),
		Created:   created,
		CloudType: types.GCP,
		Region:    "",
	}
}

func newInstance(inst *compute.Instance) *types.Instance {
	created, err := utils.ConvertTimeRFC3339(inst.CreationTimestamp)
	if err != nil {
		log.Warnf("[GCP] cannot convert time: %s, err: %s", inst.CreationTimestamp, err.Error())
	}
	tags := convertTags(inst.Labels)
	return &types.Instance{
		Name:         inst.Name,
		ID:           strconv.Itoa(int(inst.Id)),
		Created:      created,
		CloudType:    types.GCP,
		Tags:         tags,
		Owner:        tags[ctx.OwnerLabel],
		Metadata:     map[string]string{"zone": getZone(inst.Zone)},
		Region:       getRegionFromZoneURL(&inst.Zone),
		InstanceType: inst.MachineType[strings.LastIndex(inst.MachineType, "/")+1:],
		State:        getInstanceState(inst),
	}
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

func convertTags(tags map[string]string) map[string]string {
	result := make(map[string]string, 0)
	for k, v := range tags {
		result[strings.ToLower(k)] = v
	}
	return result
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

func (p gcpProvider) getDisks() ([]*types.Disk, error) {
	aggregator := p.computeClient.Disks.AggregatedList(p.projectID)
	disks := make([]*types.Disk, 0)
	diskList, err := aggregator.Do()
	if err != nil {
		log.Errorf("[GCP] Failed to fetch the available disks, err: %s", err.Error())
		return nil, err
	}
	log.Debugf("[GCP] Processing disks (%d): [%v]", len(diskList.Items), diskList.Items)
	for _, items := range diskList.Items {
		for _, gDisk := range items.Disks {
			creationTimeStamp, err := utils.ConvertTimeRFC3339(gDisk.CreationTimestamp)
			if err != nil {
				log.Errorf("[GCP] Failed to creation timestamp of disk, err: %s", err.Error())
				return nil, err
			}
			tags := convertTags(gDisk.Labels)
			aDisk := &types.Disk{
				CloudType: types.GCP,
				ID:        strconv.Itoa(int(gDisk.Id)),
				Name:      gDisk.Name,
				Region:    getRegionFromZoneURL(&gDisk.Zone),
				Created:   creationTimeStamp,
				Size:      gDisk.SizeGb,
				Type:      gDisk.Type,
				State:     getDiskStatus(gDisk),
				Owner:     tags[ctx.OwnerLabel],
				Metadata:  map[string]string{"zone": getZone(gDisk.Zone)},
			}
			disks = append(disks, aDisk)
		}
	}
	return disks, nil
}

func (p gcpProvider) getDatabases(aggregator *sqladmin.InstancesListCall) ([]*types.Database, error) {
	databases := make([]*types.Database, 0)
	gDatabaseList, err := aggregator.Do()
	if err != nil {
		log.Errorf("[GCP] Failed to fetch the database instances, err: %s", err.Error())
		return nil, err
	}
	log.Debugf("[GCP] Processing database instances (%d): [%v]", len(gDatabaseList.Items), gDatabaseList.Items)
	for _, databaseInstance := range gDatabaseList.Items {
		instanceName := databaseInstance.Name

		listOperationCall := p.sqlClient.Operations.List(p.projectID, instanceName)
		creationTimeStamp, err := getDatabaseInstanceCreationTimeStamp(listOperationCall, instanceName)
		if err != nil {
			log.Errorf("[GCP] Failed to get creation timestamp of disk, err: %s", err.Error())
			return nil, err
		}
		tags := convertTags(databaseInstance.Settings.UserLabels)
		aDisk := &types.Database{
			CloudType:    types.GCP,
			ID:           databaseInstance.Etag,
			Name:         instanceName,
			Region:       databaseInstance.GceZone,
			Created:      creationTimeStamp,
			State:        getDatabaseInstanceStatus(databaseInstance),
			Owner:        tags[ctx.OwnerLabel],
			InstanceType: databaseInstance.InstanceType,
		}
		databases = append(databases, aDisk)
	}
	return databases, nil
}

func getDatabaseInstanceCreationTimeStamp(opService *sqladmin.OperationsListCall, instanceName string) (time.Time, error) {
	operationsList, err := opService.Do()
	if err != nil {
		log.Errorf("[GCP] Failed to get operations of database(%s) instance, err: %s", instanceName, err.Error())
		return time.Time{}, err
	}
	for _, operation := range operationsList.Items {
		if operation.OperationType == "CREATE" {
			return utils.ConvertTimeRFC3339(operation.EndTime)
		}
	}
	return time.Time{}, errors.New("[GCP] Failed to get CREATE operation for database instance")
}

//Possible values:
//CREATING: Disk is provisioning.
//RESTORING: Source data is being copied into the disk.
//FAILED: Disk creation failed.
//READY: Disk is ready for use.
//DELETING: Disk is deleting.
func getDiskStatus(gDisk *compute.Disk) types.State {
	switch gDisk.Status {
	case "CREATING", "RESTORING", "STAGING", "READY", "FAILED":
		return types.Running
	case "DELETING":
		return types.Terminated
	default:
		return types.Unknown
	}
}

//SQL_INSTANCE_STATE_UNSPECIFIED	The state of the instance is unknown.
//RUNNABLE	The instance is running.
//SUSPENDED	The instance is currently offline, but it may run again in the future.
//PENDING_DELETE	The instance is being deleted.
//PENDING_CREATE	The instance is being created.
//MAINTENANCE	The instance is down for maintenance.
//FAILED	The instance failed to be created.
func getDatabaseInstanceStatus(instance *sqladmin.DatabaseInstance) types.State {
	switch instance.State {
	case "RUNNABLE", "SUSPENDED", "PENDING_CREATE", "MAINTENANCE", "FAILED":
		return types.Running
	case "PENDING_DELETE":
		return types.Terminated
	default:
		return types.Unknown

	}
}
