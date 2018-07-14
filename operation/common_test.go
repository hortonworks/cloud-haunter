package operation

import (
	"errors"
	"testing"
	"time"

	"github.com/hortonworks/cloud-haunter/utils"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

func init() {
	ctx.CloudProviders[types.DUMMY] = func() types.CloudProvider {
		return dummyProvider{}
	}
}
func TestCollect(t *testing.T) {
	clouds := []types.CloudType{types.DUMMY}
	getter := func(p types.CloudProvider) (i []types.CloudItem, e error) {
		return []types.CloudItem{types.Instance{Name: "instance"}}, nil
	}

	itemsChan, _ := collect(clouds, getter)

	items := [][]types.CloudItem{
		<-itemsChan,
		<-itemsChan,
	}

	assert.Equal(t, "instance", items[0][0].GetName())
}

func TestWait(t *testing.T) {
	itemsChan := make(chan []types.CloudItem, 10)
	errChan := make(chan error, 5)

	go func() {
		items := wait(itemsChan, errChan, "error message")

		assert.Equal(t, "instance", items[0].GetName())
	}()

	itemsChan <- []types.CloudItem{types.Instance{Name: "instance"}}
	errChan <- errors.New("some error")

	time.Sleep(100 * time.Millisecond)

	close(itemsChan)
	close(errChan)
}

func TestFilter(t *testing.T) {
	items := []types.CloudItem{
		&types.Instance{},
	}

	filtered := filter(items, func(types.CloudItem) bool {
		return true
	})

	assert.Equal(t, len(items), len(filtered))
}

func TestIsIgnored(t *testing.T) {
	items := []types.CloudItem{
		&types.Instance{
			CloudType: types.AWS,
			Name:      "ignored-by-ignore-label",
			Tags:      types.Tags{ctx.AwsIgnoreLabel: "true"},
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "skipThisName-0",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "ignored-by-owner",
			Owner:     "skipThisOwner-0",
		},
		&types.Instance{
			CloudType: types.AWS,
			Name:      "ignored-by-label",
			Tags:      types.Tags{"skipThisLabel-0": "true"},
		},
		&types.Access{
			CloudType: types.AWS,
			Name:      "skipThisAccess-0",
		},
		&types.Access{
			CloudType: types.AWS,
			Name:      "ignored-by-owner",
			Owner:     "skipThisOwner-0",
		},
	}
	ignores, _ := utils.LoadIgnores("testdata/sample-ignore.yml")

	for _, item := range items {
		isIgnored := isIgnored(item, ignores)
		assert.True(t, isIgnored, "Item found: "+item.GetName())
	}
}

func TestConvertInstancesToCloudItems(t *testing.T) {
	instances := []*types.Instance{{}}

	items := convertInstancesToCloudItems(instances)

	assert.Equal(t, len(instances), len(items))
}

type dummyProvider struct {
}

func (p dummyProvider) GetRunningInstances() (i []*types.Instance, e error) {
	return
}

func (p dummyProvider) TerminateInstances(i []*types.Instance) (e error) {
	return
}

func (p dummyProvider) GetAccesses() (a []*types.Access, e error) {
	return
}
