package config

import (
	"math"
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
	assert.Equal(t, defaultBudgetPeriod, cfg.BudgetPeriod)
	assert.Equal(t, defaultBudgetLimit, cfg.BudgetLimit)
}

// TestLoadEnvParsesValues verifies each variable is read when set.
func TestLoadEnvParsesValues(t *testing.T) {
	t.Parallel()
	cfg := &Config{}

	assert.NoError(t, cfg.loadEnv(envMap(map[string]string{
		"RETENTION_DAYS":          "30",
		"RUNNING_PERIOD":          "48h",
		"ACCESS_AVAILABLE_PERIOD": "72h",
		"BUDGET_PERIOD":           "72h",
		"BUDGET_LIMIT":            "49.5",
	})))
	assert.Equal(t, 30, cfg.RetentionDays)
	assert.Equal(t, 48*time.Hour, cfg.LongRunningPeriod)
	assert.Equal(t, 72*time.Hour, cfg.AccessAvailablePeriod)
	assert.Equal(t, 72*time.Hour, cfg.BudgetPeriod)
	assert.Equal(t, 49.5, cfg.BudgetLimit)
}

// TestLoadEnvErrors verifies a malformed value for any variable is reported.
func TestLoadEnvErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"RETENTION_DAYS":          "not-a-number",
		"RUNNING_PERIOD":          "not-a-duration",
		"ACCESS_AVAILABLE_PERIOD": "not-a-duration",
		"BUDGET_PERIOD":           "not-a-duration",
		"BUDGET_LIMIT":            "not-a-number",
	}
	for key, bad := range cases {
		key, bad := key, bad
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			assert.Error(t, (&Config{}).loadEnv(envMap(map[string]string{key: bad})))
		})
	}
}

// TestLoadEnvRejectsBudgetPeriodShorterThanADay verifies the budget period
// cannot be shorter than the daily granularity of the billing data.
func TestLoadEnvRejectsBudgetPeriodShorterThanADay(t *testing.T) {
	t.Parallel()
	assert.Error(t, (&Config{}).loadEnv(envMap(map[string]string{"BUDGET_PERIOD": "23h"})))
}

// TestLoadEnvBudgetLimit verifies BUDGET_LIMIT accepts only finite, non-negative
// amounts: NaN, infinities and negatives would corrupt the budget comparison.
func TestLoadEnvBudgetLimit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		value   string
		want    float64
		wantErr bool
	}{
		{name: "zero is valid", value: "0", want: 0},
		{name: "positive decimal is valid", value: "49.5", want: 49.5},
		{name: "positive integer is valid", value: "100", want: 100},
		{name: "largest finite float64 is valid", value: "1.7976931348623157e308", want: math.MaxFloat64},
		{name: "overflow beyond float64 is rejected", value: "1e400", wantErr: true},
		{name: "NaN is rejected", value: "NaN", wantErr: true},
		{name: "lowercase nan is rejected", value: "nan", wantErr: true},
		{name: "positive infinity is rejected", value: "+Inf", wantErr: true},
		{name: "unsigned infinity is rejected", value: "Inf", wantErr: true},
		{name: "negative infinity is rejected", value: "-Inf", wantErr: true},
		{name: "negative integer is rejected", value: "-1", wantErr: true},
		{name: "negative fraction is rejected", value: "-0.01", wantErr: true},
		{name: "malformed text is rejected", value: "abc", wantErr: true},
		{name: "comma decimal separator is rejected", value: "1,5", wantErr: true},
		{name: "trailing currency suffix is rejected", value: "100USD", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{}

			err := cfg.loadEnv(envMap(map[string]string{"BUDGET_LIMIT": tc.value}))

			if tc.wantErr {
				assert.ErrorContains(t, err, "must be a finite, non-negative number")
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, cfg.BudgetLimit)
		})
	}
}
