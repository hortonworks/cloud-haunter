package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/hortonworks/cloud-haunter/types"
)

const (
	defaultRetentionDays         = 90
	defaultLongRunningPeriod     = 24 * time.Hour
	defaultAccessAvailablePeriod = 120 * 24 * time.Hour
)

// Default label/tag names used across all cloud providers. They are declared as
// vars (not consts) so a release build can override them at link time via
// -ldflags "-X .../config.DefaultIgnoreLabel=...". main seeds the Config fields
// from these.
var (
	// DefaultIgnoreLabel marks an item to be skipped by filters.
	DefaultIgnoreLabel = "cloud-cost-reducer-ignore"

	// DefaultOwnerLabel identifies the owner of an item.
	DefaultOwnerLabel = "owner"

	// DefaultResourceGroupingLabel groups related resources on AWS and GCP.
	DefaultResourceGroupingLabel = "Cloudera-Environment-Resource-Name"
)

// Config carries the per-run settings and wired-in registries that the action,
// operation and filter components depend on. It is built once in main and
// injected into each component's constructor, replacing direct reads of the
// package-global state in the context package. This keeps those components free
// of shared mutable state so their tests can construct isolated instances and
// run in parallel.
type Config struct {
	// FilterConfig holds the include/exclude configuration loaded from the
	// filter config file (nil when none was provided).
	FilterConfig types.IFilterConfig

	// IgnoreLabel is the label/tag that marks an item to be skipped by filters.
	IgnoreLabel string

	// OwnerLabel is the label/tag that identifies the owner of an item.
	OwnerLabel string

	// ResourceGroupingLabel groups related resources on AWS and GCP.
	ResourceGroupingLabel string

	// AwsExcludedRegions holds AWS regions to skip, keyed by lowercase name.
	AwsExcludedRegions map[string]bool

	// IgnoreLabelDisabled disables honouring the ignore label when true.
	IgnoreLabelDisabled bool

	// ExactMatchOwner switches owner matching from "starts with" to "equals".
	ExactMatchOwner bool

	// DryRun, when true, makes actions/providers log what they would do instead
	// of mutating cloud resources.
	DryRun bool

	// RetentionDays is how long a storage item is kept before the cleanup action
	// removes it (RETENTION_DAYS).
	RetentionDays int

	// LongRunningPeriod is the age after which the longrunning filter considers a
	// resource long-running (RUNNING_PERIOD).
	LongRunningPeriod time.Duration

	// AccessAvailablePeriod is the age after which the oldaccess filter considers
	// an access old (ACCESS_AVAILABLE_PERIOD).
	AccessAvailablePeriod time.Duration

	// CloudProviders is the registry of available cloud provider factories.
	CloudProviders map[types.CloudType]func() types.CloudProvider

	// Dispatchers is the registry of available notification dispatchers.
	Dispatchers map[string]types.Dispatcher
}

// LoadEnv populates the environment-derived tunables (RETENTION_DAYS,
// RUNNING_PERIOD, ACCESS_AVAILABLE_PERIOD), applying defaults when a variable is
// unset. It returns an error if a variable is set but cannot be parsed, so the
// caller can fail fast at a single point instead of the components failing
// (inconsistently) deep inside their constructors.
func (c *Config) LoadEnv() error {
	return c.loadEnv(os.Getenv)
}

// loadEnv is the testable core of LoadEnv: the variable lookup is injected so
// tests can drive it from an in-memory map and run in parallel, instead of
// mutating the process environment via t.Setenv (which also forbids t.Parallel).
func (c *Config) loadEnv(getenv func(string) string) error {
	c.RetentionDays = defaultRetentionDays
	if v := getenv("RETENTION_DAYS"); v != "" {
		days, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid RETENTION_DAYS %q: %w", v, err)
		}
		c.RetentionDays = days
	}

	c.LongRunningPeriod = defaultLongRunningPeriod
	if v := getenv("RUNNING_PERIOD"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid RUNNING_PERIOD %q: %w", v, err)
		}
		c.LongRunningPeriod = d
	}

	c.AccessAvailablePeriod = defaultAccessAvailablePeriod
	if v := getenv("ACCESS_AVAILABLE_PERIOD"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid ACCESS_AVAILABLE_PERIOD %q: %w", v, err)
		}
		c.AccessAvailablePeriod = d
	}

	return nil
}
