package azure

import (
	ctx "context"
	"os"

	"github.com/hortonworks/cloud-cost-reducer/utils"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2017-12-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2017-05-10/resources"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

var (
	subscriptionId string
	vmClient       compute.VirtualMachinesClient
	rgClient       resources.GroupsClient
	resClient      resources.Client
)

var typesToCollect = map[string]bool{"Microsoft.Compute/virtualMachines": true}

func init() {
	subscriptionId = os.Getenv("AZURE_SUBSCRIPTION_ID")
	if len(subscriptionId) > 0 {
		log.Infof("[AZURE] Trying to register as provider")
		authorization, err := auth.NewAuthorizerFromEnvironment()
		if err != nil {
			log.Errorf("[AZURE] Failed to authenticate, err: %s", err.Error())
			return
		}
		vmClient = compute.NewVirtualMachinesClient(subscriptionId)
		vmClient.Authorizer = authorization
		rgClient = resources.NewGroupsClient(subscriptionId)
		rgClient.Authorizer = authorization
		resClient = resources.NewClient(subscriptionId)
		resClient.Authorizer = authorization

		context.CloudProviders[types.AZURE] = new(AzureProvider)
		log.Info("[AZURE] Successfully registered as provider")
	} else {
		log.Warn("[AZURE] AZURE_SUBSCRIPTION_ID environment variable is missing")
	}
}

type AzureProvider struct {
}

func (p *AzureProvider) GetRunningInstances() ([]*types.Instance, error) {
	instances := make([]*types.Instance, 0)
	result, err := vmClient.ListAll(ctx.Background())
	if err != nil {
		log.Errorf("[AZURE] Failed to fetch the running instances, err: %s", err.Error())
		return nil, err
	}
	for _, inst := range result.Values() {
		instances = append(instances, &types.Instance{
			Name:      *inst.Name,
			Id:        *inst.ID,
			CloudType: types.AZURE,
			Tags:      utils.ConvertTags(inst.Tags),
		})
	}
	return instances, nil
}

func (a AzureProvider) GetOwnerLessInstances() ([]*types.Instance, error) {
	instances := make([]*types.Instance, 0)
	groups, err := rgClient.List(ctx.Background(), "", nil)
	if err != nil {
		log.Errorf("[AZURE] Failed to fetch the existing resource groups, err: %s", err.Error())
		return nil, err
	}
	for _, g := range groups.Values() {
		resources, err := resClient.ListByResourceGroup(ctx.Background(), *g.Name, "", "", nil) // TODO maybe we can filter for (running) instances
		if err != nil {
			log.Warn("[AZURE] Failed to fetch the resources for %s, err: %s", *g.Name, err.Error())
			continue
		}
		for _, r := range resources.Values() {
			if _, ok := typesToCollect[*r.Type]; ok {
				if _, ok := r.Tags["Owner"]; !ok {
					instances = append(instances, &types.Instance{
						Name:      *r.Name,
						Id:        *r.ID,
						CloudType: types.AZURE,
						Tags:      utils.ConvertTags(r.Tags),
					})
				}
			}
		}
	}

	return instances, nil
}
