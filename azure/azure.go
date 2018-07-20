package azure

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/hortonworks/cloud-haunter/utils"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2017-12-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"strings"
	"sync"
)

var provider = azureProvider{}

type azureProvider struct {
	subscriptionID string
	vmClient       compute.VirtualMachinesClient
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
			log.Infof("[AZURE] Trying to prepare")
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
	return nil
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
	vms := make([]azureInstance, 0)
	instanceChan := make(chan azureInstance)

	for _, vm := range result.Values() {
		go func(vm compute.VirtualMachine) {
			defer wg.Done()

			instanceIdParts := strings.Split(*vm.ID, "/")
			// /subscriptions/<sub_id>/resourceGroups/<rg_name>/providers/Microsoft.Compute/virtualMachines/<inst_name>
			if len(instanceIdParts) == 9 {
				resourceGroupName := instanceIdParts[4]
				instanceName := instanceIdParts[8]
				log.Debugf("[AZURE] Fetch instance view of instance: %s", instanceName)
				viewResult, err := p.vmClient.InstanceView(context.Background(), resourceGroupName, instanceName)
				if err != nil {
					log.Errorf("[AZURE] Failed to fetch the instance view of instance %s, err: %s", instanceName, err.Error())
				} else {
					instanceChan <- azureInstance{instance: vm, instanceView: viewResult}
				}
			} else {
				log.Errorf("[AZURE] Failed to find the resource group and name of instance: %s", *vm.Name)
			}
		}(vm)
	}

	go func() {
		wg.Wait()
		close(instanceChan)
	}()

	for inst := range instanceChan {
		vms = append(vms, inst)
	}

	return convertVmsToInstances(vms)
}

type azureInstance struct {
	instance     compute.VirtualMachine
	instanceView compute.VirtualMachineInstanceView
}

func (p azureProvider) TerminateInstances([]*types.Instance) error {
	return errors.New("[AZURE] Termination not supported")
	// AZURE
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
	// 	resources, err := resClient.ListByResourceGroup(ctx.Background(), *g.Name, "", "", nil) // TODO maybe we can filter for (running) instances
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

func (p azureProvider) StopInstances([]*types.Instance) error {
	return errors.New("[AZURE] Stop not supported")
}

func (p azureProvider) GetAccesses() ([]*types.Access, error) {
	return nil, errors.New("[AZURE] Access not supported")
}

func convertVmsToInstances(vms []azureInstance) ([]*types.Instance, error) {
	instances := make([]*types.Instance, 0)
	for _, inst := range vms {
		newInstance := newInstance(inst.instance, inst.instanceView, getCreationTimeFromTags, utils.ConvertTags)
		log.Debugf("[AZURE] Converted instance: %s", newInstance)
		instances = append(instances, newInstance)
	}
	return instances, nil
}

func newInstance(inst compute.VirtualMachine, view compute.VirtualMachineInstanceView,
	getCreationTimeFromTags getCreationTimeFromTagsFuncSignature, convertTags func(map[string]*string) types.Tags) *types.Instance {
	tags := convertTags(inst.Tags)
	return &types.Instance{
		Name:         *inst.Name,
		Id:           *inst.ID,
		Created:      getCreationTimeFromTags(tags, utils.ConvertTimeUnix),
		CloudType:    types.AZURE,
		Tags:         tags,
		Owner:        tags[ctx.AzureOwnerLabel],
		Region:       *inst.Location,
		InstanceType: string(inst.HardwareProfile.VMSize),
		State:        getInstanceState(view),
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
func getInstanceState(view compute.VirtualMachineInstanceView) types.InstanceState {
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
