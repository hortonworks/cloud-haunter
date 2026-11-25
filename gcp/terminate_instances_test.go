package gcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	compute "google.golang.org/api/compute/v1"
)

const (
	testProject = "project"
	region      = "europe-west1"
	zoneA       = "europe-west1-b"
	zoneB       = "europe-west1-c"
	otherRegion = "us-central1"
)

// fakeGroup is a managed instance group served by fakeComputeAPI. Exactly one of zone or region is set.
type fakeGroup struct {
	zone, region     string
	name             string
	baseInstanceName string
	members          []string // instance URLs
	failListing      bool
}

// scope is the path segment the compute API addresses the group by, e.g. "zones/europe-west1-b".
func (g fakeGroup) scope() string {
	if g.region != "" {
		return "regions/" + g.region
	}
	return "zones/" + g.zone
}

// fakeComputeAPI is an in-memory fake of the compute REST endpoints the instance termination uses.
// It records every request as "METHOD path" (plus the request body for deleteInstances).
// Mutations answer with a finished operation, or with a running one that is finished when polled.
// A failed operation is reported like the real API does: HTTP 200 with the error inside the operation.
// A rejected request is answered with an HTTP error instead.
type fakeComputeAPI struct {
	groups            []fakeGroup
	failOperations    bool
	rejectMutations   bool
	pendingOperations bool // the running operations are zonal (zoneA), so only use them with zoneA standalone instances

	mu       sync.Mutex
	requests []string
}

func instanceURL(zone, name string) string {
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/instances/%s", testProject, zone, name)
}

func (f *fakeComputeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	call := r.Method + " " + r.URL.Path
	if strings.HasSuffix(r.URL.Path, "/deleteInstances") {
		// the zonal and the regional request have the same JSON shape
		var body compute.InstanceGroupManagersDeleteInstancesRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		sort.Strings(body.Instances)
		call += " " + strings.Join(body.Instances, ",")
	}
	f.mu.Lock()
	f.requests = append(f.requests, call)
	f.mu.Unlock()

	switch {
	case r.URL.Path == "/projects/"+testProject+"/aggregated/instanceGroupManagers":
		items := map[string]*compute.InstanceGroupManagersScopedList{}
		for _, g := range f.groups {
			if items[g.scope()] == nil {
				items[g.scope()] = &compute.InstanceGroupManagersScopedList{}
			}
			manager := &compute.InstanceGroupManager{Name: g.name, BaseInstanceName: g.baseInstanceName}
			scopeURL := "https://www.googleapis.com/compute/v1/projects/" + testProject + "/" + g.scope()
			if g.region != "" {
				manager.Region = scopeURL
			} else {
				manager.Zone = scopeURL
			}
			items[g.scope()].InstanceGroupManagers = append(items[g.scope()].InstanceGroupManagers, manager)
		}
		writeJSON(w, map[string]interface{}{"items": items})
	case strings.HasSuffix(r.URL.Path, "/listManagedInstances"):
		group := f.group(r.URL.Path)
		if group == nil || group.failListing {
			http.Error(w, `{"error":{"code":500,"message":"listing failed"}}`, http.StatusInternalServerError)
			return
		}
		var members []*compute.ManagedInstance
		for _, m := range group.members {
			members = append(members, &compute.ManagedInstance{Instance: m})
		}
		// the zonal and the regional response have the same JSON shape
		writeJSON(w, compute.InstanceGroupManagersListManagedInstancesResponse{ManagedInstances: members})
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/operations/"):
		writeJSON(w, f.operation("DONE"))
	case f.rejectMutations:
		http.Error(w, `{"error":{"code":400,"message":"request rejected"}}`, http.StatusBadRequest)
	case f.pendingOperations:
		operation := f.operation("RUNNING")
		operation.Zone = "https://www.googleapis.com/compute/v1/projects/" + testProject + "/zones/" + zoneA
		writeJSON(w, operation)
	default:
		writeJSON(w, f.operation("DONE"))
	}
}

