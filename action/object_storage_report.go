package action

import (
	"encoding/csv"
	"os"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

func init() {
	ctx.Actions[types.StorageReportAction] = new(StorageReportAction)
}

type StorageReportAction struct {
}

func (a StorageReportAction) Execute(op types.OpType, filter []types.FilterType, items []types.CloudItem) {
	log.Info("Generating report.")
	file, err := os.Create("report.csv")
	if err != nil {
		log.Fatalf("Could not create file: %s", err)
		return
	}
	defer file.Close()
	csvWriter := csv.NewWriter(file)
	defer csvWriter.Flush()
	for _, item := range items {
		csvWriter.Write([]string{item.GetCloudType().String(), item.GetName()})
		log.Infof("[%s] %s", item.GetCloudType(), item.GetName())
	}
	log.Info("Report generation complete.")
}
