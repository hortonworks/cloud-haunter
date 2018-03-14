package azure

import (
	"context"
	"os"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2017-12-01/compute"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	log "github.com/Sirupsen/logrus"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

var (
	subscriptionId string
	vmClient       compute.VirtualMachinesClient
)

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
		types.CloudProviders[types.AZURE] = new(AzureProvider)
		log.Info("[AZURE] Successfully registered as provider")
	} else {
		log.Warn("[AZURE] AZURE_SUBSCRIPTION_ID environment variable is missing")
	}
}

type AzureProvider struct {
}

func (p *AzureProvider) GetRunningInstances() []*types.Instance {
	instances := make([]*types.Instance, 0)
	result, err := vmClient.ListAll(context.Background())
	if err != nil {
		log.Errorf("[AZURE] Failed to fetch the running instances, err: %s", err.Error())
		return instances
	}
	for _, inst := range result.Values() {
		instances = append(instances, &types.Instance{
			Name: *inst.Name,
			Id:   *inst.ID,
		})
	}
	return instances
}
