// Package config defines the git-rain configuration schema and related constants.
package config

// Config represents the complete git-rain configuration
type Config struct {
	Global GlobalConfig `mapstructure:"global" toml:"global"`
	UI     UIConfig     `mapstructure:"ui"     toml:"ui"`
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

	// BranchMode controls which branches are synced per run.
	// Options: "mainline" (default), "checked-out", "all-local", "all-branches".
	BranchMode string `mapstructure:"branch_mode" toml:"branch_mode"`

	// SyncTags fetches all tags from remotes in addition to branch refs.
	// Off by default — tag fetches can pull large or unwanted histories.
	SyncTags bool `mapstructure:"sync_tags" toml:"sync_tags"`

	// FetchPrune passes --prune on git fetch (removes stale remote-tracking refs).
	// Off by default — pruning is destructive to those refs. CLI --prune and per-repo
	// git config rain.fetchprune / registry fetch_prune can still enable it.
	FetchPrune bool `mapstructure:"fetch_prune" toml:"fetch_prune"`

	// MainlinePatterns are additional branch name patterns treated as mainline
	// when branch_mode = "mainline". Supports exact names and prefix globs
	// ending in "/" (e.g. "feat/", "JIRA-").
	MainlinePatterns []string `mapstructure:"mainline_patterns" toml:"mainline_patterns"`
}

// UIConfig contains TUI/display settings
type UIConfig struct {
	// Show the rain animation in the repo selector. Toggle live with 'r'.
	// Automatically suppressed when the terminal is too short.
	ShowRainAnimation bool `mapstructure:"show_rain_animation" toml:"show_rain_animation"`

	// Animation mode: "basic" (rain drops), "advanced" (clouds + rain + flowers),
	// "matrix" (falling code glyphs), or "garden" (seeds, rain, growth, then sun).
	RainAnimationMode string `mapstructure:"rain_animation_mode" toml:"rain_animation_mode"`

	// Show flavor quotes: TUI banner plus CLI motivation lines.
	ShowStartupQuote bool `mapstructure:"show_startup_quote" toml:"show_startup_quote"`

	// Flavor quote behavior after interval elapses (TUI). Options: "refresh", "hide".
	StartupQuoteBehavior string `mapstructure:"startup_quote_behavior" toml:"startup_quote_behavior"`

	// Interval in seconds for flavor quote behavior in the TUI.
	StartupQuoteIntervalSec int `mapstructure:"startup_quote_interval_sec" toml:"startup_quote_interval_sec"`

	// Rain animation tick interval in milliseconds.
	// Lower values animate faster but can increase terminal CPU usage.
	RainTickMS int `mapstructure:"rain_tick_ms" toml:"rain_tick_ms"`

	// Color profile for rain and TUI accents.
	// Options: "storm", "drizzle", "monsoon", "rainbow", "synthwave".
	ColorProfile string `mapstructure:"color_profile" toml:"color_profile"`
}

const (
	UIColorProfileStorm     = "storm"
	UIColorProfileDrizzle   = "drizzle"
	UIColorProfileMonsoon   = "monsoon"
	UIColorProfileRainbow   = "rainbow"
	UIColorProfileSynthwave = "synthwave"

	UIQuoteBehaviorRefresh = "refresh"
	UIQuoteBehaviorHide    = "hide"

	UIRainAnimationBasic    = "basic"
	UIRainAnimationAdvanced = "advanced"
	UIRainAnimationMatrix   = "matrix"
	UIRainAnimationGarden   = "garden"
)

// UIColorProfiles returns valid built-in UI color profile names.
func UIColorProfiles() []string {
	return []string{
		UIColorProfileStorm,
		UIColorProfileDrizzle,
		UIColorProfileMonsoon,
		UIColorProfileRainbow,
		UIColorProfileSynthwave,
	}
}