func (f *fakeComputeAPI) operation(status string) *compute.Operation {
	operation := &compute.Operation{Name: "operation", Status: status}
	if status == "DONE" && f.failOperations {
		operation.Error = &compute.OperationError{Errors: []*compute.OperationErrorErrors{{Code: "RESOURCE_NOT_READY", Message: "deletion failed"}}}
	}
	return operation
}

// group returns the group addressed by a .../{zones|regions}/{location}/instanceGroupManagers/{name}/... path.
func (f *fakeComputeAPI) group(path string) *fakeGroup {
	for i, g := range f.groups {
		if strings.Contains(path, "/"+g.scope()+"/instanceGroupManagers/"+g.name+"/") {
			return &f.groups[i]
		}
	}
	return nil
}

// mutations returns the recorded requests that change state, sorted. listManagedInstances is a POST but a read.
func (f *fakeComputeAPI) mutations() []string {
	return f.recorded(func(r string) bool {
		return !strings.HasPrefix(r, "GET ") && !strings.HasSuffix(r, "/listManagedInstances")
	})
}

// listings returns the recorded listManagedInstances requests, sorted.
func (f *fakeComputeAPI) listings() []string {
	return f.recorded(func(r string) bool { return strings.HasSuffix(r, "/listManagedInstances") })
}

func (f *fakeComputeAPI) recorded(keep func(string) bool) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var requests []string
	for _, r := range f.requests {
		if keep(r) {
			requests = append(requests, r)
		}
	}
	sort.Strings(requests)
	return requests
}

