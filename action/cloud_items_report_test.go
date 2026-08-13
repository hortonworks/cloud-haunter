package action

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

// TestCloudItemsReportReturnsErrorOnCreateFailure covers the file-create error
// branch (previously log.Fatalf) by targeting a read-only directory.
func TestCloudItemsReportReturnsErrorOnCreateFailure(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("file permissions are not enforced for root")
	}
	readonly := filepath.Join(t.TempDir(), "readonly")
	assert.NoError(t, os.Mkdir(readonly, 0555))

	err := CloudItemsReportAction{dir: readonly}.Execute(types.Instances, nil, []types.CloudItem{
		types.Instance{CloudType: types.AWS, Name: "inst-1"},
	})

	assert.Error(t, err)
}

// TestCloudItemsReportWritesCSV runs the report action against a temp directory
// and verifies the CSV it produces. The output directory is injected per
// instance, so the test needs no t.Chdir and runs in parallel.
func TestCloudItemsReportWritesCSV(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	err := CloudItemsReportAction{dir: dir}.Execute(types.Instances, nil, []types.CloudItem{
		types.Instance{CloudType: types.AWS, Name: "inst-1", Owner: "alice"},
		types.Storage{CloudType: types.GCP, Name: "store-1", Owner: "bob"},
	})
	assert.NoError(t, err)

	matches, err := filepath.Glob(filepath.Join(dir, "report-cloud-items-*.csv"))
	assert.NoError(t, err)
	assert.Len(t, matches, 1, "exactly one report file should be created")

	f, err := os.Open(matches[0])
	assert.NoError(t, err)
	defer func() { _ = f.Close() }()

	rows, err := csv.NewReader(f).ReadAll()
	assert.NoError(t, err)
	assert.Equal(t, [][]string{
		{"Cloud Type", "Name", "Owner"},
		{"AWS", "inst-1", "alice"},
		{"GCP", "store-1", "bob"},
	}, rows)
}
