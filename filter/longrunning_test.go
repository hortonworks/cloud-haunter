package operation

import (
	"testing"
	"time"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

func TestLongRunningInit(t *testing.T) {
	assert.NotNil(t, ctx.Filters[types.LongRunningFilter])
}

func TestLongRunningFilter(t *testing.T) {
	now := time.Now()
	items := []types.CloudItem{
		&types.Instance{
			CloudType: types.AWS,
			Name:      "short running",
			Created:   now.Add(-defaultRunningPeriod).Add(1 * time.Second),
			State:     types.Running,
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "long running",
			Created:   now.Add(-defaultRunningPeriod).Add(-1 * time.Second),
			State:     types.Running,
		},
	}

	filteredItems := longRunning{defaultRunningPeriod}.Execute(items)

	assert.Equal(t, 1, len(filteredItems))
}

func TestLongRunningFilterOnResources(t *testing.T) {
	now := time.Now()
	items := []types.CloudItem{
		&types.Resource{
			CloudType:    types.AWS,
			Name:         "short running load balancer",
			Created:      now.Add(-defaultRunningPeriod).Add(1 * time.Second),
			ResourceType: types.LoadBalancer,
		},
		&types.Resource{
			CloudType:    types.AWS,
			Name:         "long running load balancer",
			Created:      now.Add(-defaultRunningPeriod).Add(-1 * time.Second),
			ResourceType: types.LoadBalancer,
		},
		&types.Resource{
			CloudType:    types.AWS,
			Name:         "VPC",
			ResourceType: types.Vpc,
		},
	}

	filteredItems := longRunning{defaultRunningPeriod}.Execute(items)

	assert.Equal(t, 2, len(filteredItems))
	assert.NotEqual(t, "short running load balancer", filteredItems[0].GetName())
	assert.NotEqual(t, "short running load balancer", filteredItems[1].GetName())
}