func writeJSON(w http.ResponseWriter, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func newFakeComputeProvider(t *testing.T, api *fakeComputeAPI, dryRun bool) gcpProvider {
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	computeClient, err := compute.New(server.Client())
	require.NoError(t, err)
	computeClient.BasePath = server.URL + "/"
	return gcpProvider{cfg: &config.Config{DryRun: dryRun}, projectID: testProject, computeClient: computeClient}
}

// gcpInstance builds a selected instance the way the inventory does: zone in the metadata, region derived from it.
func gcpInstance(zone, name string) *types.Instance {
	return &types.Instance{CloudType: types.GCP, Name: name, Region: zone[:len(zone)-2], Metadata: map[string]string{"zone": zone}}
}

func deleteInstance(zone, name string) string {
	return fmt.Sprintf("DELETE /projects/%s/zones/%s/instances/%s", testProject, zone, name)
}

func deleteMembers(scope, name string, memberURLs ...string) string {
	return fmt.Sprintf("POST /projects/%s/%s/instanceGroupManagers/%s/deleteInstances %s", testProject, scope, name, strings.Join(memberURLs, ","))
}

func listMembers(scope, name string) string {
	return fmt.Sprintf("POST /projects/%s/%s/instanceGroupManagers/%s/listManagedInstances", testProject, scope, name)
}

func TestTerminateInstances(t *testing.T) {
	cases := []struct {
		name              string
		groups            []fakeGroup
		selected          []*types.Instance
		failOperations    bool
		rejectMutations   bool
		expectedMutations []string
		expectedErrs      int
	}{
		{
			name:              "standalone instance does not delete a same-prefix group in another zone",
			groups:            []fakeGroup{{zone: zoneB, name: "unrelated-group", baseInstanceName: "worker", members: []string{instanceURL(zoneB, "worker-zzzz")}}},
			selected:          []*types.Instance{gcpInstance(zoneA, "worker-mock")},
			expectedMutations: []string{deleteInstance(zoneA, "worker-mock")},
		},
		{
			name:              "standalone instance does not delete a same-prefix group it is not a member of",
			groups:            []fakeGroup{{zone: zoneA, name: "unrelated-group", baseInstanceName: "worker", members: []string{instanceURL(zoneA, "worker-zzzz")}}},
			selected:          []*types.Instance{gcpInstance(zoneA, "worker-mock")},
			expectedMutations: []string{deleteInstance(zoneA, "worker-mock")},
		},
		{
			name: "partially selected group only loses the selected members",
			groups: []fakeGroup{{zone: zoneA, name: "group", baseInstanceName: "worker", members: []string{
				instanceURL(zoneA, "worker-a"), instanceURL(zoneA, "worker-b"), instanceURL(zoneA, "worker-c")}}},
			selected:          []*types.Instance{gcpInstance(zoneA, "worker-a"), gcpInstance(zoneA, "worker-c")},
			expectedMutations: []string{deleteMembers("zones/"+zoneA, "group", instanceURL(zoneA, "worker-a"), instanceURL(zoneA, "worker-c"))},
		},
		{
			// deleting the group could also delete a VM the group creates after its members were listed
			name:              "fully selected group only loses the selected members, the group is kept",
			groups:            []fakeGroup{{zone: zoneA, name: "group", baseInstanceName: "worker", members: []string{instanceURL(zoneA, "worker-a"), instanceURL(zoneA, "worker-b")}}},
			selected:          []*types.Instance{gcpInstance(zoneA, "worker-a"), gcpInstance(zoneA, "worker-b")},
			expectedMutations: []string{deleteMembers("zones/"+zoneA, "group", instanceURL(zoneA, "worker-a"), instanceURL(zoneA, "worker-b"))},
		},
		{
			name:              "member with a custom name is removed through its group",
			groups:            []fakeGroup{{zone: zoneA, name: "group", baseInstanceName: "worker", members: []string{instanceURL(zoneA, "worker-a"), instanceURL(zoneA, "custom-name")}}},
			selected:          []*types.Instance{gcpInstance(zoneA, "custom-name")},
			expectedMutations: []string{deleteMembers("zones/"+zoneA, "group", instanceURL(zoneA, "custom-name"))},
		},
		{
			name: "overlapping prefixes only touch the group of the member",
			groups: []fakeGroup{
				{zone: zoneA, name: "workers", baseInstanceName: "worker", members: []string{instanceURL(zoneA, "worker-a")}},
				{zone: zoneA, name: "gpu-workers", baseInstanceName: "worker-gpu", members: []string{instanceURL(zoneA, "worker-gpu-a"), instanceURL(zoneA, "worker-gpu-b")}},
			},
			selected:          []*types.Instance{gcpInstance(zoneA, "worker-gpu-a")},
			expectedMutations: []string{deleteMembers("zones/"+zoneA, "gpu-workers", instanceURL(zoneA, "worker-gpu-a"))},
		},
		{
			name:              "partially selected regional group only loses the selected members",
			groups:            []fakeGroup{{region: region, name: "regional", baseInstanceName: "worker", members: []string{instanceURL(zoneA, "worker-a"), instanceURL(zoneB, "worker-b")}}},
			selected:          []*types.Instance{gcpInstance(zoneB, "worker-b")},
			expectedMutations: []string{deleteMembers("regions/"+region, "regional", instanceURL(zoneB, "worker-b"))},
		},
		{
			name:              "fully selected regional group only loses the selected members, the group is kept",
			groups:            []fakeGroup{{region: region, name: "regional", baseInstanceName: "worker", members: []string{instanceURL(zoneA, "worker-a"), instanceURL(zoneB, "worker-b")}}},
			selected:          []*types.Instance{gcpInstance(zoneA, "worker-a"), gcpInstance(zoneB, "worker-b")},
			expectedMutations: []string{deleteMembers("regions/"+region, "regional", instanceURL(zoneA, "worker-a"), instanceURL(zoneB, "worker-b"))},
		},
		{
			name:         "instance is not deleted when its group membership cannot be verified",
			groups:       []fakeGroup{{zone: zoneA, name: "group", baseInstanceName: "worker", members: []string{instanceURL(zoneA, "worker-a")}, failListing: true}},
			selected:     []*types.Instance{gcpInstance(zoneA, "worker-a")},
			expectedErrs: 1,
		},
		{
			name:         "instance is not deleted when its regional group membership cannot be verified",
			groups:       []fakeGroup{{region: region, name: "regional", baseInstanceName: "worker", members: []string{instanceURL(zoneA, "worker-a")}, failListing: true}},
			selected:     []*types.Instance{gcpInstance(zoneA, "worker-a")},
			expectedErrs: 1,
		},
		{
			name:         "a covering group that fails to list blocks standalone instances too",
			groups:       []fakeGroup{{zone: zoneA, name: "broken", baseInstanceName: "other", failListing: true}},
			selected:     []*types.Instance{gcpInstance(zoneA, "standalone")},
			expectedErrs: 1,
		},
		{
			name: "member is found although another covering group fails to list",
			groups: []fakeGroup{
				{zone: zoneA, name: "broken", baseInstanceName: "other", failListing: true},
				{zone: zoneA, name: "group", baseInstanceName: "worker", members: []string{instanceURL(zoneA, "worker-a"), instanceURL(zoneA, "worker-b")}},
			},
			selected:          []*types.Instance{gcpInstance(zoneA, "worker-a")},
			expectedMutations: []string{deleteMembers("zones/"+zoneA, "group", instanceURL(zoneA, "worker-a"))},
		},
		{
			name: "failed regional deletions are returned",
			groups: []fakeGroup{
				{region: region, name: "full", baseInstanceName: "full", members: []string{instanceURL(zoneA, "full-a")}},
				{region: region, name: "partial", baseInstanceName: "partial", members: []string{instanceURL(zoneA, "partial-a"), instanceURL(zoneB, "partial-b")}},
			},
			selected:       []*types.Instance{gcpInstance(zoneA, "full-a"), gcpInstance(zoneA, "partial-a")},
			failOperations: true,
			expectedMutations: []string{
				deleteMembers("regions/"+region, "full", instanceURL(zoneA, "full-a")),
				deleteMembers("regions/"+region, "partial", instanceURL(zoneA, "partial-a")),
			},
			expectedErrs: 2,
		},
		{
			name:              "rejected deletions are returned",
			groups:            []fakeGroup{{zone: zoneA, name: "group", baseInstanceName: "worker", members: []string{instanceURL(zoneA, "worker-a")}}},
			selected:          []*types.Instance{gcpInstance(zoneA, "worker-a"), gcpInstance(zoneA, "standalone")},
			rejectMutations:   true,
			expectedMutations: []string{deleteInstance(zoneA, "standalone"), deleteMembers("zones/"+zoneA, "group", instanceURL(zoneA, "worker-a"))},
			expectedErrs:      2,
		},
		{
			name:              "failed deletions are returned",
			groups:            []fakeGroup{{zone: zoneA, name: "group", baseInstanceName: "worker", members: []string{instanceURL(zoneA, "worker-a")}}},
			selected:          []*types.Instance{gcpInstance(zoneA, "worker-a"), gcpInstance(zoneA, "standalone")},
			failOperations:    true,
			expectedMutations: []string{deleteInstance(zoneA, "standalone"), deleteMembers("zones/"+zoneA, "group", instanceURL(zoneA, "worker-a"))},
			expectedErrs:      2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeComputeAPI{groups: tc.groups, failOperations: tc.failOperations, rejectMutations: tc.rejectMutations}
			provider := newFakeComputeProvider(t, api, false)

			errs := provider.TerminateInstances(types.NewInstanceContainer(tc.selected))

			assert.Len(t, errs, tc.expectedErrs)
			assert.Equal(t, tc.expectedMutations, api.mutations())
		})
	}
}

