// Package config defines the git-rain configuration schema and related constants.
package config

// Config represents the complete git-rain configuration
type Config struct {
	Global GlobalConfig `mapstructure:"global" toml:"global"`
}

// GlobalConfig contains global settings
type GlobalConfig struct {
	// Scan configuration
	ScanPath    string   `mapstructure:"scan_path"    toml:"scan_path"`
	ScanExclude []string `mapstructure:"scan_exclude" toml:"scan_exclude"`
	ScanDepth   int      `mapstructure:"scan_depth"   toml:"scan_depth"`
	ScanWorkers int      `mapstructure:"scan_workers" toml:"scan_workers"`

	// FetchWorkers controls parallel per-repo rain operations.
	FetchWorkers int `mapstructure:"fetch_workers" toml:"fetch_workers"`

	// Default mode for repos (used for registry opt-out model).
	DefaultMode string `mapstructure:"default_mode" toml:"default_mode"`

	// Re-scan known repos for new submodules.
	RescanSubmodules bool `mapstructure:"rescan_submodules" toml:"rescan_submodules"`

	// Skip filesystem walk; only hydrate repos already in the registry.
	DisableScan bool `mapstructure:"disable_scan" toml:"disable_scan"`

	// Allow destructive local branch realignment during rain.
	// When false, local-only commits are never rewritten.
	RiskyMode bool `mapstructure:"risky_mode" toml:"risky_mode"`
}
