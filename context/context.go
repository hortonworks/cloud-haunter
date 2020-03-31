package context

import "github.com/hortonworks/cloud-haunter/types"

var (
	// Version is global variable to store application version generated during release
	Version string
	// BuildTime is global variable to store build time generated during release
	BuildTime string

	// IgnoreLabel across all cloud providers
	IgnoreLabel = "cloud-cost-reducer-ignore"

	// OwnerLabel across all cloud providers
	OwnerLabel = "owner"

	// AzureCreationTimeLabel is the instance creation time on Azure, because Azure is stupid and doesn't tell
	AzureCreationTimeLabel = "creation-timestamp,cb-creation-timestamp,cdp-creation-timestamp"

	AwsBulkOperationSize = 50
)

// DryRun is a global flag to skip concrete action
var DryRun = false

// Verbose is a global flag for verbose logging
var Verbose = false

// Operations contains all the available operations
var Operations = make(map[types.OpType]types.Operation)

// Filters that can be applied on the operations
var Filters = make(map[types.FilterType]types.Filter)

// CloudProviders contains all the available cloud providers
var CloudProviders = make(map[types.CloudType]func() types.CloudProvider)

// Dispatchers contains all the available dispatchers
var Dispatchers = make(map[string]types.Dispatcher)

// Actions contains all the available actions
var Actions = make(map[types.ActionType]types.Action)

// FilterConfig contains the include/exclude configurations from config file
var FilterConfig *types.FilterConfig