// TestTerminateInstancesDryRun verifies that dry-run resolves the group membership, but sends no mutating request.
func TestTerminateInstancesDryRun(t *testing.T) {
	api := &fakeComputeAPI{groups: []fakeGroup{
		{zone: zoneA, name: "full", baseInstanceName: "full", members: []string{instanceURL(zoneA, "full-a")}},
		{zone: zoneA, name: "partial", baseInstanceName: "partial", members: []string{instanceURL(zoneA, "partial-a"), instanceURL(zoneA, "partial-b")}},
		{region: region, name: "regional", baseInstanceName: "regional", members: []string{instanceURL(zoneA, "regional-a"), instanceURL(zoneB, "regional-b")}},
	}}
	provider := newFakeComputeProvider(t, api, true)
	selected := []*types.Instance{gcpInstance(zoneA, "full-a"), gcpInstance(zoneA, "partial-a"), gcpInstance(zoneA, "regional-a"), gcpInstance(zoneA, "standalone")}

	errs := provider.TerminateInstances(types.NewInstanceContainer(selected))

	assert.Empty(t, errs)
	assert.Equal(t, []string{
		listMembers("regions/"+region, "regional"),
		listMembers("zones/"+zoneA, "full"),
		listMembers("zones/"+zoneA, "partial"),
	}, api.listings())
	assert.Empty(t, api.mutations())
}

