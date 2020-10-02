package ibm

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
	"github.com/softlayer/softlayer-go/datatypes"
	"github.com/softlayer/softlayer-go/services"
	"github.com/softlayer/softlayer-go/session"
)

var provider = ibmProvider{}

type ibmProvider struct {
	userName string
	session  *session.Session
}

func init() {
	userName, ok := os.LookupEnv("SOFTLAYER_USERNAME")
	if !ok {
		log.Warn("[IBM] SOFTLAYER_USERNAME environment variable is missing")
		return
	}
	if _, ok := os.LookupEnv("SOFTLAYER_API_KEY"); !ok {
		log.Warn("[IBM] SOFTLAYER_API_KEY environment variable is missing")
		return
	}
	ctx.CloudProviders[types.IBM] = func() types.CloudProvider {
		provider.init(userName)
		return provider
	}
}

func (p *ibmProvider) init(userName string) {
	p.userName = userName
	p.session = session.New()
	if ctx.Verbose {
		p.session.Debug = true
	}
}

func (p ibmProvider) GetAccountName() string {
	return p.userName
}

func (p ibmProvider) GetInstances() ([]*types.Instance, error) {
	log.Debug("[IBM] Fetching instances")
	service := services.GetAccountService(p.session)
	vms, err := service.Mask("id;hostname;createDate;account[email];datacenter;powerState;maxCpu;maxMemory;tagReferences").GetVirtualGuests()
	if err != nil {
		log.Errorf("[IBM] Failed to fetch the running instances, err: %s", err.Error())
		return nil, err
	}

	instances := []*types.Instance{}
	for _, vm := range vms {
		instances = append(instances, newInstance(vm))
	}

	return instances, nil
}

func (p ibmProvider) GetStacks() ([]*types.Stack, error) {
	return nil, nil
}

func (p ibmProvider) GetDisks() ([]*types.Disk, error) {
	return nil, errors.New("[IBM] Disk operations are not supported")
}

func (p ibmProvider) DeleteDisks(*types.DiskContainer) []error {
	return []error{errors.New("[IBM] Disk deletion is not supported")}
}

func (p ibmProvider) GetImages() ([]*types.Image, error) {
	return nil, errors.New("[IBM] Not implemented")
}

func (p ibmProvider) DeleteImages(images *types.ImageContainer) []error {
	return []error{errors.New("[IBM] Not implemented")}
}

func (p ibmProvider) TerminateInstances(instances *types.InstanceContainer) []error {
	return []error{errors.New("[IBM] Not implemented")}
}

func (p ibmProvider) TerminateStacks(*types.StackContainer) []error {
	return []error{errors.New("[IBM] Termination is not supported")}
}

func (p ibmProvider) StopInstances(*types.InstanceContainer) []error {
	return []error{errors.New("[IBM] Stop not supported")}
}

func (p ibmProvider) StopDatabases(*types.DatabaseContainer) []error {
	return []error{errors.New("[IBM] Not implemented")}
}

func (p ibmProvider) GetAccesses() ([]*types.Access, error) {
	return nil, errors.New("[IBM] Not implemented")
}

func (p ibmProvider) GetDatabases() ([]*types.Database, error) {
	return nil, errors.New("[IBM] Not implemented")
}

func newInstance(vm datatypes.Virtual_Guest) *types.Instance {
	tags := convertTags(vm)
	return &types.Instance{
		CloudType:    types.IBM,
		ID:           strconv.Itoa(*vm.Id),
		Name:         *vm.Hostname,
		Created:      vm.CreateDate.Time,
		Owner:        *vm.Account.Email,
		InstanceType: getInstanceType(vm),
		Region:       *vm.Datacenter.LongName,
		State:        getInstanceState(vm),
		Tags:         tags,
	}
}

func getInstanceType(vm datatypes.Virtual_Guest) string {
	return fmt.Sprintf("%dvCPU-%dRAM", *vm.MaxCpu, *vm.MaxMemory)
}

func getInstanceState(vm datatypes.Virtual_Guest) types.State {
	switch *vm.PowerState.KeyName {
	case "RUNNING":
		return types.Running
	case "HALTED":
		return types.Stopped
	default:
		return types.Unknown
	}
}

func convertTags(vm datatypes.Virtual_Guest) map[string]string {
	result := make(map[string]string, 0)
	for _, t := range vm.TagReferences {
		tag := strings.Split(*t.Tag.Name, ":")
		if len(tag) == 1 {
			tag = append(tag, "")
		}
		result[tag[0]] = tag[1]
	}
	return result
}
