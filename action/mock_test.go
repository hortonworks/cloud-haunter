package action

import (
	"sync"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
)

// mockProvider is a configurable types.CloudProvider used across the action
// tests. It records how many times each mutating operation was invoked and
// captures the containers/arguments it received, so tests can assert that an
// action routed items to the correct provider method. Counters are guarded by a
// mutex because an action may invoke the same provider from several goroutines
// (e.g. stop dispatches instances and databases concurrently on one cloud).
type mockProvider struct {
	mu sync.Mutex

	stopInstances      int
	stopDatabases      int
	terminateInstances int
	terminateStacks    int
	terminateResources int
	deleteDisks        int
	deleteImages       int
	deleteAlerts       int
	deleteDatabases    int
	cleanupStorages    int

	retentionDays int
	instances     *types.InstanceContainer
	databases     *types.DatabaseContainer
	storages      *types.StorageContainer

	// opErr is returned by every mutating operation, letting a test drive the
	// provider-error path.
	opErr []error
}

func newMockProvider() *mockProvider {
	return &mockProvider{}
}

func (p *mockProvider) GetAccountName() string { return "mock" }

func (p *mockProvider) GetInstances() ([]*types.Instance, error) { return nil, nil }
func (p *mockProvider) GetAccesses() ([]*types.Access, error)    { return nil, nil }
func (p *mockProvider) GetDatabases() ([]*types.Database, error) { return nil, nil }
func (p *mockProvider) GetDisks() ([]*types.Disk, error)         { return nil, nil }
func (p *mockProvider) GetImages() ([]*types.Image, error)       { return nil, nil }
func (p *mockProvider) GetStacks() ([]*types.Stack, error)       { return nil, nil }
func (p *mockProvider) GetAlerts() ([]*types.Alert, error)       { return nil, nil }
func (p *mockProvider) GetStorages() ([]*types.Storage, error)   { return nil, nil }
func (p *mockProvider) GetResources() ([]*types.Resource, error) { return nil, nil }

func (p *mockProvider) StopInstances(c *types.InstanceContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopInstances++
	p.instances = c
	return p.opErr
}

func (p *mockProvider) TerminateInstances(c *types.InstanceContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.terminateInstances++
	p.instances = c
	return p.opErr
}

func (p *mockProvider) StopDatabases(c *types.DatabaseContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopDatabases++
	p.databases = c
	return p.opErr
}

func (p *mockProvider) DeleteDatabases(c *types.DatabaseContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deleteDatabases++
	p.databases = c
	return p.opErr
}

func (p *mockProvider) TerminateStacks(*types.StackContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.terminateStacks++
	return p.opErr
}

func (p *mockProvider) DeleteDisks(*types.DiskContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deleteDisks++
	return p.opErr
}

func (p *mockProvider) DeleteImages(*types.ImageContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deleteImages++
	return p.opErr
}

func (p *mockProvider) DeleteAlerts(*types.AlertContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deleteAlerts++
	return p.opErr
}

func (p *mockProvider) TerminateResources(*types.ResourceContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.terminateResources++
	return p.opErr
}

func (p *mockProvider) CleanupStorages(c *types.StorageContainer, retentionDays int) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cleanupStorages++
	p.storages = c
	p.retentionDays = retentionDays
	return p.opErr
}

// providerCfg builds a config whose CloudProviders registry returns the given
// provider for each supplied cloud type.
func providerCfg(providers map[types.CloudType]*mockProvider) *config.Config {
	registry := map[types.CloudType]func() types.CloudProvider{}
	for cloud, p := range providers {
		p := p
		registry[cloud] = func() types.CloudProvider { return p }
	}
	return &config.Config{CloudProviders: registry}
}
