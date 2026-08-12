package operation

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
)

// mockProvider is a configurable types.CloudProvider for operation tests. Each
// Get* method returns its preset slice together with the shared err, so a test
// can drive an operation's collect path and assert on the items it produces (or
// simulate a provider failure by setting err).
type mockProvider struct {
	instances []*types.Instance
	accesses  []*types.Access
	databases []*types.Database
	disks     []*types.Disk
	images    []*types.Image
	stacks    []*types.Stack
	alerts    []*types.Alert
	storages  []*types.Storage
	resources []*types.Resource
	err       error
}

func (p *mockProvider) GetAccountName() string { return "mock" }

func (p *mockProvider) GetInstances() ([]*types.Instance, error) { return p.instances, p.err }
func (p *mockProvider) GetAccesses() ([]*types.Access, error)    { return p.accesses, p.err }
func (p *mockProvider) GetDatabases() ([]*types.Database, error) { return p.databases, p.err }
func (p *mockProvider) GetDisks() ([]*types.Disk, error)         { return p.disks, p.err }
func (p *mockProvider) GetImages() ([]*types.Image, error)       { return p.images, p.err }
func (p *mockProvider) GetStacks() ([]*types.Stack, error)       { return p.stacks, p.err }
func (p *mockProvider) GetAlerts() ([]*types.Alert, error)       { return p.alerts, p.err }
func (p *mockProvider) GetStorages() ([]*types.Storage, error)   { return p.storages, p.err }
func (p *mockProvider) GetResources() ([]*types.Resource, error) { return p.resources, p.err }

// The mutating operations are unused by the operation package.
func (p *mockProvider) StopInstances(*types.InstanceContainer) []error       { return nil }
func (p *mockProvider) TerminateInstances(*types.InstanceContainer) []error  { return nil }
func (p *mockProvider) StopDatabases(*types.DatabaseContainer) []error       { return nil }
func (p *mockProvider) TerminateStacks(*types.StackContainer) []error        { return nil }
func (p *mockProvider) DeleteAlerts(*types.AlertContainer) []error           { return nil }
func (p *mockProvider) DeleteDatabases(*types.DatabaseContainer) []error     { return nil }
func (p *mockProvider) DeleteDisks(*types.DiskContainer) []error             { return nil }
func (p *mockProvider) DeleteImages(*types.ImageContainer) []error           { return nil }
func (p *mockProvider) CleanupStorages(*types.StorageContainer, int) []error { return nil }
func (p *mockProvider) TerminateResources(*types.ResourceContainer) []error  { return nil }

// providerRegistry builds a CloudProviders registry returning the given provider
// per cloud type.
func providerRegistry(providers map[types.CloudType]types.CloudProvider) map[types.CloudType]func() types.CloudProvider {
	registry := map[types.CloudType]func() types.CloudProvider{}
	for cloud, p := range providers {
		p := p
		registry[cloud] = func() types.CloudProvider { return p }
	}
	return registry
}

// providerCfg builds a config whose CloudProviders registry returns the given
// provider per cloud type.
func providerCfg(providers map[types.CloudType]types.CloudProvider) *config.Config {
	return &config.Config{CloudProviders: providerRegistry(providers)}
}
