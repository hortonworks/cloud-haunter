package action

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

// TestJSONActionExecute writes to an injected buffer and verifies the action
// prints the items as JSON. Because it no longer swaps os.Stdout, it is parallel.
func TestJSONActionExecute(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	mock := newMockProvider()
	cfg := providerCfg(map[types.CloudType]*mockProvider{types.AWS: mock})
	items := []types.CloudItem{
		types.Instance{CloudType: types.AWS, Name: "inst-1"},
	}

	assert.NoError(t, jsonAction{cfg: cfg, out: &buf}.Execute(types.Instances, nil, items))

	// The printed JSON must round-trip back to the same item names.
	var decoded []types.Instance
	assert.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Len(t, decoded, 1)
	assert.Equal(t, "inst-1", decoded[0].Name)
}
