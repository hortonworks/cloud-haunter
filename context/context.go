package context

import (
	"github.com/hortonworks/cloud-cost-reducer/types"
)

var (
	Version   string
	BuildTime string

	AwsIgnoreLabel   string = "cloud-cost-reducer-ignore"
	AzureIgnoreLabel string = "cloud-cost-reducer-ignore"
	GcpIgnoreLabel   string = "cloud-cost-reducer-ignore"

	AwsOwnerLabel   string = "Owner"
	AzureOwnerLabel string = "Owner"
	GcpOwnerLabel   string = "owner"

	CreationTimeLabel string = "cb-creation-timestamp"
)

// DryRun is a global flag to skip concrete action
var DryRun = false

// Verbose is a global flag for verbose logging
var Verbose = false

var Operations = make(map[types.OpType]types.Operation)

var CloudProviders = make(map[types.CloudType]func() types.CloudProvider)

var IgnoreLabels = make(map[types.CloudType]string)

var OwnerLabels = make(map[types.CloudType]string)

var Dispatchers = make(map[string]types.Dispatcher)

var Actions = make(map[types.ActionType]types.Action)

func init() {
	IgnoreLabels[types.AWS] = AwsIgnoreLabel
	IgnoreLabels[types.AZURE] = AzureIgnoreLabel
	IgnoreLabels[types.GCP] = GcpIgnoreLabel

	OwnerLabels[types.AWS] = AwsOwnerLabel
	OwnerLabels[types.AZURE] = AzureOwnerLabel
	OwnerLabels[types.GCP] = GcpOwnerLabel
}
