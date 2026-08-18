package action

import (
	"testing"

	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

// TestLogActionExecute is a smoke test: the log action only writes to the
// logger, so we exercise the account-name lookup and item-marshalling path and
// assert it completes without error.
func TestLogActionExecute(t *testing.T) {
	t.Parallel()
	mock := newMockProvider()
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})

	err := NewLog(cfg).Execute(types.Instances, nil, []types.CloudItem{
		types.Instance{CloudType: types.AWS, Name: "inst-1"},
	})

	assert.NoError(t, err)
}
