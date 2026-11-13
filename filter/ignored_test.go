package filter

import (
	"testing"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

func TestIgnoredKeepsLabelledAndResourceGroupIgnoredItems(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{IgnoreLabel: "ignore"}
	inheritsIgnore := map[string]string{types.ResourceGroupIgnoredMetadata: "true"}
	items := []types.CloudItem{
		&types.Instance{Name: "labelled-instance", Tags: types.Tags{"ignore": "true"}},
		&types.Database{Name: "labelled-database", Tags: types.Tags{"ignore": ""}},
		&types.Disk{Name: "labelled-disk", Tags: types.Tags{"ignore": "true"}},
		&types.Instance{Name: "group-ignored-instance", Metadata: inheritsIgnore},
		&types.Database{Name: "group-ignored-database", Metadata: inheritsIgnore},
		&types.Instance{Name: "plain-instance", Tags: types.Tags{"owner": "someone"}},
		&types.Database{Name: "group-not-ignored-database", Metadata: map[string]string{types.ResourceGroupIgnoredMetadata: "false"}},
		&types.Stack{Name: "labelled-stack-without-metadata", Tags: types.Tags{"ignore": "true"}},
		&types.Stack{Name: "plain-stack"},
	}

	result, err := ignored{cfg}.Execute(items)

	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"labelled-instance", "labelled-database", "labelled-disk",
		"group-ignored-instance", "group-ignored-database", "labelled-stack-without-metadata"}, filterNames(result))
}

func TestIgnoredDoesNotDependOnIgnoreLabelDisabled(t *testing.T) {
	t.Parallel()
	items := []types.CloudItem{&types.Instance{Name: "labelled", Tags: types.Tags{"ignore": "true"}}}

	enabled, _ := ignored{&config.Config{IgnoreLabel: "ignore"}}.Execute(items)
	disabled, _ := ignored{&config.Config{IgnoreLabel: "ignore", IgnoreLabelDisabled: true}}.Execute(items)

	assert.Equal(t, []string{"labelled"}, filterNames(enabled))
	assert.Equal(t, []string{"labelled"}, filterNames(disabled))
}
