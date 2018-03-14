package main

import (
	"flag"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	_ "github.com/hortonworks/cloud-cost-reducer/aws"
	_ "github.com/hortonworks/cloud-cost-reducer/azure"
	_ "github.com/hortonworks/cloud-cost-reducer/gcp"
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

	flag.Parse()

	if *help {
		opType = &(&types.S{S: types.HELP.String()}).S
	}

	op := func() types.Operation {
		for ot := range types.Operations {
			if ot.String() == *opType {
				return types.Operations[ot]
			}
		}
		return nil
	}()
	if op == nil {
		panic("Operation is not supported.")
	}

	clouds := []types.CloudType{}
	for t := range types.CloudProviders {
		if len(*cloudType) == 0 || t.String() == strings.ToUpper(*cloudType) {
			clouds = append(clouds, t)
		}
	}
	if len(clouds) == 0 {
		panic("Cloud provider not found.")
	}

	op.Execute(clouds)
}
