package context

import "github.com/hortonworks/cloud-haunter/types"

var (
	// Version is global variable to store application version generated during release
	Version string
	// BuildTime is global variable to store build time generated during release
	BuildTime string

	// AwsIgnoreLabel is the instance label on AWS to skip from operation
	AwsIgnoreLabel = "cloud-cost-reducer-ignore"
	// AzureIgnoreLabel is the instance label on Azure to skip from operation
	AzureIgnoreLabel = "cloud-cost-reducer-ignore"
	// GcpIgnoreLabel is the instance label on GCP to skip from operation
	GcpIgnoreLabel = "cloud-cost-reducer-ignore"

	// AwsOwnerLabel is the owner label on AWS
	AwsOwnerLabel = "Owner"
	// AzureOwnerLabel is the owner label on Azure
	AzureOwnerLabel = "Owner"
	// GcpOwnerLabel is the owner label on GCP
	GcpOwnerLabel = "owner"

	// AzureCreationTimeLabel is the instance creation tim on Azure, because Azure is stupid and doesn't tell
	AzureCreationTimeLabel = "cb-creation-timestamp"
)

// DryRun is a global flag to skip concrete action
var DryRun = false

// Verbose is a global flag for verbose logging
var Verbose = false

// Operations contains all the available operations
var Operations = make(map[types.OpType]types.Operation)

// CloudProviders contains all the available cloud providers
var CloudProviders = make(map[types.CloudType]func() types.CloudProvider)

// IgnoreLabels contains all the ignore labes by cloud
var IgnoreLabels = make(map[types.CloudType]string)

// OwnerLabels contains all the owner labes by cloud
var OwnerLabels = make(map[types.CloudType]string)

// Dispatchers contains all the available dispatchers
var Dispatchers = make(map[string]types.Dispatcher)

// Actions contains all the available actions
var Actions = make(map[types.ActionType]types.Action)

// Ignores contains the ignore configurations from config file
var Ignores *types.Ignores

func init() {
	IgnoreLabels[types.AWS] = AwsIgnoreLabel
	IgnoreLabels[types.AZURE] = AzureIgnoreLabel
	IgnoreLabels[types.GCP] = GcpIgnoreLabel

	OwnerLabels[types.AWS] = AwsOwnerLabel
	OwnerLabels[types.AZURE] = AzureOwnerLabel
	OwnerLabels[types.GCP] = GcpOwnerLabel
}
