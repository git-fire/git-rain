// Package config defines the git-rain configuration schema and related constants.
package config

import (
	"math"
	"strings"
)

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
	// "matrix" (falling code glyphs in the same column pattern), "garden"
	// (seeds, rain, growth, then sun), or "snow" (winter scene).
	RainAnimationMode string `mapstructure:"rain_animation_mode" toml:"rain_animation_mode"`

	// RainPanelSize is how tall the animation canvas is in the TUI: "compact",
	// "comfortable", or "tall". The runtime clamps to the terminal so the panel
	// still fits (see RainPanelRows).
	RainPanelSize string `mapstructure:"rain_panel_size" toml:"rain_panel_size"`

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

	// Garden mode tuning (used when rain_animation_mode = "garden").
	// In the settings TUI, three rows appear under Rain animation mode; any
	// field left at zero here still falls back to built-in defaults at runtime.

	// GardenSeedRate is the target probability (0..1) for sky seeds; the
	// simulator caps seeds per frame, so this reads as density, not i.i.d.
	// rolls. Lower = rarer; default ~0.055.
	GardenSeedRate float64 `mapstructure:"garden_seed_rate"        toml:"garden_seed_rate,omitempty"`

	// GardenGrowthPace multiplies the moisture thresholds for every growth
	// stage. >1 slows growth; <1 speeds it up; 0 = use defaults.
	GardenGrowthPace float64 `mapstructure:"garden_growth_pace"      toml:"garden_growth_pace,omitempty"`

	// GardenBloomDurationBase is the minimum bloom lifetime in animation
	// frames before a flower starts to wither. 0 = default.
	GardenBloomDurationBase int `mapstructure:"garden_bloom_duration_base"   toml:"garden_bloom_duration_base,omitempty"`

	// GardenBloomDurationJitter is added on top of the base bloom duration
	// (uniform random in [0, jitter)). 0 = default.
	GardenBloomDurationJitter int `mapstructure:"garden_bloom_duration_jitter" toml:"garden_bloom_duration_jitter,omitempty"`

	// GardenWitherDuration is how many frames a withered plant lingers
	// before crumbling and seeding offspring. 0 = default.
	GardenWitherDuration int `mapstructure:"garden_wither_duration"  toml:"garden_wither_duration,omitempty"`

	// GardenOffspringMin/Max bound how many seeds a dying plant scatters.
	// 0 = use defaults (currently 1 and 2).
	GardenOffspringMin int `mapstructure:"garden_offspring_min"    toml:"garden_offspring_min,omitempty"`
	GardenOffspringMax int `mapstructure:"garden_offspring_max"    toml:"garden_offspring_max,omitempty"`

	// GardenOffspringSpread is the half-width X jitter applied around the
	// parent column when scattering offspring seeds. 0 = default.
	GardenOffspringSpread int `mapstructure:"garden_offspring_spread" toml:"garden_offspring_spread,omitempty"`

	// SnowAccumulationRate scales how much ground snow depth each landed flake
	// adds when rain_animation_mode = "snow". 1 = default; 2 ≈ twice as fast.
	// Values are rounded to a whole number of depth units per landing (1..8).
	SnowAccumulationRate float64 `mapstructure:"snow_accumulation_rate" toml:"snow_accumulation_rate,omitempty"`
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
	UIRainAnimationSnow     = "snow"

	UIRainPanelCompact     = "compact"
	UIRainPanelComfortable = "comfortable"
	UIRainPanelTall        = "tall"
)

// RainPanelRows returns the target animation height in terminal rows for a
// panel size preset. Unknown or empty values use comfortable.
func RainPanelRows(preset string) int {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case UIRainPanelCompact:
		return 5
	case UIRainPanelTall:
		return 11
	default:
		return 8
	}
}

// NormalizeRainPanelSize returns a canonical preset name.
func NormalizeRainPanelSize(preset string) string {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case UIRainPanelCompact:
		return UIRainPanelCompact
	case UIRainPanelTall:
		return UIRainPanelTall
	default:
		return UIRainPanelComfortable
	}
}

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

// SnowAccumPerLanding returns ground depth units added per landed snowflake.
// rate is cfg.UI.SnowAccumulationRate; 0 or negative means 1. Result is in [1, 8].
func SnowAccumPerLanding(rate float64) int {
	if rate <= 0 {
		return 1
	}
	n := int(math.Round(rate))
	if n < 1 {
		return 1
	}
	if n > 8 {
		return 8
	}
	return n
}
