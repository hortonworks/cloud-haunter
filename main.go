package main

import (
	"flag"
	"os"
	"strings"

	"github.com/hortonworks/cloud-haunter/utils"

	log "github.com/Sirupsen/logrus"
	_ "github.com/hortonworks/cloud-haunter/action"
	_ "github.com/hortonworks/cloud-haunter/aws"
	_ "github.com/hortonworks/cloud-haunter/azure"
	ctx "github.com/hortonworks/cloud-haunter/context"
	_ "github.com/hortonworks/cloud-haunter/filter"
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
	filterTypes := flag.String("f", "", "type of filters")
	actionType := flag.String("a", "log", "type of action")
	cloudType := flag.String("c", "", "type of cloud")
	ignoresLoc := flag.String("i", "", "ignores YAML")
	dryRun := flag.Bool("d", false, "dry run")
	verbose := flag.Bool("v", false, "verbose")

	flag.Parse()

	if *help {
		printHelp()
		os.Exit(0)
	}

	if ignoresLoc != nil && len(*ignoresLoc) != 0 {
		var err error
		ctx.Ignores, err = utils.LoadIgnores(*ignoresLoc)
		if err != nil {
			panic("Unable to parse ignore configuration: " + err.Error())
		}
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

	var filters []types.Filter
	var filterNames []types.FilterType
	if filterTypes != nil {
		filters = func() (f []types.Filter) {
			for filter := range ctx.Filters {
				if strings.Contains(*filterTypes, filter.String()) {
					f = append(f, ctx.Filters[filter])
					filterNames = append(filterNames, filter)
				}
			}
			return f
		}()
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

	items := ctx.Operations[*op].Execute(clouds)
	for _, filter := range filters {
		items = filter.Execute(items)
	}
	action.Execute(*op, filterNames, items)
}

func printHelp() {
	println(`NAME:
   Cloud Haunter
USAGE:
   ch -o=operation -a=action [-f=filter1,filter2] [-c=cloud]
VERSION:`)
	println("   " + ctx.Version)
	println(`
AUTHOR(S):
   Hortonworks
OPERATIONS:`)
	for ot := range ctx.Operations {
		println("\t-o " + ot.String())
	}
	println("FILTERS:")
	for f := range ctx.Filters {
		println("\t-f " + f.String())
	}
	println("ACTIONS:")
	for a := range ctx.Actions {
		println("\t-a " + a.String())
	}
	println("CLOUDS:")
	for ct := range ctx.CloudProviders {
		println("\t-c " + ct.String())
	}
	println("IGNORE:\n\t-i=/location/of/ignore/config.yml")
	println("DRY RUN:\n\t-d")
	println("VERBOSE:\n\t-v")
	println("HELP:\n\t-h")
}