// TestTerminateInstancesListsOnlyGroupsCoveringTheInstance verifies that only the groups in the instance's zone or
// region are asked for their members, whatever their name, and each of them only once.
func TestTerminateInstancesListsOnlyGroupsCoveringTheInstance(t *testing.T) {
	api := &fakeComputeAPI{groups: []fakeGroup{
		{zone: zoneA, name: "same-zone", baseInstanceName: "other"},
		{zone: zoneB, name: "other-zone", baseInstanceName: "worker"},
		{region: region, name: "same-region", baseInstanceName: "other"},
		{region: otherRegion, name: "other-region", baseInstanceName: "worker"},
	}}
	provider := newFakeComputeProvider(t, api, false)
	selected := []*types.Instance{gcpInstance(zoneA, "worker-a"), gcpInstance(zoneA, "worker-b")}

	errs := provider.TerminateInstances(types.NewInstanceContainer(selected))

	assert.Empty(t, errs)
	assert.Equal(t, []string{listMembers("regions/"+region, "same-region"), listMembers("zones/"+zoneA, "same-zone")}, api.listings())
	assert.Equal(t, []string{deleteInstance(zoneA, "worker-a"), deleteInstance(zoneA, "worker-b")}, api.mutations())
}

// TestTerminateInstancesPollsOperations verifies that a deletion is only successful once its operation is finished
// without an error, and that an operation which is already finished is not polled.
func TestTerminateInstancesPollsOperations(t *testing.T) {
	pollOperation := "GET /projects/" + testProject + "/zones/" + zoneA + "/operations/operation"
	cases := []struct {
		name              string
		pendingOperations bool
		failOperations    bool
		expectedPolls     []string
		expectedErrs      int
	}{
		{name: "running operation that finishes", pendingOperations: true, expectedPolls: []string{pollOperation}},
		{name: "running operation that fails", pendingOperations: true, failOperations: true, expectedPolls: []string{pollOperation}, expectedErrs: 1},
		{name: "already finished operation is not polled"},
		{name: "already failed operation is not polled", failOperations: true, expectedErrs: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			api := &fakeComputeAPI{pendingOperations: tc.pendingOperations, failOperations: tc.failOperations}
			provider := newFakeComputeProvider(t, api, false)

			errs := provider.TerminateInstances(types.NewInstanceContainer([]*types.Instance{gcpInstance(zoneA, "standalone")}))

			assert.Len(t, errs, tc.expectedErrs)
			assert.Equal(t, []string{deleteInstance(zoneA, "standalone")}, api.mutations())
			assert.Equal(t, tc.expectedPolls, api.recorded(func(r string) bool { return strings.Contains(r, "/operations/") }))
		})
	}
}
