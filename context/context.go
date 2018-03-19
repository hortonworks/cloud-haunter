package context

import (
	"github.com/hortonworks/cloud-cost-reducer/types"
)

var (
	Version   string
	BuildTime string
)

var DryRun = false

var Operations = make(map[types.OpType]types.Operation)

var CloudProviders = make(map[types.CloudType]types.CloudProvider)

var Dispatchers = make(map[string]types.Dispatcher)

var Actions = make(map[types.ActionType]types.Action)
