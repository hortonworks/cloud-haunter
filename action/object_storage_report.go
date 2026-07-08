package action

import (
	"encoding/csv"
	"fmt"
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
	file, err := os.Create(fmt.Sprintf("report.csv"))
	if err != nil {
		log.Fatalf("Could not create file: %s", err)
		return
	}
	defer file.Close()
	csvWriter := csv.NewWriter(file)
	defer csvWriter.Flush()

	for _, item := range items {
		csvWriter.Write([]string{item.GetCloudType().String(), item.G, item.GetName()})
		log.Infof("[%s] %s", item.GetCloudType(), item.GetName())
	}
}
