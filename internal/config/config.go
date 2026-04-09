package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Default values for all config fields.
const (
	DefaultJig                  = "feature"
	DefaultSnapshotsEnabled     = true
	DefaultIntervalEnabled      = false
	DefaultIntervalSeconds      = 300
	DefaultMaxSnapshots         = 100
	DefaultStaleThresholdHours  = 24
	DefaultRepoSpecPath         = ".kerf/{codename}/"
)

// Config represents ~/.kerf/config.yaml.
type Config struct {
	DefaultJig     string          `yaml:"default_jig,omitempty"`
	DefaultProject string          `yaml:"default_project,omitempty"`
	Snapshots      SnapshotsConfig `yaml:"snapshots,omitempty"`
	Sessions       SessionsConfig  `yaml:"sessions,omitempty"`
	Finalize       FinalizeConfig  `yaml:"finalize,omitempty"`
}

// SnapshotsConfig holds snapshot-related settings.
type SnapshotsConfig struct {
	Enabled         *bool `yaml:"enabled,omitempty"`
	IntervalEnabled *bool `yaml:"interval_enabled,omitempty"`
	IntervalSeconds *int  `yaml:"interval_seconds,omitempty"`
	MaxSnapshots    *int  `yaml:"max_snapshots,omitempty"`
}

// SessionsConfig holds session-related settings.
type SessionsConfig struct {
	StaleThresholdHours *int `yaml:"stale_threshold_hours,omitempty"`
}

// FinalizeConfig holds finalization settings.
type FinalizeConfig struct {
	RepoSpecPath string `yaml:"repo_spec_path,omitempty"`
}

// Effective returns the value with default applied.
func (c *Config) EffectiveDefaultJig() string {
	if c.DefaultJig != "" {
		return c.DefaultJig
	}
	return DefaultJig
}

func (c *Config) EffectiveSnapshotsEnabled() bool {
	if c.Snapshots.Enabled != nil {
		return *c.Snapshots.Enabled
	}
	return DefaultSnapshotsEnabled
}

func (c *Config) EffectiveIntervalEnabled() bool {
	if c.Snapshots.IntervalEnabled != nil {
		return *c.Snapshots.IntervalEnabled
	}
	return DefaultIntervalEnabled
}

func (c *Config) EffectiveIntervalSeconds() int {
	if c.Snapshots.IntervalSeconds != nil {
		return *c.Snapshots.IntervalSeconds
	}
	return DefaultIntervalSeconds
}

func (c *Config) EffectiveMaxSnapshots() int {
	if c.Snapshots.MaxSnapshots != nil {
		return *c.Snapshots.MaxSnapshots
	}
	return DefaultMaxSnapshots
}

func (c *Config) EffectiveStaleThresholdHours() int {
	if c.Sessions.StaleThresholdHours != nil {
		return *c.Sessions.StaleThresholdHours
	}
	return DefaultStaleThresholdHours
}

func (c *Config) EffectiveRepoSpecPath() string {
	if c.Finalize.RepoSpecPath != "" {
		return c.Finalize.RepoSpecPath
	}
	return DefaultRepoSpecPath
}

// Load parses config.yaml from disk. Returns defaults if file is missing.
func Load(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

// Save serializes config to disk.
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// validKeys enumerates all known dot-notation config keys.
var validKeys = []string{
	"default_jig",
	"default_project",
	"snapshots.enabled",
	"snapshots.interval_enabled",
	"snapshots.interval_seconds",
	"snapshots.max_snapshots",
	"sessions.stale_threshold_hours",
	"finalize.repo_spec_path",
}

// ValidKeys returns all known configuration keys.
func ValidKeys() []string {
	return append([]string{}, validKeys...)
}

func isValidKey(key string) bool {
	for _, k := range validKeys {
		if k == key {
			return true
		}
	}
	return false
}

// Get returns the string representation of a config value by dot-notation key.
func (c *Config) Get(key string) (string, error) {
	if !isValidKey(key) {
		return "", fmt.Errorf("unknown configuration key '%s'", key)
	}

	switch key {
	case "default_jig":
		return c.EffectiveDefaultJig(), nil
	case "default_project":
		return c.DefaultProject, nil
	case "snapshots.enabled":
		return strconv.FormatBool(c.EffectiveSnapshotsEnabled()), nil
	case "snapshots.interval_enabled":
		return strconv.FormatBool(c.EffectiveIntervalEnabled()), nil
	case "snapshots.interval_seconds":
		return strconv.Itoa(c.EffectiveIntervalSeconds()), nil
	case "snapshots.max_snapshots":
		return strconv.Itoa(c.EffectiveMaxSnapshots()), nil
	case "sessions.stale_threshold_hours":
		return strconv.Itoa(c.EffectiveStaleThresholdHours()), nil
	case "finalize.repo_spec_path":
		return c.EffectiveRepoSpecPath(), nil
	default:
		return "", fmt.Errorf("unknown configuration key '%s'", key)
	}
}

// Set writes a value to a config field by dot-notation key, parsing the string.
func (c *Config) Set(key string, value string) error {
	if !isValidKey(key) {
		return fmt.Errorf("unknown configuration key '%s'", key)
	}

	switch key {
	case "default_jig":
		c.DefaultJig = value
	case "default_project":
		c.DefaultProject = value
	case "snapshots.enabled":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid value for 'snapshots.enabled': must be true or false")
		}
		c.Snapshots.Enabled = &b
	case "snapshots.interval_enabled":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid value for 'snapshots.interval_enabled': must be true or false")
		}
		c.Snapshots.IntervalEnabled = &b
	case "snapshots.interval_seconds":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for 'snapshots.interval_seconds': must be an integer")
		}
		c.Snapshots.IntervalSeconds = &n
	case "snapshots.max_snapshots":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for 'snapshots.max_snapshots': must be an integer")
		}
		c.Snapshots.MaxSnapshots = &n
	case "sessions.stale_threshold_hours":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for 'sessions.stale_threshold_hours': must be an integer")
		}
		c.Sessions.StaleThresholdHours = &n
	case "finalize.repo_spec_path":
		if !strings.Contains(value, "{codename}") {
			return fmt.Errorf("invalid value for 'finalize.repo_spec_path': must contain {codename}")
		}
		c.Finalize.RepoSpecPath = value
	default:
		return fmt.Errorf("unknown configuration key '%s'", key)
	}

	return nil
}
