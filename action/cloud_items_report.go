package action

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"time"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Actions[types.CloudItemsReportAction] = new(CloudItemsReportAction)
}

type CloudItemsReportAction struct {
}

func (a CloudItemsReportAction) Execute(op types.OpType, filter []types.FilterType, items []types.CloudItem) {
	isoTimestamp := time.Now().UTC().Format(time.RFC3339)
	isoTimestamp = strings.ReplaceAll(isoTimestamp, ":", "-") // colon is disallowed on some platforms (e.g. MS/Windows).
	fileName := fmt.Sprintf("report-cloud-items-%s.csv", isoTimestamp)

	log.Info("Generating cloud items report.")

	file, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("Could not create file: %s", err)
		return
	}
	defer file.Close()
	csvWriter := csv.NewWriter(file)
	defer csvWriter.Flush()
	csvWriter.Write([]string{"Cloud Type", "Name", "Owner"})
	for _, item := range items {
		csvWriter.Write([]string{item.GetCloudType().String(), item.GetName(), item.GetOwner()})
	}
	log.Infof("Written cloud items report to %s", fileName)
}
