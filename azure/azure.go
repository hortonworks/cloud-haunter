package azure

import (
	"context"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2015-11-01/subscriptions"
	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2021-04-01/storage"

	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hortonworks/cloud-haunter/utils"

	"sync"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

const (
	ResourceGroupName string = "ResourceGroupName"
	ScaleSetName      string = "ScaleSetName"
)

var provider = azureProvider{}

type azureProvider struct {
	subscriptionID         string
	vmClient               *armcompute.VirtualMachinesClient
	vmScaleSetClient       *armcompute.VirtualMachineScaleSetsClient
	vmScaleSetVMClient     *armcompute.VirtualMachineScaleSetVMsClient
	imageClient            *armcompute.ImagesClient
	rgClient               *armresources.ResourceGroupsClient
	dbClient               *armpostgresqlflexibleservers.ServersClient
	subscriptionClient     subscriptions.Client
	storageAccountClient   storage.AccountsClient
	storageContainerClient storage.BlobContainersClient
	// resClient      resources.Client
}

func init() {
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if len(subscriptionID) == 0 {
		log.Warn("[AZURE] AZURE_SUBSCRIPTION_ID environment variable is missing")
		return
	}
	ctx.CloudProviders[types.AZURE] = func() types.CloudProvider {
		if provider.vmClient == nil {
			log.Debug("[AZURE] Trying to prepare")
			if err := provider.init(subscriptionID, azidentity.NewEnvironmentCredential, auth.NewAuthorizerFromEnvironment); err != nil {
				panic("[AZURE] Failed to initialize provider: " + err.Error())
			}
			log.Info("[AZURE] Successfully prepared")
		}
		return provider
	}
}

func (p *azureProvider) init(subscriptionID string,
	credentialProvider func(*azidentity.EnvironmentCredentialOptions) (*azidentity.EnvironmentCredential, error),
	authorizer func() (autorest.Authorizer, error)) error {

	authorization, err := authorizer()

	p.subscriptionID = subscriptionID
	p.subscriptionClient = subscriptions.NewClient()
	p.subscriptionClient.Authorizer = authorization
	p.storageAccountClient = storage.NewAccountsClient(subscriptionID)
	p.storageAccountClient.Authorizer = authorization
	p.storageContainerClient = storage.NewBlobContainersClient(subscriptionID)
	p.storageContainerClient.Authorizer = authorization

	credential, err := credentialProvider(nil)
	if err != nil {
		log.Error("[AZURE] Failed to initialize credential: " + err.Error())
		return err
	}
	if p.vmClient, err = armcompute.NewVirtualMachinesClient(subscriptionID, credential, nil); err != nil {
		return err
	}
	if p.vmScaleSetClient, err = armcompute.NewVirtualMachineScaleSetsClient(subscriptionID, credential, nil); err != nil {
		return err
	}
	if p.vmScaleSetVMClient, err = armcompute.NewVirtualMachineScaleSetVMsClient(subscriptionID, credential, nil); err != nil {
		return err
	}
	if p.imageClient, err = armcompute.NewImagesClient(subscriptionID, credential, nil); err != nil {
		return err
	}
	if p.rgClient, err = armresources.NewResourceGroupsClient(subscriptionID, credential, nil); err != nil {
		return err
	}
	if p.dbClient, err = armpostgresqlflexibleservers.NewServersClient(subscriptionID, credential, nil); err != nil {
		return err
	}
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

func (p azureProvider) GetStacks() ([]*types.Stack, error) {
	log.Debug("[AZURE] Fetching resource groups")

	resourceGroups := p.rgClient.NewListPager(nil)
	var stacks []*types.Stack
	for i := 0; resourceGroups.More(); i++ {
		log.Infof("[AZURE] Fetching resource groups, round: %d", i+1)
		page, err := resourceGroups.NextPage(context.Background())
		if err != nil {
			log.Errorf("[AZURE] Failed to fetch resource groups, err: %s", err.Error())
			break
		}
		for _, rg := range page.Value {
			stacks = append(stacks, newStack(*rg))
		}
	}

	return stacks, nil
}

func (p azureProvider) GetInstances() ([]*types.Instance, error) {
	log.Debug("[AZURE] Fetching instances")

	vms := p.vmClient.NewListAllPager(nil)
	var vmList []*armcompute.VirtualMachine
	for vms.More() {
		log.Infof("[AZURE] fetching next page of instances")
		page, err := vms.NextPage(context.Background())
		if err != nil {
			log.Errorf("[AZURE] Failed to fetch the next instance list, err: %s", err.Error())
			break
		}
		vmList = append(vmList, page.Value...)
	}
	log.Infof("Total number of instances: %d", len(vmList))

	scaleSets := p.vmScaleSetClient.NewListAllPager(nil)
	var scaleSetList []*armcompute.VirtualMachineScaleSet
	for scaleSets.More() {
		log.Infof("[AZURE] fetching next page of scale sets")
		page, err := scaleSets.NextPage(context.Background())
		if err != nil {
			log.Errorf("[AZURE] Failed to fetch the next scale set list, err: %s", err.Error())
			break
		}
		scaleSetList = append(scaleSetList, page.Value...)
	}
	log.Infof("Total number of scale sets: %d", len(scaleSetList))

	wg := sync.WaitGroup{}
	wg.Add(len(vmList) + len(scaleSetList))
	instanceChan := make(chan interface{})

	for _, vm := range vmList {
		go func(vm armcompute.VirtualMachine) {
			defer wg.Done()

			resourceGroupName, _ := getResourceGroupName(*vm.ID)
			instanceName := *vm.Name
			if len(resourceGroupName) != 0 {
				log.Debugf("[AZURE] Fetch instance view of instance: %s", instanceName)
				if viewResult, err := p.vmClient.InstanceView(context.Background(), resourceGroupName, instanceName, nil); err != nil {
					log.Errorf("[AZURE] Failed to fetch the instance view of %s, err: %s", instanceName, err.Error())
				} else {
					instanceChan <- azureInstance{vm, viewResult, resourceGroupName}
				}
			} else {
				log.Errorf("[AZURE] Failed to find the resource group and name of instance: %s", instanceName)
			}
		}(*vm)
	}

	for _, ss := range scaleSetList {
		go func(scaleSet armcompute.VirtualMachineScaleSet) {
			defer wg.Done()

			resourceGroupName, _ := getResourceGroupName(*scaleSet.ID)
			if len(resourceGroupName) != 0 {
				ssVms := p.vmScaleSetVMClient.NewListPager(resourceGroupName, *scaleSet.Name, nil)
				for ssVms.More() {
					page, err := ssVms.NextPage(context.Background())
					if err != nil {
						log.Errorf("[AZURE] Failed to fetch the next scale set list, err: %s", err.Error())
					}
					for _, vm := range page.Value {
						if viewResult, err := p.vmScaleSetVMClient.GetInstanceView(context.Background(), resourceGroupName, *scaleSet.Name, *vm.InstanceID, nil); err != nil {
							log.Errorf("[AZURE] Failed to fetch the instance view of %s, err: %s", *vm.Name, err.Error())
						} else {
							instanceChan <- azureScaleSetInstance{*vm, viewResult, resourceGroupName, *scaleSet.Name, scaleSet.Tags}
						}
					}
				}
			} else {
				log.Errorf("[AZURE] Failed to find the resource group and name of scale set: %s", *scaleSet.Name)
			}
		}(*ss)
	}

	go func() {
		wg.Wait()
		close(instanceChan)
	}()

	var instances []*types.Instance
	for inst := range instanceChan {
		switch inst.(type) {
		case azureInstance:
			instances = append(instances, newInstanceByVM(inst.(azureInstance)))
		case azureScaleSetInstance:
			instances = append(instances, newInstanceByScaleSetVM(inst.(azureScaleSetInstance)))
		}
	}

	return instances, nil
}

type azureInstance struct {
	instance          armcompute.VirtualMachine
	instanceView      armcompute.VirtualMachinesClientInstanceViewResponse
	resourceGroupName string
}

type azureScaleSetInstance struct {
	instance          armcompute.VirtualMachineScaleSetVM
	instanceView      armcompute.VirtualMachineScaleSetVMsClientGetInstanceViewResponse
	resourceGroupName string
	scaleSetName      string
	tagMap            map[string]*string
}

func (p azureProvider) GetDisks() ([]*types.Disk, error) {
	return nil, errors.New("[AZURE] Disk operations are not supported")
}

func (p azureProvider) DeleteDisks(*types.DiskContainer) []error {
	return []error{errors.New("[AZURE] Disk deletion is not supported")}
}

func (p azureProvider) GetImages() ([]*types.Image, error) {
	log.Debug("[AZURE] Fetching images")
	imageResult := p.imageClient.NewListPager(nil)

	var images []*types.Image
	for imageResult.More() {
		page, err := imageResult.NextPage(context.Background())
		if err != nil {
			log.Errorf("[AZURE] Failed to fetch the images, err: %s", err.Error())
			break
		}
		for _, image := range page.Value {
			images = append(images, newImage(*image))
		}
	}

	return images, nil
}

func (p azureProvider) DeleteImages(images *types.ImageContainer) []error {
	log.Debug("[AZURE] Delete images")
	var imagesToDelete []azureImage
	for _, image := range images.Get(types.AZURE) {
		resourceGroup, name := getImageResourceGroupAndName(image.ID)
		imagesToDelete = append(imagesToDelete, azureImage{image, resourceGroup, name})
	}
	var existingImages []armcompute.Image
	existingImagesPager := p.imageClient.NewListPager(nil)
	for existingImagesPager.More() {
		page, err := existingImagesPager.NextPage(context.Background())
		if err != nil {
			return []error{errors.New("Failed to fetch available images, err: " + err.Error())}
		}
		for _, image := range page.Value {
			existingImages = append(existingImages, *image)
		}
	}
	return deleteImages(imagesClient{p.imageClient}, imagesToDelete, existingImages)
}

type imagesClient struct {
	*armcompute.ImagesClient
}

type azureImage struct {
	*types.Image
	resourceGroup string
	name          string
}

func (p azureProvider) TerminateInstances(*types.InstanceContainer) []error {
	return []error{errors.New("[AZURE] Termination is not supported")}
	//rgClient = resources.NewGroupsClient(subscriptionId)
	//	rgClient.Authorizer = authorization
	//	resClient = resources.NewClient(subscriptionId)
	//	resClient.Authorizer = authorization
	//instances := make([]*types.Instance, 0)
	//groups, err := rgClient.List(ctx.Background(), "", nil)
	//if err != nil {
	//	log.Errorf("[AZURE] Failed to fetch the existing resource groups, err: %s", err.Error())
	//	return nil, err
	//}
	//typesToCollect =: map[string]bool{"Microsoft.Compute/virtualMachines": true}
	//for _, g := range groups.Values() {
	//	resources, err := resClient.ListByResourceGroup(ctx.Background(), *g.Name, "", "", nil)
	//	if err != nil {
	//		log.Warn("[AZURE] Failed to fetch the resources for %s, err: %s", *g.Name, err.Error())
	//		continue
	//	}
	//	for _, r := range resources.Values() {
	//		if _, ok := typesToCollect[*r.Type]; ok {
	//			if _, ok := r.Tags["Owner"]; !ok {
	//				instances = append(instances, &types.Instance{
	//					Name:      *r.Name,
	//					Id:        *r.ID,
	//					CloudType: types.AZURE,
	//					Tags:      utils.ConvertTags(r.Tags),
	//				})
	//			}
	//		}
	//	}
	//}
	//
	//return instances, nil
}

func (p azureProvider) TerminateStacks(stacks *types.StackContainer) []error {
	log.Debug("[AZURE] Delete resource groups")

	azureStacks := stacks.Get(types.AZURE)
	var errs []error

	for _, rg := range azureStacks {
		rgName := rg.Name
		log.Infof("[AZURE] Delete resource group: %s", rgName)
		if ctx.DryRun {
			log.Infof("[AZURE] Dry-run set, resource group is not deleted: %s", rgName)
		} else {
			_, err := p.rgClient.BeginDelete(context.Background(), rgName, nil)
			if err != nil {
				if strings.Contains(err.Error(), "Code=\"ScopeLocked\"") {
					log.Warnf("[AZURE] Resource group %s has a resource lock, so it can not be deleted", rgName)
				} else {
					errs = append(errs, err)
				}
			}
		}
	}

	return errs
}

func (p azureProvider) StopInstances(instances *types.InstanceContainer) []error {
	azureInstances := instances.Get(types.AZURE)
	log.Debugf("[AZURE] Stopping instances (%d): %v", len(azureInstances), azureInstances)
	wg := sync.WaitGroup{}
	wg.Add(len(azureInstances))
	errChan := make(chan error)
	sem := make(chan bool, 5)

	for _, i := range azureInstances {
		go func(instance *types.Instance) {
			sem <- true
			defer func() {
				wg.Done()
				<-sem
			}()

			log.Debugf("[AZURE] Stopping instance: %s", instance.Name)
			var err error
			if _, ok := instance.Metadata[ScaleSetName]; ok {
				_, err = p.vmScaleSetVMClient.BeginDeallocate(context.Background(), instance.Metadata[ResourceGroupName], instance.Metadata[ScaleSetName], getScaleSetVMInstanceID(instance.Name), nil)
			} else {
				_, err = p.vmClient.BeginDeallocate(context.Background(), instance.Metadata[ResourceGroupName], instance.Name, nil)
			}
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

	var ers []error
	for err := range errChan {
		ers = append(ers, err)
	}
	return ers
}

func (p azureProvider) StopDatabases(databases *types.DatabaseContainer) (errs []error) {
	log.Debugf("[AZURE] Stop databases: %v", databases)

	for _, database := range databases.Get(types.AZURE) {
		log.Debugf("[AZURE] Stopping database: %s", database.Name)
		if ctx.DryRun {
			log.Infof("[AZURE] Dry-run set, database is not stopped: %s", database.Name)
		} else {
			_, err := p.dbClient.BeginStop(context.Background(), database.Metadata[ResourceGroupName], database.Name, nil)
			if err != nil {
				log.Errorf("[AZURE] Failed to stop database: %s", database.Name)
				errs = append(errs, err)
				continue
			}
		}
	}
	return errs
}

func (p azureProvider) GetAccesses() ([]*types.Access, error) {
	return nil, errors.New("[AZURE] Access not supported")
}

func (p azureProvider) GetDatabases() ([]*types.Database, error) {
	log.Debugf("[AZURE] Fetching Azure databases")

	var databases []*types.Database
	serverPager := p.dbClient.NewListPager(nil)
	for serverPager.More() {
		log.Debugf("[AZURE] Fetching database, next page")
		page, err := serverPager.NextPage(context.Background())
		if err != nil {
			log.Errorf("[AZURE] Failed to fetch databases, err: %v", err)
			break
		}
		for _, database := range page.Value {
			tags := make(types.Tags, len(database.Tags))
			for k, v := range database.Tags {
				tags[k] = *v
			}
			resourceGroup, _ := getResourceGroupName(*database.ID)
			databases = append(databases, &types.Database{
				ID:           *database.ID,
				Name:         *database.Name,
				Tags:         tags,
				Owner:        tags[ctx.OwnerLabel],
				Created:      getCreationTimeFromTags(tags, utils.ConvertTimeUnix),
				InstanceType: *database.SKU.Name,
				Region:       *database.Location,
				State:        convertDBStatusToState(*database.Properties.State),
				CloudType:    types.AZURE,
				Metadata:     map[string]string{ResourceGroupName: resourceGroup},
			})
		}
	}

	return databases, nil
}

func (p azureProvider) GetAlerts() ([]*types.Alert, error) {
	return nil, errors.New("[AZURE] Getting alerts is not supported yet")
}

func (p azureProvider) DeleteAlerts(*types.AlertContainer) []error {
	return []error{errors.New("[AZURE] Deleting alerts is not supported yet")}
}

func (p azureProvider) GetStorages() ([]*types.Storage, error) {
	accountListResultIterator, err := p.storageAccountClient.ListComplete(context.Background())
	if err != nil {
		log.Errorf("[AZURE] Failed to get storage accounts, err: %s", err)
		return nil, err
	}
	storages := []*types.Storage{}
	for accountListResultIterator.NotDone() {
		storageAccount := accountListResultIterator.Value()
		resourceGroup, resourceName := getResourceGroupName(*storageAccount.ID)
		tags := utils.ConvertTags(storageAccount.Tags)
		storage := &types.Storage{
			ID:        *storageAccount.ID,
			Name:      resourceName,
			Owner:     tags[ctx.OwnerLabel],
			Created:   storageAccount.CreationTime.Time,
			CloudType: types.AZURE,
			Tags:      tags,
			Region:    *storageAccount.PrimaryLocation,
			MetaData: map[string]string{
				ResourceGroupName: resourceGroup,
			},
		}

		storages = append(storages, storage)

		err = accountListResultIterator.Next()
		if err != nil {
			log.Errorf("[AZURE] Failed to get storage accounts next iteration, err: %s", err)
			return nil, err
		}
	}
	return storages, nil
}

func (p azureProvider) getContainerUrls(storage types.Storage) (*[]azblob.ContainerURL, error) {
	serviceUrl, err := p.getServiceUrl(storage)
	if err != nil {
		return nil, err
	}
	log.Debugf("[AZURE] Listing containers for storage account %s", storage.Name)
	containerUrls := []azblob.ContainerURL{}
	containerMarker := azblob.Marker{}
	for containerMarker.NotDone() {
		containers, err := serviceUrl.ListContainersSegment(context.Background(), containerMarker, azblob.ListContainersSegmentOptions{})
		if err != nil {
			log.Errorf("[AZURE] Failed to list containers for storage account %s, err: %s", storage.Name, err)
			return nil, err
		}
		for _, c := range containers.ContainerItems {
			containerUrls = append(containerUrls, serviceUrl.NewContainerURL(c.Name))
		}
		containerMarker = containers.NextMarker
	}
	log.Debugf("[AZURE] Storage account %s has %d containers", storage.Name, len(containerUrls))
	return &containerUrls, nil
}

func (p azureProvider) getServiceUrl(storage types.Storage) (*azblob.ServiceURL, error) {
	log.Debugf("[AZURE] Getting service url for storage account %s", storage.Name)
	keysResult, err := p.storageAccountClient.ListKeys(context.Background(), storage.MetaData[ResourceGroupName], storage.Name, "")
	if err != nil {
		log.Errorf("[AZURE] Failed to list access keys for storage account %s, err: %s", storage.Name, err)
		return nil, err
	}
	key := *(((*keysResult.Keys)[0]).Value)
	credential, err := azblob.NewSharedKeyCredential(storage.Name, key)
	if err != nil {
		log.Errorf("[AZURE] Failed to create credential for storage account %s, err: %s", storage.Name, err)
		return nil, err
	}
	url, err := url.Parse(fmt.Sprintf(`https://%s.blob.core.windows.net`, storage.Name))
	if err != nil {
		log.Errorf("[AZURE] Failed to create url for storage account %s, err: %s", storage.Name, err)
		return nil, err
	}
	serviceUrl := azblob.NewServiceURL(*url, azblob.NewPipeline(credential, azblob.PipelineOptions{}))
	log.Debugf("[AZURE] Service url for storage account %s is %s", storage.Name, serviceUrl.String())
	return &serviceUrl, nil
}

func (p azureProvider) getBlobsInContainer(storage types.Storage, containerUrl azblob.ContainerURL) (*[]azblob.BlobItemInternal, error) {
	log.Debugf("[AZURE] Getting blobs in storage account %s container %s", storage.Name, containerUrl.String())
	blobItems := []azblob.BlobItemInternal{}
	marker := azblob.Marker{}
	for marker.NotDone() {
		blobs, err := containerUrl.ListBlobsFlatSegment(context.Background(), marker, azblob.ListBlobsSegmentOptions{})
		if err != nil {
			log.Errorf("[AZURE] Failed to list blobs for storage account %s container %s, err: %s", storage.Name, containerUrl.String(), err)
			return nil, err
		}
		blobItems = append(blobItems, blobs.Segment.BlobItems...)
		marker = blobs.NextMarker
	}
	log.Debugf("[AZURE] Storage account %s container %s has %d blobs", storage.Name, containerUrl.String(), len(blobItems))
	return &blobItems, nil
}

func (p azureProvider) CleanupStorages(storageContainer *types.StorageContainer, retentionDays int) []error {
	retentionTime := time.Now().AddDate(0, 0, -retentionDays)
	log.Debugf("[AZURE] Cleaning up storages older than %s", retentionTime)

	wg := sync.WaitGroup{}
	errorChan := make(chan error)

	storages := storageContainer.Get(types.AZURE)
	storagesByRegion := map[string][]*types.Storage{}
	for _, storage := range storages {
		storagesByRegion[storage.Region] = append(storagesByRegion[storage.Region], storage)
	}
	wg.Add(len(storagesByRegion))

	for r, s := range storagesByRegion {
		go func(region string, storagesInRegion []*types.Storage) {
			defer wg.Done()

			for _, storage := range storagesInRegion {
				var cleanedUpStorage int64
				containerUrls, err := p.getContainerUrls(*storage)
				if err != nil {
					log.Errorf("[AZURE] Failed to get containers for storage account %s, err: %s", storage.Name, err)
					errorChan <- err
					continue
				}
				for _, containerUrl := range *containerUrls {
					blobs, err := p.getBlobsInContainer(*storage, containerUrl)
					if err != nil {
						log.Errorf("[AZURE] Failed to list blobs in container %s , err: %s", containerUrl.String(), err)
						errorChan <- err
						continue
					}
					for _, blob := range *blobs {
						if blob.Properties.CreationTime.Before(retentionTime) {
							log.Infof("[AZURE] Blob %s in storage account %s is older than %s", blob.Name, storage.Name, retentionTime)
							if !ctx.DryRun {
								_, err := containerUrl.NewBlobURL(blob.Name).Delete(context.Background(), azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
								if err != nil {
									log.Errorf("[AZURE] Failed to delete blob %s in storage account %s", blob.Name, storage.Name)
									errorChan <- err
								} else {
									cleanedUpStorage += *blob.Properties.ContentLength
								}
							}
						}
					}
				}
				log.Infof("[AZURE] Cleaned up %s worth of files in storage account %s", utils.GetHumanReadableFileSize(cleanedUpStorage), storage.Name)
			}
		}(r, s)
	}

	go func() {
		wg.Wait()
		close(errorChan)
	}()

	var errs []error
	for err := range errorChan {
		errs = append(errs, err)
	}

	return errs
}

func deleteImages(imagesClient imagesClient, imagesToDelete []azureImage, existingImages []armcompute.Image) []error {
	wg := sync.WaitGroup{}
	errorChan := make(chan error)
	for _, image := range existingImages {
		for _, imageToDelete := range imagesToDelete {
			if imageToDelete.name == *image.Name && imageToDelete.Region == *image.Location {
				resourceGroup, _ := getResourceGroupName(*image.ID)
				if resourceGroup == imageToDelete.resourceGroup {
					wg.Add(1)
					go func(i armcompute.Image, ai azureImage) {
						defer wg.Done()
						if ctx.DryRun {
							log.Infof("[AZURE] Dry-run set, image is not deleted: %s:%s, region: %s", *i.Name, ai.ID, *i.Location)
						} else {
							log.Infof("[AZURE] Delete image: %s", *i.ID)
							_, err := imagesClient.BeginDelete(context.Background(), resourceGroup, *i.Name, nil)
							if err != nil {
								log.Errorf("[AZURE] Unable to delete image: %s because: %s", ai.ID, err.Error())
								errorChan <- errors.New(ai.ID)
							}
						}
					}(image, imageToDelete)
				}
				break
			}
		}
	}
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	var errs []error
	for err := range errorChan {
		errs = append(errs, err)
	}

	return errs
}

func getImageResourceGroupAndName(url string) (string, string) {
	uri := strings.Replace(url, "https://", "", 1)
	parts := strings.Split(uri, "/")

	return strings.Split(parts[0], ".")[0], parts[len(parts)-1]
}

func getResourceGroupName(resourceID string) (resourceGroupName, resourceName string) {
	if strings.Index(resourceID, "https://") == 0 {
		uri := strings.Replace(resourceID, "https://", "", 1)
		parts := strings.Split(uri, "/")

		return strings.Split(parts[0], ".")[0], parts[len(parts)-1]
	}
	instanceIDParts := strings.Split(resourceID, "/")
	// /subscriptions/<sub_id>/resourceGroups/<rg_name>/providers/Microsoft.Compute/<type>/<inst_name>
	if len(instanceIDParts) == 9 {
		return instanceIDParts[4], instanceIDParts[8]
	}
	return
}

func getScaleSetVMInstanceID(name string) string {
	parts := strings.Split(name, "_")
	return parts[len(parts)-1]
}

func newInstanceByVM(inst azureInstance) *types.Instance {
	vm := inst.instance
	view := inst.instanceView
	return newInstance(*vm.Name, *vm.ID, *vm.Location, string(*vm.Properties.HardwareProfile.VMSize), inst.resourceGroupName, getInstanceState(view.Statuses), vm.Tags)
}

func newInstanceByScaleSetVM(inst azureScaleSetInstance) *types.Instance {
	instance := newInstance(*inst.instance.Name, *inst.instance.ID, *inst.instance.Location, string(*inst.instance.Properties.HardwareProfile.VMSize), inst.resourceGroupName, getScaleSetInstanceState(inst.instanceView), inst.tagMap)
	instance.Metadata[ScaleSetName] = inst.scaleSetName
	return instance
}

func newImage(image armcompute.Image) *types.Image {
	return &types.Image{
		ID:        *image.ID,
		Name:      *image.Name,
		Region:    *image.Location,
		CloudType: types.AZURE,
		Tags:      utils.ConvertTags(image.Tags),
	}
}

func newInstance(name, ID, location, instanceType, resourceGroupName string, state types.State, tagMap map[string]*string) *types.Instance {
	tags := utils.ConvertTags(tagMap)
	return &types.Instance{
		Name:         name,
		ID:           ID,
		Created:      getCreationTimeFromTags(tags, utils.ConvertTimeUnix),
		CloudType:    types.AZURE,
		Tags:         tags,
		Owner:        tags[ctx.OwnerLabel],
		Region:       location,
		InstanceType: instanceType,
		State:        state,
		Metadata:     map[string]string{ResourceGroupName: resourceGroupName},
	}
}

func getInstanceState(view []*armcompute.InstanceViewStatus) types.State {
	for _, v := range view {
		if v.Time == nil {
			return convertViewStatusToState(*v)
		}
	}
	return types.Unknown
}

func getScaleSetInstanceState(view armcompute.VirtualMachineScaleSetVMsClientGetInstanceViewResponse) types.State {
	for _, v := range view.Statuses {
		if v.Time == nil {
			return convertViewStatusToState(*v)
		}
	}
	return types.Unknown
}

// Possible values:
//
//	"PowerState/deallocated"
//	"PowerState/deallocating"
//	"PowerState/running"
//	"PowerState/starting"
//	"PowerState/stopped"
//	"PowerState/stopping"
//	"PowerState/unknown"
//
// The assumption is that the status without time is the currently active status
func convertViewStatusToState(actualStatus armcompute.InstanceViewStatus) types.State {
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

// Possible values:

// ServerStateDisabled  = "Disabled"
// ServerStateDropping  = "Dropping"
// ServerStateReady     = "Ready"
// ServerStateStarting  = "Starting"
// ServerStateStopped   = "Stopped"
// ServerStateStopping  = "Stopping"
// ServerStateUpdating  = "Updating"
func convertDBStatusToState(state armpostgresqlflexibleservers.ServerState) types.State {
	switch state {
	case armpostgresqlflexibleservers.ServerStateStopped,
		armpostgresqlflexibleservers.ServerStateStopping:
		return types.Stopped
	case armpostgresqlflexibleservers.ServerStateReady,
		armpostgresqlflexibleservers.ServerStateStarting,
		armpostgresqlflexibleservers.ServerStateUpdating:
		return types.Running
	case armpostgresqlflexibleservers.ServerStateDropping,
		armpostgresqlflexibleservers.ServerStateDisabled:
		return types.Terminated
	default:
		return types.Unknown
	}
}

func getCreationTimeFromTags(tags types.Tags, convertTimeUnix func(unixTimestamp string) time.Time) time.Time {
	for _, label := range strings.Split(ctx.AzureCreationTimeLabel, ",") {
		if creationTimestamp, ok := tags[label]; ok {
			return convertTimeUnix(creationTimestamp)
		}
	}
	return convertTimeUnix("0")
}

func newStack(rg armresources.ResourceGroup) *types.Stack {
	tags := utils.ConvertTags(rg.Tags)
	return &types.Stack{
		ID:        *rg.ID,
		Name:      *rg.Name,
		Created:   getCreationTimeFromTags(tags, utils.ConvertTimeUnix),
		Tags:      tags,
		Owner:     tags[ctx.OwnerLabel],
		CloudType: types.AZURE,
		State:     types.Running,
		Region:    *rg.Location,
	}
}
