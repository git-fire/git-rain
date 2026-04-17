package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
)

// LoadOptions configures LoadWithOptions (e.g. explicit config file path).
type LoadOptions struct {
	ConfigFile string
}

// Load loads configuration from files and environment variables.
// Priority (highest to lowest):
//  1. Environment variables (GIT_RAIN_*)
//  2. Explicit --config file (optional)
//  3. user config dir/git-rain/config.toml (user config)
//  4. Default config
func Load() (*Config, error) {
	return LoadWithOptions(LoadOptions{})
}

// LoadWithOptions loads config with optional explicit config file override.
func LoadWithOptions(opts LoadOptions) (*Config, error) {
	v := viper.New()

	setDefaults(v)

	v.SetConfigName("config")
	v.SetConfigType("toml")

	userCfgDir, cfgWarning := resolvedUserConfigDir()
	v.AddConfigPath(userCfgDir)
	if cfgWarning != "" {
		fmt.Fprintf(os.Stderr, "warning: %s\n", cfgWarning)
	}
	v.AddConfigPath("/etc/git-rain")
	if opts.ConfigFile != "" {
		v.SetConfigFile(opts.ConfigFile)
	}

	v.SetEnvPrefix("GIT_RAIN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		if opts.ConfigFile != "" {
			return nil, fmt.Errorf("config file not found: %s", opts.ConfigFile)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values in Viper
func setDefaults(v *viper.Viper) {
	defaults := DefaultConfig()

	v.SetDefault("global.scan_path", defaults.Global.ScanPath)
	v.SetDefault("global.scan_exclude", defaults.Global.ScanExclude)
	v.SetDefault("global.scan_depth", defaults.Global.ScanDepth)
	v.SetDefault("global.scan_workers", defaults.Global.ScanWorkers)
	v.SetDefault("global.fetch_workers", defaults.Global.FetchWorkers)
	v.SetDefault("global.default_mode", defaults.Global.DefaultMode)
	v.SetDefault("global.rescan_submodules", defaults.Global.RescanSubmodules)
	v.SetDefault("global.disable_scan", defaults.Global.DisableScan)
	v.SetDefault("global.risky_mode", defaults.Global.RiskyMode)
	v.SetDefault("global.branch_mode", defaults.Global.BranchMode)
	v.SetDefault("global.sync_tags", defaults.Global.SyncTags)
	v.SetDefault("global.fetch_prune", defaults.Global.FetchPrune)
	v.SetDefault("global.mainline_patterns", defaults.Global.MainlinePatterns)

	v.SetDefault("ui.show_rain_animation", defaults.UI.ShowRainAnimation)
	v.SetDefault("ui.rain_animation_mode", defaults.UI.RainAnimationMode)
	v.SetDefault("ui.show_startup_quote", defaults.UI.ShowStartupQuote)
	v.SetDefault("ui.startup_quote_behavior", defaults.UI.StartupQuoteBehavior)
	v.SetDefault("ui.startup_quote_interval_sec", defaults.UI.StartupQuoteIntervalSec)
	v.SetDefault("ui.rain_tick_ms", defaults.UI.RainTickMS)
	v.SetDefault("ui.color_profile", defaults.UI.ColorProfile)
}

// SaveConfig writes cfg to path as TOML. Intermediate directories are created
// if needed. Uses an exclusive lock (path + ".lock") and a PID-scoped temp file
// so concurrent writers or interrupted renames cannot corrupt the live config.
func SaveConfig(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	lock := flock.New(path + ".lock")
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("config file lock: %w", err)
	}
	defer func() { _ = lock.Unlock() }()

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	tmp := fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing temp config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replacing config file: %w", err)
	}
	return nil
}

// LoadOrDefault loads config or returns defaults if no config found.
func LoadOrDefault() *Config {
	cfg, err := Load()
	if err != nil {
		defaultCfg := DefaultConfig()
		return &defaultCfg
	}
	return cfg
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	dm := strings.TrimSpace(c.Global.DefaultMode)
	if dm == "" {
		dm = DefaultConfig().Global.DefaultMode
	}
	switch dm {
	case "leave-untouched", "sync-default", "sync-all", "sync-current-branch":
		c.Global.DefaultMode = dm
	default:
		return fmt.Errorf("global.default_mode must be one of leave-untouched, sync-default, sync-all, sync-current-branch, got %q", c.Global.DefaultMode)
	}
	if c.Global.FetchWorkers <= 0 {
		c.Global.FetchWorkers = DefaultFetchWorkers
	}
	return nil
}

// DefaultConfigPath returns the default user config file path.
func DefaultConfigPath() string {
	userCfgDir, cfgWarning := resolvedUserConfigDir()
	if cfgWarning != "" {
		fmt.Fprintf(os.Stderr, "warning: %s\n", cfgWarning)
	}
	return filepath.Join(userCfgDir, "config.toml")
}

// UserGitRainDir returns the per-user git-rain application directory (the parent
// of config.toml and repos.toml). The warning string is non-empty when a
// non-primary fallback from resolvedUserConfigDir is in use.
func UserGitRainDir() (dir string, warning string) {
	return resolvedUserConfigDir()
}

func userConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not determine user config directory: %w", err)
	}
	return filepath.Join(base, "git-rain"), nil
}

func fallbackUserConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory for fallback: %w", err)
	}
	if !filepath.IsAbs(home) {
		abs, absErr := filepath.Abs(home)
		if absErr != nil {
			return "", fmt.Errorf("fallback home directory is not absolute (%q): %w", home, absErr)
		}
		home = abs
	}
	return filepath.Join(home, ".config", "git-rain"), nil
}

func resolvedUserConfigDir() (string, string) {
	if dir, err := userConfigDir(); err == nil {
		return dir, ""
	}
	if dir, err := fallbackUserConfigDir(); err == nil {
		return dir, fmt.Sprintf("using fallback user config directory %q", dir)
	}
	tempBase := os.TempDir()
	if !filepath.IsAbs(tempBase) {
		if abs, absErr := filepath.Abs(tempBase); absErr == nil {
			tempBase = abs
		}
	}
	dir := filepath.Join(tempBase, "git-rain")
	return dir, fmt.Sprintf("using temporary config fallback %q; this path may not persist across reboots", dir)
}

// WriteExampleConfig writes an example config file to the specified path.
// Same locking and atomic replace semantics as SaveConfig.
func WriteExampleConfig(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	lock := flock.New(path + ".lock")
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("config file lock: %w", err)
	}
	defer func() { _ = lock.Unlock() }()

	content := ExampleConfigTOML()
	tmp := fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to replace config file: %w", err)
	}
	return nil
}
