package main

import (
	"flag"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	_ "github.com/hortonworks/cloud-cost-reducer/action"
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
	opType := flag.String("o", "", "type of operation")
	actionType := flag.String("a", "", "type of action")
	cloudType := flag.String("c", "", "type of cloud")
	dryRun := flag.Bool("d", false, "dry run")

	flag.Parse()

	context.DryRun = *dryRun

	if *help {
		printHelp()
		os.Exit(0)
	}

	op := func() types.Operation {
		for i := range context.Operations {
			if i.String() == *opType {
				return context.Operations[i]
			}
		}
		return nil
	}()
	if op == nil {
		panic("Operation is not supported.")
	}

	action := func() types.Action {
		for i := range context.Actions {
			if i.String() == *actionType {
				return context.Actions[i]
			}
		}
		return nil
	}()
	if action == nil {
		panic("Action is not supported.")
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

	action.Execute(*opType, op.Execute(clouds))
}

func printHelp() {
	println(`NAME:
   Cloud Cost Reducer
USAGE:
   ccr -o=operation -a=action [-c=cloud]
   
VERSION:`)
	println(context.Version)
	println(`
AUTHOR(S):
   Hortonworks
   
OPERATIONS:`)
	for ot := range context.Operations {
		println("\t-o=" + ot.String())
	}
	println("ACTIONS:")
	for a := range context.Actions {
		println("\t-a=" + a.String())
	}
	println("CLOUDS:")
	for ct := range context.CloudProviders {
		println("\t-c=" + ct.String())
	}
	println("Dry run:\n\t-d")
	println("Print help:\n\t-p")
}
