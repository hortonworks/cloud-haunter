package main

import (
	"flag"
	"os"

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
	opType := flag.String("op", "help", "type of operation")

	flag.Parse()

	if *help {
		opType = &(&types.S{S: "help"}).S
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
	op.Execute()
}
