package main

import (
	"flag"
	"os"
	"sort"
	"strings"

	"github.com/hortonworks/cloud-haunter/utils"

	"github.com/hortonworks/cloud-haunter/action"
	"github.com/hortonworks/cloud-haunter/aws"
	"github.com/hortonworks/cloud-haunter/azure"
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/filter"
	"github.com/hortonworks/cloud-haunter/gcp"
	"github.com/hortonworks/cloud-haunter/operation"
	"github.com/hortonworks/cloud-haunter/slack"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// Version is the application version, injected at link time via -ldflags.
var Version string

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
	excludedAwsRegions := flag.String("excludeAwsRegion", "", "comma separated list of AWS regions to exclude")

	flag.Parse()

	awsExcludedRegions := parseRegionsStr(*excludedAwsRegions)

	if *verbose {
		log.SetLevel(log.DebugLevel)
	}
	if *dryRun {
		log.Warn("We are in dry run mode.")
	}
	var filterConfig types.IFilterConfig
	if filterConfigLoc != nil && len(*filterConfigLoc) != 0 {
		var err error
		filterConfig, err = utils.LoadFilterConfig(*filterConfigLoc)
		if err != nil {
			log.Warnf("[UTIL] Failed to load %s as V1 filter config, trying as V2. Error: %s", *filterConfigLoc, err.Error())
			filterConfig, err = utils.LoadFilterConfigV2(*filterConfigLoc)
			if err != nil {
				panic("Unable to parse filter configuration: " + err.Error())
			}
		}
	}

	cfg := &config.Config{
		FilterConfig:          filterConfig,
		IgnoreLabel:           config.DefaultIgnoreLabel,
		OwnerLabel:            config.DefaultOwnerLabel,
		IgnoreLabelDisabled:   *ignoreLabelDisabled,
		ExactMatchOwner:       *exactMatchOwner,
		DryRun:                *dryRun,
		ResourceGroupingLabel: config.DefaultResourceGroupingLabel,
		AwsExcludedRegions:    awsExcludedRegions,
		CloudProviders:        make(map[types.CloudType]func() types.CloudProvider),
		Dispatchers:           make(map[string]types.Dispatcher),
	}
	if err := cfg.LoadEnv(); err != nil {
		log.Fatalf("[MAIN] Invalid configuration: %s", err.Error())
	}
	operations, filters, actions := registerComponents(cfg)

	if *help {
		printHelp(operations, filters, actions)
		os.Exit(0)
	}

	op := func() *types.OpType {
		for i := range operations {
			if i.String() == *opType {
				return &i
			}
		}
		return nil
	}()
	if op == nil {
		panic("Operation is not found.")
	}

	var matchedFilters []types.Filter
	var filterNames []types.FilterType
	selectedFilters := utils.SplitListToMap(*filterTypes)
	for f := range filters {
		if _, ok := selectedFilters[f.String()]; ok {
			matchedFilters = append(matchedFilters, filters[f])
			filterNames = append(filterNames, f)
		}
	}

	action := func() types.Action {
		for i := range actions {
			if i.String() == *actionType {
				return actions[i]
			}
		}
		return nil
	}()
	if action == nil {
		panic("Action is not found.")
	}

	var clouds []types.CloudType
	selectedClouds := utils.SplitListToMap(*cloudTypes)
	for t := range cfg.CloudProviders {
		_, ok := selectedClouds[t.String()]
		if len(selectedClouds) == 0 || ok {
			clouds = append(clouds, t)
		} else {
			delete(cfg.CloudProviders, t)
		}
	}
	if len(clouds) == 0 {
		panic("Cloud provider not found.")
	}

	items, err := operations[*op].Execute(clouds)
	if err != nil {
		log.Fatalf("[MAIN] Operation %s failed: %s", op.String(), err.Error())
	}
	for _, filter := range matchedFilters {
		items, err = filter.Execute(items)
		if err != nil {
			log.Fatalf("[MAIN] Filter failed: %s", err.Error())
		}
	}
	if err := action.Execute(*op, filterNames, items); err != nil {
		log.Fatalf("[MAIN] Action %s failed: %s", *actionType, err.Error())
	}
	log.Info("Action completed.")
}

