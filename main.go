package main

import (
	"flag"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	_ "github.com/hortonworks/cloud-cost-reducer/aws"
	_ "github.com/hortonworks/cloud-cost-reducer/azure"
	"github.com/hortonworks/cloud-cost-reducer/context"
	_ "github.com/hortonworks/cloud-cost-reducer/gcp"
	_ "github.com/hortonworks/cloud-cost-reducer/hipchat"
	_ "github.com/hortonworks/cloud-cost-reducer/operation"
	"github.com/hortonworks/cloud-cost-reducer/types"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
			os.Exit(1)
		}
	}()

	help := flag.Bool("h", false, "print help")
	opType := flag.String("o", types.HELP.String(), "type of operation")
	cloudType := flag.String("c", "", "type of cloud")
	dryRun := flag.Bool("d", false, "dry run")

	flag.Parse()

	context.DRY_RUN = *dryRun

	if *help {
		opType = &(&types.S{S: types.HELP.String()}).S
	}

	op := func() types.Operation {
		for ot := range context.Operations {
			if ot.String() == *opType {
				return context.Operations[ot]
			}
		}
		return nil
	}()
	if op == nil {
		panic("Operation is not supported.")
	}

	clouds := []types.CloudType{}
	for t := range context.CloudProviders {
		if len(*cloudType) == 0 || t.String() == strings.ToUpper(*cloudType) {
			clouds = append(clouds, t)
		}
	}
	if len(clouds) == 0 {
		panic("Cloud provider not found.")
	}

	op.Execute(clouds)
}
