package action

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestUnsetOsEnv(t *testing.T) {
	os.Unsetenv("RETENTION_DAYS")

	initCleanup()

	assert.Equal(t, ctx.Actions[types.CleanupAction].(cleanupAction).retentionDays, defaultRetentionDays)
}

func TestEmptyOsEnv(t *testing.T) {
	os.Setenv("RETENTION_DAYS", "")

	initCleanup()

	assert.Equal(t, ctx.Actions[types.CleanupAction].(cleanupAction).retentionDays, defaultRetentionDays)
}

func TestSetOsEnv(t *testing.T) {
	os.Setenv("RETENTION_DAYS", "30")

	initCleanup()

	assert.Equal(t, ctx.Actions[types.CleanupAction].(cleanupAction).retentionDays, 30)
}
