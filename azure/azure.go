package azure

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/hortonworks/cloud-haunter/utils"

	"sync"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2017-12-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2015-11-01/subscriptions"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
)

var provider = azureProvider{}

type azureProvider struct {
	subscriptionID     string
	vmClient           compute.VirtualMachinesClient
	subscriptionClient subscriptions.Client
	// rgClient       resources.GroupsClient
	// resClient      resources.Client
}

func init() {
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if len(subscriptionID) == 0 {
		log.Warn("[AZURE] AZURE_SUBSCRIPTION_ID environment variable is missing")
		return
	}
	ctx.CloudProviders[types.AZURE] = func() types.CloudProvider {
		if len(provider.vmClient.SubscriptionID) == 0 {
			log.Debug("[AZURE] Trying to prepare")
			if err := provider.init(subscriptionID, auth.NewAuthorizerFromEnvironment); err != nil {
				panic("[AZURE] Failed to initialize provider: " + err.Error())
			}
			log.Info("[AZURE] Successfully prepared")
		}
		return provider
	}
}

func (p *azureProvider) init(subscriptionID string, authorizer func() (autorest.Authorizer, error)) error {
	authorization, err := authorizer()
	if err != nil {
		return errors.New("Failed to authenticate, err: " + err.Error())
	}
	p.subscriptionID = subscriptionID
	p.vmClient = compute.NewVirtualMachinesClient(subscriptionID)
	p.vmClient.Authorizer = authorization
	p.subscriptionClient = subscriptions.NewClient()
	p.subscriptionClient.Authorizer = authorization
	return nil
}

func (p azureProvider) GetAccountName() string {
	if result, err := p.subscriptionClient.Get(context.Background(), p.subscriptionID); err != nil {
		log.Errorf("[AZURE] Failed to retrieve subscription info, err: %s", err.Error())
	} else {
		displayName := result.DisplayName
		if displayName != nil {
			return *displayName
		}
	}
	return p.subscriptionID
}

func (p azureProvider) GetInstances() ([]*types.Instance, error) {
	log.Debug("[AZURE] Fetching instances")
	result, err := p.vmClient.ListAll(context.Background())
	if err != nil {
		log.Errorf("[AZURE] Failed to fetch the instances, err: %s", err.Error())
		return nil, err
	}

	wg := sync.WaitGroup{}
	wg.Add(len(result.Values()))
	instances := make([]azureInstance, 0)
	instanceChan := make(chan azureInstance)

	for _, vm := range result.Values() {
		go func(vm compute.VirtualMachine) {
			defer wg.Done()

			resourceGroupName := getResourceGroupName(*vm.ID)
			instanceName := *vm.Name
			if len(resourceGroupName) != 0 {
				log.Debugf("[AZURE] Fetch instance view of instance: %s", instanceName)
				if viewResult, err := p.vmClient.InstanceView(context.Background(), resourceGroupName, instanceName); err != nil {
					log.Errorf("[AZURE] Failed to fetch the instance view of instance %s, err: %s", instanceName, err.Error())
				} else {
					instanceChan <- azureInstance{vm, viewResult, resourceGroupName}
				}
			} else {
				log.Errorf("[AZURE] Failed to find the resource group and name of instance: %s", instanceName)
			}
		}(vm)
	}

	go func() {
		wg.Wait()
		close(instanceChan)
	}()

	for inst := range instanceChan {
		instances = append(instances, inst)
	}

	return convertVmsToInstances(instances)
}

type azureInstance struct {
	instance          compute.VirtualMachine
	instanceView      compute.VirtualMachineInstanceView
	resourceGroupName string
}

func (p azureProvider) TerminateInstances([]*types.Instance) []error {
	return []error{errors.New("[AZURE] Termination not supported")}
	// rgClient = resources.NewGroupsClient(subscriptionId)
	// 	rgClient.Authorizer = authorization
	// 	resClient = resources.NewClient(subscriptionId)
	// 	resClient.Authorizer = authorization
	// instances := make([]*types.Instance, 0)
	// groups, err := rgClient.List(ctx.Background(), "", nil)
	// if err != nil {
	// 	log.Errorf("[AZURE] Failed to fetch the existing resource groups, err: %s", err.Error())
	// 	return nil, err
	// }
	// typesToCollect =: map[string]bool{"Microsoft.Compute/virtualMachines": true}
	// for _, g := range groups.Values() {
	// 	resources, err := resClient.ListByResourceGroup(ctx.Background(), *g.Name, "", "", nil)
	// 	if err != nil {
	// 		log.Warn("[AZURE] Failed to fetch the resources for %s, err: %s", *g.Name, err.Error())
	// 		continue
	// 	}
	// 	for _, r := range resources.Values() {
	// 		if _, ok := typesToCollect[*r.Type]; ok {
	// 			if _, ok := r.Tags["Owner"]; !ok {
	// 				instances = append(instances, &types.Instance{
	// 					Name:      *r.Name,
	// 					Id:        *r.ID,
	// 					CloudType: types.AZURE,
	// 					Tags:      utils.ConvertTags(r.Tags),
	// 				})
	// 			}
	// 		}
	// 	}
	// }

	// return instances, nil
}

