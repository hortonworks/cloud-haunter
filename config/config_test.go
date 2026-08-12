package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// envMap returns a getenv func backed by m, so tests exercise loadEnv without
// touching the process environment (which would forbid t.Parallel).
func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// TestLoadEnvDefaults verifies the fallbacks applied when no variable is set.
func TestLoadEnvDefaults(t *testing.T) {
	t.Parallel()
	cfg := &Config{}

	assert.NoError(t, cfg.loadEnv(envMap(nil)))
	assert.Equal(t, defaultRetentionDays, cfg.RetentionDays)
	assert.Equal(t, defaultLongRunningPeriod, cfg.LongRunningPeriod)
	assert.Equal(t, defaultAccessAvailablePeriod, cfg.AccessAvailablePeriod)
}

// TestLoadEnvParsesValues verifies each variable is read when set.
func TestLoadEnvParsesValues(t *testing.T) {
	t.Parallel()
	cfg := &Config{}

	assert.NoError(t, cfg.loadEnv(envMap(map[string]string{
		"RETENTION_DAYS":          "30",
		"RUNNING_PERIOD":          "48h",
		"ACCESS_AVAILABLE_PERIOD": "72h",
	})))
	assert.Equal(t, 30, cfg.RetentionDays)
	assert.Equal(t, 48*time.Hour, cfg.LongRunningPeriod)
	assert.Equal(t, 72*time.Hour, cfg.AccessAvailablePeriod)
}

// TestLoadEnvErrors verifies a malformed value for any variable is reported.
func TestLoadEnvErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"RETENTION_DAYS":          "not-a-number",
		"RUNNING_PERIOD":          "not-a-duration",
		"ACCESS_AVAILABLE_PERIOD": "not-a-duration",
	}
	for key, bad := range cases {
		key, bad := key, bad
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			assert.Error(t, (&Config{}).loadEnv(envMap(map[string]string{key: bad})))
		})
	}
}