// registerComponents wires the available cloud providers, dispatchers, actions,
// operations and filters, injecting the shared cfg into each. Providers and the
// Slack dispatcher register themselves into cfg.CloudProviders / cfg.Dispatchers;
// the actions, operations and filters are returned as local registries that main
// owns. This inverts the previous pattern where each package registered itself
// from an init() function and read global state directly, keeping the mapping
// explicit and in one place.
func registerComponents(cfg *config.Config) (
	map[types.OpType]types.Operation,
	map[types.FilterType]types.Filter,
	map[types.ActionType]types.Action,
) {
	aws.Register(cfg)
	azure.Register(cfg)
	gcp.Register(cfg)
	slack.Register(cfg)

	actions := map[types.ActionType]types.Action{
		types.LogAction:              action.NewLog(cfg),
		types.Json:                   action.NewJSON(cfg),
		types.StopAction:             action.NewStop(cfg),
		types.TerminationAction:      action.NewTermination(cfg),
		types.NotificationAction:     action.NewNotification(cfg),
		types.CloudItemsReportAction: action.NewCloudItemsReport(),
		types.CleanupAction:          action.NewCleanup(cfg),
	}

	operations := map[types.OpType]types.Operation{
		types.Instances:   operation.NewInstances(cfg),
		types.CloudAccess: operation.NewAccess(cfg),
		types.Databases:   operation.NewDatabases(cfg),
		types.Disks:       operation.NewDisks(cfg),
		types.Images:      operation.NewImages(cfg),
		types.ReadImages:  operation.NewReadImages(),
		types.Stacks:      operation.NewStacks(cfg),
		types.Alerts:      operation.NewAlerts(cfg),
		types.Storages:    operation.NewStorages(cfg),
		types.Resources:   operation.NewResources(cfg),
	}

	filters := map[types.FilterType]types.Filter{
		types.OwnerlessFilter:   filter.NewOwnerless(cfg),
		types.RunningFilter:     filter.NewRunning(cfg),
		types.StoppedFilter:     filter.NewStopped(cfg),
		types.FailedFilter:      filter.NewFailed(cfg),
		types.UnusedFilter:      filter.NewUnused(cfg),
		types.MatchFilter:       filter.NewMatch(cfg),
		types.NoMatchFilter:     filter.NewNoMatch(cfg),
		types.LongRunningFilter: filter.NewLongRunning(cfg),
		types.OldAccessFilter:   filter.NewOldAccess(cfg),
	}

	return operations, filters, actions
}

// should be kept in sync with README.md
func printHelp(operations map[types.OpType]types.Operation, filters map[types.FilterType]types.Filter, actions map[types.ActionType]types.Action) {
	println(`NAME:
   Cloud Haunter
USAGE:
   ch -o=operation -a=action [-f=filter1,filter2] [-c=cloud1,cloud2]
VERSION:`)
	println("   " + Version)
	println(`
AUTHOR(S):
   Hortonworks
OPERATIONS:`)
	for _, o := range getSortedOperations(operations) {
		println("\t-o " + o)
	}
	println("FILTERS:")
	for _, f := range getSortedFilters(filters) {
		println("\t-f " + f)
	}
	println("ACTIONS:")
	for _, a := range getSortedActions(actions) {
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
	println("EXCLUDE AWS REGIONS:\n\t-excludeAwsRegion")
	println("HELP:\n\t-h")
}

func getSortedOperations(operations map[types.OpType]types.Operation) []string {
	names := []string{}
	for ot := range operations {
		names = append(names, string(ot))
	}
	sort.Strings(names)
	return names
}

func getSortedFilters(filters map[types.FilterType]types.Filter) []string {
	names := []string{}
	for f := range filters {
		names = append(names, string(f))
	}
	sort.Strings(names)
	return names
}

func getSortedActions(actions map[types.ActionType]types.Action) []string {
	names := []string{}
	for a := range actions {
		names = append(names, string(a))
	}
	sort.Strings(names)
	return names
}

func parseRegionsStr(regionsStr string) map[string]bool {
	excluded := make(map[string]bool)
	for _, item := range strings.Split(regionsStr, ",") {
		excluded[strings.ToLower(item)] = true
	}
	return excluded
}
