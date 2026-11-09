package operation

import (
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

// NewBillableResources returns the getBillableResources operation implementation.
func NewBillableResources(cfg *config.Config) types.Operation {
	return billableResources{[]types.Operation{NewInstances(cfg), NewDatabases(cfg), NewDisks(cfg)}}
}

type billableResources struct {
	operations []types.Operation
}

func (o billableResources) Execute(clouds []types.CloudType) ([]types.CloudItem, error) {
	log.Debugf("[GET_BILLABLE_RESOURCES] Collecting instances, databases and disks on: [%s]", clouds)
	var items []types.CloudItem
	for _, operation := range o.operations {
		collected, err := operation.Execute(clouds)
		if err != nil {
			return nil, err
		}
		items = append(items, collected...)
	}
	return items, nil
}
