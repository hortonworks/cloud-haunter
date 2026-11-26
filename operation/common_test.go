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

// TestWait drains both channels and returns the collected items. Pre-filling then
// closing the buffered channels lets wait run synchronously and deterministically.
func TestWait(t *testing.T) {
	t.Parallel()
	itemsChan := make(chan []types.CloudItem, 10)
	errChan := make(chan error, 5)
	itemsChan <- []types.CloudItem{types.Instance{Name: "instance-1"}}
	itemsChan <- []types.CloudItem{types.Instance{Name: "instance-2"}}
	close(itemsChan)
	close(errChan)

	items, err := wait(itemsChan, errChan, "error message")

	assert.NoError(t, err)
	assert.Len(t, items, 2)
}

// TestWaitReturnsEveryProviderError verifies that wait drains both channels and
// returns every provider error, and no items, so the caller cannot act on a partial
// inventory.
func TestWaitReturnsEveryProviderError(t *testing.T) {
	t.Parallel()
	itemsChan := make(chan []types.CloudItem, 10)
	errChan := make(chan error, 5)
	itemsChan <- []types.CloudItem{types.Instance{Name: "instance"}}
	errChan <- errors.New("first error")
	errChan <- errors.New("second error")
	close(itemsChan)
	close(errChan)

	items, err := wait(itemsChan, errChan, "error message")

	assert.ErrorContains(t, err, "error message")
	assert.ErrorContains(t, err, "first error")
	assert.ErrorContains(t, err, "second error")
	assert.Nil(t, items)
}

func TestConvertInstancesToCloudItems(t *testing.T) {
	t.Parallel()
	instances := []*types.Instance{{}}

	items := convertInstancesToCloudItems(instances)

	assert.Equal(t, len(instances), len(items))
}

func TestWithCloudPrefix(t *testing.T) {
	t.Parallel()
	assert.EqualError(t, withCloudPrefix(types.AWS, errors.New("AccessDenied")), "[AWS] AccessDenied")
	assert.EqualError(t, withCloudPrefix(types.AZURE, errors.New("[AZURE] Getting disks: not supported")), "[AZURE] Getting disks: not supported")
	assert.ErrorIs(t, withCloudPrefix(types.AWS, types.ErrNotSupported), types.ErrNotSupported)
}
