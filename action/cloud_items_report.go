package action

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewCloudItemsReport returns the cloud-items report action implementation. The
// report is written into the current working directory (empty dir).
func NewCloudItemsReport() types.Action {
	return CloudItemsReportAction{}
}

type CloudItemsReportAction struct {
	// dir is the directory the report file is created in. Empty means the current
	// working directory (filepath.Join drops it), preserving the original
	// behaviour. It is injectable so tests point each instance at its own
	// t.TempDir and run in parallel, instead of sandboxing via t.Chdir (which
	// mutates the process working directory and forbids t.Parallel).
	dir string
}

func (a CloudItemsReportAction) Execute(op types.OpType, filter []types.FilterType, items []types.CloudItem) (err error) {
	isoTimestamp := time.Now().UTC().Format(time.RFC3339)
	isoTimestamp = strings.ReplaceAll(isoTimestamp, ":", "-") // colon is disallowed on some platforms (e.g. MS/Windows).
	fileName := filepath.Join(a.dir, fmt.Sprintf("report-cloud-items-%s.csv", isoTimestamp))

	log.Infof("Writing cloud items report...")

	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("could not create report file %s: %w", fileName, err)
	}
	// A Close error can mean buffered data never reached disk, so surface it -
	// but don't let it mask an earlier write error already being returned.
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("could not close report file %s: %w", fileName, cerr)
		}
	}()

	csvWriter := csv.NewWriter(file)
	if err := csvWriter.Write([]string{"Cloud Type", "Name", "Owner"}); err != nil {
		return fmt.Errorf("could not write report header: %w", err)
	}
	for _, item := range items {
		if err := csvWriter.Write([]string{item.GetCloudType().String(), item.GetName(), item.GetOwner()}); err != nil {
			return fmt.Errorf("could not write report row: %w", err)
		}
	}
	// Flush buffered rows and report any error deferred until now.
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("could not flush report %s: %w", fileName, err)
	}

	log.Infof("Writing of cloud items report is complete. Report filename: %s", fileName)
	return nil
}
