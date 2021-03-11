package gcp

import (
	"errors"
	"fmt"
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

/**
 * In GCP currently we create resources one-by-one (without using a template), so stacks have to be assembled from resources.
 * We define a stack for each {ResourceGroupingLabel} label value of found instances:
 * - Add instances with the same tag to stack
 * - Add database with the same tag to stack
 * - Filter networks by their description (if provided in {ResourceDescription} config), and add them to stack by the instance's network interface in stack
 * - Add the network's firewall and subnets to stack
 * - Add external IPs as referenced by instances in stack
 * Then add references to these resources in the MetaData property of the stack, so later we can reference them.
 */
func (p gcpProvider) GetStacks() ([]*types.Stack, error) {
	databases, err := p.GetDatabases()
	if err != nil {
		log.Error("[GCP] Failed to fetch databases")
		return nil, err
	}

	instanceResponse, err := p.computeClient.Instances.AggregatedList(p.projectID).Do()
	if err != nil {
		log.Error("[GCP] Failed to fetch instances")
		return nil, err
	}

	instancesByName := map[string]types.Instance{}
	gcpInstancesByName := map[string]compute.Instance{}
	for _, items := range instanceResponse.Items {
		for _, gcpInstance := range items.Instances {
			instance := newInstance(gcpInstance)
			instancesByName[instance.Name] = *instance
			gcpInstancesByName[instance.Name] = *gcpInstance
		}
	}

	networksBySelfLink, err := p.getNetworksBySelfLink()
	if err != nil {
		return nil, err
	}

	firewallsByNetwork, err := p.getFirewallsByNetwork()
	if err != nil {
		return nil, err
	}

	regions := map[string]bool{}
	for _, instance := range instancesByName {
		regions[instance.Region] = true
	}

	externalIpsByRegion := map[string][]*compute.Address{}
	for region := range regions {
		externalIps, err := p.computeClient.Addresses.List(p.projectID, region).Do()
		if err != nil {
			log.Errorf("[GCP] Failed to fetch external IPs in region %s", region)
			return nil, err
		}
		externalIpsByRegion[region] = externalIps.Items
	}

	gcpStacks := map[string]*GcpStack{}
	for instanceName, instance := range instancesByName {
		stackId, ok := instance.Tags[ctx.ResourceGroupingLabel]
		if !ok {
			log.Warnf("[GCP] Skipping instance %s as it does not have %s tag", instance.Name, ctx.ResourceGroupingLabel)
			continue
		}

		gcpInstance := gcpInstancesByName[instanceName]

		if _, ok := gcpStacks[stackId]; !ok {
			log.Debugf("[GCP] Creating stack for instance %s", instanceName)
			var stackDatabaseName string
			for _, database := range databases {
				if stackId == database.Tags[ctx.ResourceGroupingLabel] {
					log.Debugf("[GCP] Using db %s for stack %s", database.Name, stackId)
					stackDatabaseName = database.Name
					break
				}
			}
			var networkName string
			var firewallName string
			var subnetNames []string
			for _, networkInterface := range gcpInstance.NetworkInterfaces {
				if network, ok := networksBySelfLink[networkInterface.Network]; ok {
					networkName = network.Name
					firewallName = firewallsByNetwork[network.SelfLink].Name
					for _, subnet := range network.Subnetworks {
						subnetParts := strings.Split(subnet, "/")
						subnetNames = append(subnetNames, subnetParts[len(subnetParts)-1])
					}
				} else {
					log.Debugf("[GCP] No generated network found for instance %s", instanceName)
				}
			}
			newGcpStack := GcpStack{
				ID:              stackId,
				Region:          instance.Region,
				Zone:            instance.Metadata["zone"],
				InstanceNames:   []string{},
				ExternalIpNames: []string{},
				NetworkName:     networkName,
				SubnetNames:     subnetNames,
				FirewallName:    firewallName,
				Owner:           instance.Owner,
				DatabaseName:    stackDatabaseName,
				Created:         instance.Created,
				State:           instance.State,
			}
			gcpStacks[stackId] = &newGcpStack
		}
		currentGcpStack := gcpStacks[stackId]

		log.Debugf("[GCP] Adding instance %s to stack %s", instance.Name, stackId)
		currentGcpStack.InstanceNames = append(currentGcpStack.InstanceNames, instance.Name)

		if instance.State != currentGcpStack.State {
			currentGcpStack.State = types.Unknown
		}

		for _, address := range externalIpsByRegion[instance.Region] {
			for _, url := range address.Users {
				if url == gcpInstance.SelfLink {
					log.Debugf("[GCP] Adding external ip %s to stack %s", address.Name, stackId)
					currentGcpStack.ExternalIpNames = append(currentGcpStack.ExternalIpNames, address.Name)
				}
			}
		}
	}

	var stacks []*types.Stack
	for _, gcpStack := range gcpStacks {
		log.Debugf("[GCP] Converting gcpStack %+v", gcpStack)
		aStack := &types.Stack{
			ID:        gcpStack.ID,
			Name:      gcpStack.ID,
			Created:   gcpStack.Created,
			Owner:     gcpStack.Owner,
			CloudType: types.GCP,
			State:     types.Running,
			Region:    gcpStack.Region,
			Metadata: map[string]string{
				"instances":   strings.Join(gcpStack.InstanceNames, ","),
				"externalIps": strings.Join(gcpStack.ExternalIpNames, ","),
				"network":     gcpStack.NetworkName,
				"subnets":     strings.Join(gcpStack.SubnetNames, ","),
				"firewall":    gcpStack.FirewallName,
				"database":    gcpStack.DatabaseName,
				"zone":        gcpStack.Zone,
			},
		}
		stacks = append(stacks, aStack)
	}
	log.Debugf("[GCP] Collected stacks %+v", stacks)

	return stacks, nil
}

type GcpStack struct {
	ID              string
	Region          string
	Zone            string
	InstanceNames   []string
	ExternalIpNames []string
	NetworkName     string
	SubnetNames     []string
	FirewallName    string
	DatabaseName    string
	Created         time.Time
	Owner           string
	State           types.State
}

func (p gcpProvider) getNetworksBySelfLink() (map[string]*compute.Network, error) {
	networkListCall := p.computeClient.Networks.List(p.projectID)
	if ctx.ResourceDescription != "" {
		descriptionFilter := fmt.Sprintf("description = \"%s\"", ctx.ResourceDescription)
		networkListCall = networkListCall.Filter(descriptionFilter)
	}
	networkResponse, err := networkListCall.Do()
	if err != nil {
		log.Error("[GCP] Failed to fetch networks")
		return nil, err
	}

	networksBySelfLink := map[string]*compute.Network{}
	for _, network := range networkResponse.Items {
		networksBySelfLink[network.SelfLink] = network
	}
	return networksBySelfLink, nil
}

func (p gcpProvider) getFirewallsByNetwork() (map[string]*compute.Firewall, error) {
	firewallListCall := p.computeClient.Firewalls.List(p.projectID)
	if ctx.ResourceDescription != "" {
		descriptionFilter := fmt.Sprintf("description = \"%s\"", ctx.ResourceDescription)
		firewallListCall = firewallListCall.Filter(descriptionFilter)
	}
	firewallResponse, err := firewallListCall.Do()
	if err != nil {
		log.Error("[GCP] Failed to fetch firewalls")
		return nil, err
	}

	firewallsByNetwork := map[string]*compute.Firewall{}
	for _, firewall := range firewallResponse.Items {
		firewallsByNetwork[firewall.Network] = firewall
	}
	return firewallsByNetwork, nil
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
	log.Debug("[GCP] Terminating instances")

	gcpInstances := instances.Get(types.GCP)
	instanceGroups, err := p.computeClient.InstanceGroupManagers.AggregatedList(p.projectID).Do()
	if err != nil {
		log.Errorf("[GCP] Failed to fetch instance groups, err: %s", err.Error())
		return []error{err}
	}

	instancesToDelete := []*types.Instance{}
	instanceGroupsToDelete := map[*compute.InstanceGroupManager]bool{}

	for _, inst := range gcpInstances {
		groupFound := false
		for _, i := range instanceGroups.Items {
			for _, group := range i.InstanceGroupManagers {
				if _, ok := instanceGroupsToDelete[group]; !ok && strings.Index(inst.Name, group.BaseInstanceName+"-") == 0 {
					log.Debugf("[GCP] Found instance group for instance %s : %s", inst.GetName(), group.Name)
					instanceGroupsToDelete[group], groupFound = true, true
				}
			}
		}
		if !groupFound {
			log.Debugf("[GCP] Not found instance group for instance: %s", inst.GetName())
			instancesToDelete = append(instancesToDelete, inst)
		}
	}

	log.Debugf("[GCP] Instance groups to terminate (%d) : [%v]", len(instanceGroupsToDelete), instanceGroupsToDelete)
	wg := sync.WaitGroup{}
	wg.Add(len(instanceGroupsToDelete))
	for g := range instanceGroupsToDelete {
		go func(group *compute.InstanceGroupManager) {
			defer wg.Done()

			zone := getZone(group.Zone)
			log.Infof("[GCP] Deleting instance group %s in zone %s", group.Name, zone)
			if ctx.DryRun {
				log.Info("[GCP] Skipping group termination on dry run session")
			} else {
				_, err := p.computeClient.InstanceGroupManagers.Delete(p.projectID, zone, group.Name).Do()
				if err != nil {
					log.Errorf("[GCP] Failed to delete instance group %s, err: %s", group.Name, err.Error())
				}
			}
		}(g)
	}
	log.Debugf("[GCP] Instances to terminate (%d): [%v]", len(instancesToDelete), instancesToDelete)
	wg.Add(len(instancesToDelete))
	for _, i := range instancesToDelete {
		go func(inst *types.Instance) {
			defer wg.Done()

			zone := inst.Metadata["zone"]
			log.Infof("[GCP] Deleting instance %s in zone %s", inst.Name, zone)
			if ctx.DryRun {
				log.Info("[GCP] Skipping instance termination on dry run session")
			} else {
				_, err := p.computeClient.Instances.Delete(p.projectID, zone, inst.Name).Do()
				if err != nil {
					log.Errorf("[GCP] Failed to delete instance %s, err: %s", inst.Name, err.Error())
				}
			}
		}(i)
	}
	wg.Wait()
	return nil
}

func (p gcpProvider) TerminateStacks(stacks *types.StackContainer) []error {
	gcpStacks := stacks.Get(types.GCP)
	log.Debugf("[GCP] Terminating stacks: %v", gcpStacks)

	wg := sync.WaitGroup{}
	wg.Add(len(gcpStacks))
	errChan := make(chan error)

	for _, s := range gcpStacks {
		go func(stack *types.Stack) {
			defer wg.Done()

			if ctx.DryRun {
				log.Infof("[GCP] Dry-run set, stack is not terminated: %s", stack.Name)
			} else {
				log.Infof("[GCP] Terminating stack %s", stack.Name)

				if success := p.terminateInstancesAndDbInStack(stack, errChan); success {
					p.terminateNetworkRelatedResourcesInStack(stack, errChan)
				} else {
					log.Infof("[GCP] There were error(s) while terminating instances and db, skipping network termination for stack %s", stack.Name)
				}
			}
		}(s)
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

func (p gcpProvider) terminateInstancesAndDbInStack(stack *types.Stack, errChan chan error) bool {
	zone := stack.Metadata["zone"]
	instances := strings.Split(stack.Metadata["instances"], ",")
	database := stack.Metadata["database"]

	wg := sync.WaitGroup{}
	wg.Add(len(instances))
	resultChan := make(chan bool)

	for _, i := range instances {
		go func(instanceName string, zone string) {
			defer wg.Done()

			log.Debugf("[GCP] Terminating instance %s", instanceName)

			err := p.doAndPollComputeCall(p.computeClient.Instances.Delete(p.projectID, zone, instanceName))
			if err != nil {
				errChan <- err
			}
			resultChan <- err == nil
		}(i, zone)
	}

	if database != "" {
		wg.Add(1)

		go func(dbName string) {
			defer wg.Done()

			log.Debugf("[GCP] Terminating db %s", dbName)

			err := p.doAndPollSqlCall(p.sqlClient.Instances.Delete(p.projectID, dbName))
			if err != nil {
				errChan <- err
			}
			resultChan <- err == nil
		}(database)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for success := range resultChan {
		if !success {
			return false
		}
	}
	return true
}

func (p gcpProvider) terminateNetworkRelatedResourcesInStack(stack *types.Stack, errChan chan error) {
	ips := strings.Split(stack.Metadata["externalIps"], ",")
	firewall := stack.Metadata["firewall"]
	subnets := strings.Split(stack.Metadata["subnets"], ",")
	network := stack.Metadata["network"]

	wg := sync.WaitGroup{}
	wg.Add(len(ips))

	for _, i := range ips {
		go func(ip string, region string) {
			defer wg.Done()

			log.Debugf("[GCP] Terminating ip %s", ip)

			if _, err := p.computeClient.Addresses.Delete(p.projectID, region, ip).Do(); err != nil {
				errChan <- err
			}
		}(i, stack.Region)
	}

	if network != "" {
		wg.Add(1)

		go func(networkName string, firewallName string, subnetNames []string) {
			defer wg.Done()

			preNetworkWg := sync.WaitGroup{}
			preNetworkWg.Add(1 + len(subnetNames))

			go func(firewallName string) {
				defer preNetworkWg.Done()

				log.Debugf("[GCP] Terminating firewall %s", firewallName)

				err := p.doAndPollComputeCall(p.computeClient.Firewalls.Delete(p.projectID, firewallName))
				if err != nil {
					errChan <- err
					log.Infof("[GCP] Failed to delete firewall %s, skipping network delete", firewallName)
					return
				}
			}(firewallName)

			for _, s := range subnetNames {
				go func(subnetName string, region string) {
					defer preNetworkWg.Done()

					log.Debugf("[GCP] Terminating subnet %s", subnetName)

					err := p.doAndPollComputeCall(p.computeClient.Subnetworks.Delete(p.projectID, region, subnetName))
					if err != nil {
						errChan <- err
					}
				}(s, stack.Region)
			}

			preNetworkWg.Wait()

			log.Debugf("[GCP] Terminating network %s", networkName)

			err := p.doAndPollComputeCall(p.computeClient.Networks.Delete(p.projectID, networkName))
			if err != nil {
				errChan <- err
			}
		}(network, firewall, subnets)
	}

	wg.Wait()
}

type SqlCall interface {
	Do(opts ...googleapi.CallOption) (*sqladmin.Operation, error)
}

func (p gcpProvider) doAndPollSqlCall(call SqlCall) error {
	operation, err := call.Do()
	if err != nil {
		return err
	}

	for {
		time.Sleep(3 * time.Second)
		log.Debugf("[GCP] Polling sql operation %s", operation.Name)
		operation, err := p.sqlClient.Operations.Get(p.projectID, operation.Name).Do()

		if err != nil {
			return err
		} else if operation.Status == "DONE" {
			if operation.Error == nil {
				log.Debugf("[GCP] Polling of sql operation %s successfully finished", operation.Name)
				return nil
			}
			var errorMessages []string
			for _, operationError := range operation.Error.Errors {
				errorMessages = append(errorMessages, operationError.Code+": "+operationError.Message)
			}
			return fmt.Errorf("[GCP] Polling of sql operation %s failed: %s", operation.Name, strings.Join(errorMessages, ", "))
		}
	}
}

type ComputeCall interface {
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)
}

func (p gcpProvider) doAndPollComputeCall(call ComputeCall) error {
	operation, err := call.Do()
	if err != nil {
		return err
	}

	for {
		time.Sleep(3 * time.Second)
		log.Debugf("[GCP] Polling compute %s (region='%s', zone='%s')", operation.Name, operation.Region, operation.Zone)

		if operation.Zone != "" {
			zoneURLParts := strings.Split(operation.Zone, "/")
			zone := zoneURLParts[len(zoneURLParts)-1]
			operation, err = p.computeClient.ZoneOperations.Get(p.projectID, zone, operation.Name).Do()
		} else if operation.Region != "" {
			regionURLParts := strings.Split(operation.Region, "/")
			region := regionURLParts[len(regionURLParts)-1]
			operation, err = p.computeClient.RegionOperations.Get(p.projectID, region, operation.Name).Do()
		} else {
			operation, err = p.computeClient.GlobalOperations.Get(p.projectID, operation.Name).Do()
		}

		if err != nil {
			return err
		} else if operation.Status == "DONE" {
			if operation.Error == nil {
				log.Debugf("[GCP] Polling of compute %s successfully finished", operation.Name)
				return nil
			}
			var errorMessages []string
			for _, operationError := range operation.Error.Errors {
				errorMessages = append(errorMessages, operationError.Code+": "+operationError.Message)
			}
			return fmt.Errorf("[GCP] Polling of compute %s failed: %s", operation.Name, strings.Join(errorMessages, ", "))
		}
	}
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

func (p gcpProvider) StopDatabases(databases *types.DatabaseContainer) []error {
	gcpDatabases := databases.Get(types.GCP)
	log.Debugf("[GCP] Stopping databases: %v", gcpDatabases)

	wg := sync.WaitGroup{}
	wg.Add(len(gcpDatabases))
	errChan := make(chan error)

	stopRequest := &sqladmin.DatabaseInstance{
		Settings: &sqladmin.Settings{
			ActivationPolicy: "NEVER",
		},
	}

	for _, db := range gcpDatabases {
		go func(database *types.Database) {
			defer wg.Done()

			if ctx.DryRun {
				log.Infof("[GCP] Dry-run set, instance is not stopped: %s", database.Name)
			} else {
				log.Infof("[GCP] Sending request to stop instance %s", database.Name)

				if _, err := p.sqlClient.Instances.Patch(p.projectID, database.Name, stopRequest).Do(); err != nil {
					errChan <- err
				}
			}
		}(db)
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
				log.Errorf("[GCP] Failed to get the creation timestamp of disk, err: %s", err.Error())
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
			log.Errorf("[GCP] Failed to get the creation timestamp of the DB: %s, err: %s", instanceName, err.Error())
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
			Tags:         tags,
		}
		databases = append(databases, aDisk)
	}
	return databases, nil
}

func getDatabaseInstanceCreationTimeStamp(opService *sqladmin.OperationsListCall, dbName string) (time.Time, error) {
	operationsList, err := opService.Do()
	if err != nil {
		log.Errorf("[GCP] Failed to get the operations of the DB instance: %s, err: %s", dbName, err.Error())
		return time.Time{}, err
	}
	for _, operation := range operationsList.Items {
		if operation.OperationType == "CREATE" {
			if len(operation.EndTime) > 0 {
				return utils.ConvertTimeRFC3339(operation.EndTime)
			} else {
				log.Infof("[GCP] the creation timestamp is empty of the DB instance: %s, use the current time", dbName)
				return time.Now(), nil
			}
		}
	}
	return time.Time{}, errors.New(fmt.Sprintf("[GCP] Failed to get the CREATE operation of the DB instance: %s", dbName))
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
