package action

import (
	"testing"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/suite"
)

type mockProvider struct {
	calls int
}

func (p *mockProvider) GetAccountName() string {
	return "mock"
}

func (p *mockProvider) GetInstances() ([]*types.Instance, error) {
	return nil, nil
}

func (p *mockProvider) TerminateInstances(*types.InstanceContainer) []error {
	p.calls++
	return nil
}

func (p *mockProvider) TerminateStacks(*types.StackContainer) []error {
	p.calls++
	return nil
}

func (p *mockProvider) StopInstances(*types.InstanceContainer) []error {
	p.calls++
	return nil
}

func (p mockProvider) StopDatabases(_ *types.DatabaseContainer) (e []error) {
	return
}

func (p *mockProvider) GetAccesses() ([]*types.Access, error) {
	return nil, nil
}

func (p *mockProvider) GetDatabases() ([]*types.Database, error) {
	return nil, nil
}

func (p *mockProvider) GetDisks() ([]*types.Disk, error) {
	return nil, nil
}

func (p *mockProvider) DeleteDisks(*types.DiskContainer) []error {
	return nil
}

func (p *mockProvider) GetImages() ([]*types.Image, error) {
	return nil, nil
}

func (p *mockProvider) GetStacks() ([]*types.Stack, error) {
	return nil, nil
}

func (p *mockProvider) DeleteImages(*types.ImageContainer) []error {
	return nil
}

func (p *mockProvider) GetAlerts() ([]*types.Alert, error) {
	return nil, nil
}

func (p *mockProvider) DeleteAlerts(*types.AlertContainer) []error {
	return nil
}

func (p *mockProvider) GetStorages() ([]*types.Storage, error) {
	return nil, nil
}

func (p *mockProvider) CleanupStorages(storageContainer *types.StorageContainer, retentionDays int) []error {
	return nil
}

func (p *mockProvider) GetClusters() ([]*types.Cluster, error) {
	return nil, nil
}

type terminationSuite struct {
	suite.Suite
	providers    map[types.CloudType]func() types.CloudProvider
	mockProvider *mockProvider
}

func (s *terminationSuite) SetupSuite() {
	s.providers = ctx.CloudProviders
}

func (s *terminationSuite) SetupTest() {
	s.mockProvider = &mockProvider{0}
	ctx.CloudProviders = map[types.CloudType]func() types.CloudProvider{
		types.AWS: func() types.CloudProvider {
			return s.mockProvider
		}}
}

func (s *terminationSuite) TearDownSuite() {
	ctx.CloudProviders = s.providers
}

func (s *terminationSuite) TestTermination() {
	action := terminationAction{}
	op := types.Instances
	items := []types.CloudItem{
		types.Instance{CloudType: types.AWS},
		types.Instance{CloudType: types.GCP},
	}

	action.Execute(op, []types.FilterType{}, items)

	s.Equal(1, s.mockProvider.calls)
}

func TestTerminationSuite(t *testing.T) {
	suite.Run(t, new(terminationSuite))
}
