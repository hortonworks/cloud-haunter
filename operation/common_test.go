package operation

import (
	"errors"
	"testing"

	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

func TestCollect(t *testing.T) {
	t.Parallel()
	clouds := []types.CloudType{types.DUMMY}
	getter := func(p types.CloudProvider) (i []types.CloudItem, e error) {
		return []types.CloudItem{types.Instance{Name: "instance"}}, nil
	}

	registry := providerRegistry(map[types.CloudType]types.CloudProvider{types.DUMMY: &mockProvider{}})
	itemsChan, _ := collect(registry, clouds, getter)

	items := [][]types.CloudItem{
		<-itemsChan,
		<-itemsChan,
	}

	assert.Equal(t, "instance", items[0][0].GetName())
}

// TestWait drains both channels: items are collected and errors are logged (and
// otherwise ignored). Pre-filling then closing the buffered channels lets wait
// run synchronously and deterministically.
func TestWait(t *testing.T) {
	t.Parallel()
	itemsChan := make(chan []types.CloudItem, 10)
	errChan := make(chan error, 5)
	itemsChan <- []types.CloudItem{types.Instance{Name: "instance"}}
	errChan <- errors.New("some error")
	close(itemsChan)
	close(errChan)

	items := wait(itemsChan, errChan, "error message")

	assert.Equal(t, "instance", items[0].GetName())
}

func TestConvertInstancesToCloudItems(t *testing.T) {
	t.Parallel()
	instances := []*types.Instance{{}}

	items := convertInstancesToCloudItems(instances)

	assert.Equal(t, len(instances), len(items))
}
