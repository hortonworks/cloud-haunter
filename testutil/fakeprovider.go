// Package testutil provides shared test doubles for cloud-haunter.
//
// FakeProvider is a configurable types.CloudProvider: its Get* methods return
// preset slices (and a shared GetErr) so tests can drive the operation/read
// path, while its mutating methods record the containers they received so tests
// can assert an action routed the filtered items to the correct provider call.
// It replaces the per-package mocks that were duplicated across the action and
// operation test packages.
package testutil

import (
	"sync"

	"github.com/hortonworks/cloud-haunter/types"
)

// FakeProvider is a test double implementing types.CloudProvider. The zero value
// is usable: every reader returns an empty slice and every mutation records a
// call and returns no error. Populate the canned fields and error fields to
// drive specific paths.
type FakeProvider struct {
	// AccountName is returned by GetAccountName; defaults to "fake" when empty.
	AccountName string

	// Canned read results, returned by the matching Get* method.
	Instances []*types.Instance
	Accesses  []*types.Access
	Databases []*types.Database
	Disks     []*types.Disk
	Images    []*types.Image
	Stacks    []*types.Stack
	Alerts    []*types.Alert
	Storages  []*types.Storage
	Resources []*types.Resource

	// GetErr, when set, is returned by every Get* method to drive the read-path
	// failure branch.
	GetErr error
	// OpErr, when non-empty, is returned by every mutating method to drive the
	// action-path failure branch.
	OpErr []error

	mu sync.Mutex
	// Recorded mutation calls, one entry appended per invocation. Guarded by mu
	// because an action may invoke the same provider from several goroutines.
	StopInstancesCalls      []*types.InstanceContainer
	TerminateInstancesCalls []*types.InstanceContainer
	StopDatabasesCalls      []*types.DatabaseContainer
	DeleteDatabasesCalls    []*types.DatabaseContainer
	TerminateStacksCalls    []*types.StackContainer
	DeleteDisksCalls        []*types.DiskContainer
	DeleteImagesCalls       []*types.ImageContainer
	DeleteAlertsCalls       []*types.AlertContainer
	TerminateResourcesCalls []*types.ResourceContainer
	CleanupStoragesCalls    []CleanupStoragesCall
}

// CleanupStoragesCall records a single CleanupStorages invocation.
type CleanupStoragesCall struct {
	Container     *types.StorageContainer
	RetentionDays int
}

// GetAccountName returns AccountName, or "fake" when it was left empty.
func (p *FakeProvider) GetAccountName() string {
	if p.AccountName == "" {
		return "fake"
	}
	return p.AccountName
}

func (p *FakeProvider) GetInstances() ([]*types.Instance, error) { return p.Instances, p.GetErr }
func (p *FakeProvider) GetAccesses() ([]*types.Access, error)    { return p.Accesses, p.GetErr }
func (p *FakeProvider) GetDatabases() ([]*types.Database, error) { return p.Databases, p.GetErr }
func (p *FakeProvider) GetDisks() ([]*types.Disk, error)         { return p.Disks, p.GetErr }
func (p *FakeProvider) GetImages() ([]*types.Image, error)       { return p.Images, p.GetErr }
func (p *FakeProvider) GetStacks() ([]*types.Stack, error)       { return p.Stacks, p.GetErr }
func (p *FakeProvider) GetAlerts() ([]*types.Alert, error)       { return p.Alerts, p.GetErr }
func (p *FakeProvider) GetStorages() ([]*types.Storage, error)   { return p.Storages, p.GetErr }
func (p *FakeProvider) GetResources() ([]*types.Resource, error) { return p.Resources, p.GetErr }

func (p *FakeProvider) StopInstances(c *types.InstanceContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.StopInstancesCalls = append(p.StopInstancesCalls, c)
	return p.OpErr
}

func (p *FakeProvider) TerminateInstances(c *types.InstanceContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.TerminateInstancesCalls = append(p.TerminateInstancesCalls, c)
	return p.OpErr
}

func (p *FakeProvider) StopDatabases(c *types.DatabaseContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.StopDatabasesCalls = append(p.StopDatabasesCalls, c)
	return p.OpErr
}

func (p *FakeProvider) DeleteDatabases(c *types.DatabaseContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.DeleteDatabasesCalls = append(p.DeleteDatabasesCalls, c)
	return p.OpErr
}

func (p *FakeProvider) TerminateStacks(c *types.StackContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.TerminateStacksCalls = append(p.TerminateStacksCalls, c)
	return p.OpErr
}

func (p *FakeProvider) DeleteDisks(c *types.DiskContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.DeleteDisksCalls = append(p.DeleteDisksCalls, c)
	return p.OpErr
}

func (p *FakeProvider) DeleteImages(c *types.ImageContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.DeleteImagesCalls = append(p.DeleteImagesCalls, c)
	return p.OpErr
}

func (p *FakeProvider) DeleteAlerts(c *types.AlertContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.DeleteAlertsCalls = append(p.DeleteAlertsCalls, c)
	return p.OpErr
}

func (p *FakeProvider) TerminateResources(c *types.ResourceContainer) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.TerminateResourcesCalls = append(p.TerminateResourcesCalls, c)
	return p.OpErr
}

func (p *FakeProvider) CleanupStorages(c *types.StorageContainer, retentionDays int) []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.CleanupStoragesCalls = append(p.CleanupStoragesCalls, CleanupStoragesCall{Container: c, RetentionDays: retentionDays})
	return p.OpErr
}

// Registry builds the CloudProviders factory map that config.Config expects,
// returning the given fake for each cloud type.
func Registry(providers map[types.CloudType]*FakeProvider) map[types.CloudType]func() types.CloudProvider {
	registry := map[types.CloudType]func() types.CloudProvider{}
	for cloud, p := range providers {
		p := p
		registry[cloud] = func() types.CloudProvider { return p }
	}
	return registry
}