func (p azureProvider) StopInstances(instances []*types.Instance) []error {
	log.Debugf("[AZURE] Stopping instances (%d): %s", len(instances), instances)
	wg := sync.WaitGroup{}
	wg.Add(len(instances))
	errChan := make(chan error)

	for _, i := range instances {
		go func(instance *types.Instance) {
			defer wg.Done()

			log.Debugf("[AZURE] Stopping instance: %s", instance.Name)
			_, err := p.vmClient.Deallocate(context.Background(), instance.Metadata["resourceGroupName"], instance.Name)
			if err != nil {
				errChan <- err
			} else {
				log.Debugf("[AZURE] Instance stopped: %s", instance.Name)
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	errors := []error{}
	for err := range errChan {
		errors = append(errors, err)
	}
	return errors
}

func (p azureProvider) GetAccesses() ([]*types.Access, error) {
	return nil, errors.New("[AZURE] Access not supported")
}

func (p azureProvider) GetDatabases() ([]*types.Database, error) {
	return nil, errors.New("[AZURE] Get databases is not supported")
}

func convertVmsToInstances(instances []azureInstance) ([]*types.Instance, error) {
	converted := make([]*types.Instance, 0)
	for _, inst := range instances {
		newInstance := newInstance(inst, getCreationTimeFromTags, utils.ConvertTags)
		log.Debugf("[AZURE] Converted instance: %s", newInstance)
		converted = append(converted, newInstance)
	}
	return converted, nil
}

func getResourceGroupName(vmID string) (resourceGroupName string) {
	instanceIDParts := strings.Split(vmID, "/")
	// /subscriptions/<sub_id>/resourceGroups/<rg_name>/providers/Microsoft.Compute/virtualMachines/<inst_name>
	if len(instanceIDParts) == 9 {
		resourceGroupName = instanceIDParts[4]
	}
	return
}

func newInstance(inst azureInstance, getCreationTimeFromTags getCreationTimeFromTagsFuncSignature, convertTags func(map[string]*string) types.Tags) *types.Instance {
	tags := convertTags(inst.instance.Tags)
	return &types.Instance{
		Name:         *inst.instance.Name,
		ID:           *inst.instance.ID,
		Created:      getCreationTimeFromTags(tags, utils.ConvertTimeUnix),
		CloudType:    types.AZURE,
		Tags:         tags,
		Owner:        tags[ctx.AzureOwnerLabel],
		Region:       *inst.instance.Location,
		InstanceType: string(inst.instance.HardwareProfile.VMSize),
		State:        getInstanceState(inst.instanceView),
		Metadata:     map[string]string{"resourceGroupName": inst.resourceGroupName},
	}
}

// Possible values:
//   "PowerState/deallocated"
//   "PowerState/deallocating"
//   "PowerState/running"
//   "PowerState/starting"
//   "PowerState/stopped"
//   "PowerState/stopping"
//   "PowerState/unknown"
// The assumption is that the status without time is the currently active status
func getInstanceState(view compute.VirtualMachineInstanceView) types.State {
	var actualStatus *compute.InstanceViewStatus
	for _, v := range *view.Statuses {
		if v.Time == nil {
			actualStatus = &v
			break
		}
	}
	if actualStatus != nil {
		switch *actualStatus.Code {
		case "PowerState/deallocated", "PowerState/deallocating":
			return types.Stopped
		case "PowerState/running", "PowerState/starting":
			return types.Running
		case "PowerState/stopped", "PowerState/stopping":
			return types.Stopped
		default:
			return types.Unknown
		}
	}
	return types.Unknown
}

type getCreationTimeFromTagsFuncSignature func(types.Tags, func(unixTimestamp string) time.Time) time.Time

func getCreationTimeFromTags(tags types.Tags, convertTimeUnix func(unixTimestamp string) time.Time) time.Time {
	if creationTimestamp, ok := tags[ctx.AzureCreationTimeLabel]; ok {
		return convertTimeUnix(creationTimestamp)
	}
	return convertTimeUnix("0")
}
