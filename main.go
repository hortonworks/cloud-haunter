package main

import (
	"flag"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	_ "github.com/hortonworks/cloud-haunter/action"
	_ "github.com/hortonworks/cloud-haunter/aws"
	_ "github.com/hortonworks/cloud-haunter/azure"
	ctx "github.com/hortonworks/cloud-haunter/context"
	_ "github.com/hortonworks/cloud-haunter/gcp"
	_ "github.com/hortonworks/cloud-haunter/hipchat"
	_ "github.com/hortonworks/cloud-haunter/operation"
	"github.com/hortonworks/cloud-haunter/types"
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
	actionType := flag.String("a", "log", "type of action")
	cloudType := flag.String("c", "", "type of cloud")
	dryRun := flag.Bool("d", false, "dry run")
	verbose := flag.Bool("v", false, "verbose")

	flag.Parse()

	if *help {
		printHelp()
		os.Exit(0)
	}

	ctx.DryRun = *dryRun
	ctx.Verbose = *verbose
	if ctx.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	op := func() *types.OpType {
		for i := range ctx.Operations {
			if i.String() == *opType {
				return &i
			}
		}
		return nil
	}()
	if op == nil {
		panic("Operation is not found.")
	}

	action := func() types.Action {
		for i := range ctx.Actions {
			if i.String() == *actionType {
				return ctx.Actions[i]
			}
		}
		return nil
	}()
	if action == nil {
		panic("Action is not found.")
	}

	clouds := []types.CloudType{}
	for t := range ctx.CloudProviders {
		if len(*cloudType) == 0 || t.String() == strings.ToUpper(*cloudType) {
			clouds = append(clouds, t)
		}
	}
	if len(clouds) == 0 {
		panic("Cloud provider not found.")
	}

	action.Execute(op, ctx.Operations[*op].Execute(clouds))
}

func printHelp() {
	println(`NAME:
   Cloud Haunter
USAGE:
   ch -o=operation -a=action [-c=cloud]
VERSION:`)
	println("   " + ctx.Version)
	println(`
AUTHOR(S):
   Hortonworks
OPERATIONS:`)
	for ot := range ctx.Operations {
		println("\t-o " + ot.String())
	}
	println("ACTIONS:")
	for a := range ctx.Actions {
		println("\t-a " + a.String())
	}
	println("CLOUDS:")
	for ct := range ctx.CloudProviders {
		println("\t-c " + ct.String())
	}
	println("DRY RUN:\n\t-d")
	println("VERBOSE:\n\t-v")
	println("HELP:\n\t-p")
}
