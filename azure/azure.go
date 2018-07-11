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

func (p azureProvider) GetRunningInstances() ([]*types.Instance, error) {
	log.Debug("[AZURE] Fetching instances")
	result, err := p.vmClient.ListAll(context.Background())
	if err != nil {
		log.Errorf("[AZURE] Failed to fetch the running instances, err: %s", err.Error())
		return nil, err
	}
	return convertVmsToInstances(result)
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

func (p azureProvider) GetAccesses() ([]*types.Access, error) {
	return nil, errors.New("[AZURE] Access not supported")
}

type hasValues interface {
	Values() []compute.VirtualMachine
}

func convertVmsToInstances(vms hasValues) ([]*types.Instance, error) {
	instances := make([]*types.Instance, 0)
	for _, inst := range vms.Values() {
		newInstance := newInstance(inst, getCreationTimeFromTags, utils.ConvertTags)
		log.Debugf("[AZURE] Converted instance: %s", newInstance)
		instances = append(instances, newInstance)
	}
	return instances, nil
}

func newInstance(inst compute.VirtualMachine, getCreationTimeFromTags getCreationTimeFromTagsFuncSignature, convertTags func(map[string]*string) types.Tags) *types.Instance {
	tags := convertTags(inst.Tags)
	return &types.Instance{
		Name:      *inst.Name,
		Id:        *inst.ID,
		Created:   getCreationTimeFromTags(tags, utils.ConvertTimeUnix),
		CloudType: types.AZURE,
		Tags:      tags,
		Owner:     tags[ctx.AzureOwnerLabel],
		Region:    *inst.Location,
	}
}

type getCreationTimeFromTagsFuncSignature func(types.Tags, func(unixTimestamp string) time.Time) time.Time

func getCreationTimeFromTags(tags types.Tags, convertTimeUnix func(unixTimestamp string) time.Time) time.Time {
	if creationTimestamp, ok := tags[ctx.AzureCreationTimeLabel]; ok {
		return convertTimeUnix(creationTimestamp)
	}
	return convertTimeUnix("0")
}
