package main

import (
	"flag"
	"os"
	"sort"

	"github.com/hortonworks/cloud-haunter/utils"

	_ "github.com/hortonworks/cloud-haunter/action"
	_ "github.com/hortonworks/cloud-haunter/aws"
	_ "github.com/hortonworks/cloud-haunter/azure"
	ctx "github.com/hortonworks/cloud-haunter/context"
	_ "github.com/hortonworks/cloud-haunter/filter"
	_ "github.com/hortonworks/cloud-haunter/gcp"
	_ "github.com/hortonworks/cloud-haunter/hipchat"
	_ "github.com/hortonworks/cloud-haunter/operation"
	_ "github.com/hortonworks/cloud-haunter/slack"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
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
	cloudTypes := flag.String("c", "", "type of clouds")
	filterConfigLoc := flag.String("fc", "", "filterConfig YAML")
	dryRun := flag.Bool("d", false, "dry run")
	verbose := flag.Bool("v", false, "verbose")
	ignoreLabelDisabled := flag.Bool("i", false, "disable ignore label")
	exactMatchOwner := flag.Bool("e", false, "exact match owner")

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
	ctx.IgnoreLabelDisabled = *ignoreLabelDisabled
	ctx.ExactMatchOwner = *exactMatchOwner

	if filterConfigLoc != nil && len(*filterConfigLoc) != 0 {
		var err error
		ctx.FilterConfig, err = utils.LoadFilterConfig(*filterConfigLoc)
		if err != nil {
			log.Warnf("[UTIL] Failed to load %s as V1 filter config, trying as V2. Error: %s", *filterConfigLoc, err.Error())
			ctx.FilterConfig, err = utils.LoadFilterConfigV2(*filterConfigLoc)
			if err != nil {
				panic("Unable to parse filter configuration: " + err.Error())
			}
		}
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
	selectedFilters := utils.SplitListToMap(*filterTypes)
	for f := range ctx.Filters {
		if _, ok := selectedFilters[f.String()]; ok {
			filters = append(filters, ctx.Filters[f])
			filterNames = append(filterNames, f)
		}
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

	var clouds []types.CloudType
	selectedClouds := utils.SplitListToMap(*cloudTypes)
	for t := range ctx.CloudProviders {
		_, ok := selectedClouds[t.String()]
		if len(selectedClouds) == 0 || ok {
			clouds = append(clouds, t)
		} else {
			delete(ctx.CloudProviders, t)
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

// should be kept in sync with README.md
func printHelp() {
	println(`NAME:
   Cloud Haunter
USAGE:
   ch -o=operation -a=action [-f=filter1,filter2] [-c=cloud1,cloud2]
VERSION:`)
	println("   " + ctx.Version)
	println(`
AUTHOR(S):
   Hortonworks, Tamr
OPERATIONS:`)
	for _, o := range getSortedOperations() {
		println("\t-o " + o)
	}
	println("FILTERS:")
	for _, f := range getSortedFilters() {
		println("\t-f " + f)
	}
	println("ACTIONS:")
	for _, a := range getSortedActions() {
		println("\t-a " + a)
	}
	println("CLOUDS:")
	println("\t-c AWS")
	println("\t-c AZURE")
	println("\t-c GCP")
	println("FILTER_CONFIG:\n\t-fc=/location/of/filter/config.yml")
	println("DRY RUN:\n\t-d")
	println("VERBOSE:\n\t-v")
	println("DISABLE_IGNORE_LABEL:\n\t-i")
	println("EXACT_MATCH_OWNERS:\n\t-e")
	println("HELP:\n\t-h")
}

func getSortedOperations() []string {
	operations := []string{}
	for ot := range ctx.Operations {
		operations = append(operations, string(ot))
	}
	sort.Strings(operations)
	return operations
}

func getSortedFilters() []string {
	filters := []string{}
	for f := range ctx.Filters {
		filters = append(filters, string(f))
	}
	sort.Strings(filters)
	return filters
}

func getSortedActions() []string {
	actions := []string{}
	for a := range ctx.Actions {
		actions = append(actions, string(a))
	}
	sort.Strings(actions)
	return actions
}
