package config

const (
	DefaultFetchWorkers              = 4
	DefaultUIRainTickMS              = 150
	DefaultUIStartupQuoteIntervalSec = 10
)

// DefaultConfig returns safe default configuration
func DefaultConfig() Config {
	return Config{
		Global: GlobalConfig{
			ScanPath: ".",
			ScanExclude: []string{
				".cache",
				"node_modules",
				".venv",
				"venv",
				"vendor",
				"dist",
				"build",
				"target",
			},
			ScanDepth:        10,
			ScanWorkers:      8,
			FetchWorkers:     DefaultFetchWorkers,
			DefaultMode:      "push-known-branches",
			RescanSubmodules: false,
			DisableScan:      false,
			RiskyMode:        false,
		},
		UI: UIConfig{
			ShowRainAnimation:       true,
			RainAnimationMode:       UIRainAnimationBasic,
			ShowStartupQuote:        true,
			StartupQuoteBehavior:    UIQuoteBehaviorRefresh,
			StartupQuoteIntervalSec: DefaultUIStartupQuoteIntervalSec,
			RainTickMS:              DefaultUIRainTickMS,
			ColorProfile:            UIColorProfileStorm,
		},
	}
}

// ExampleConfigTOML returns an example configuration file
func ExampleConfigTOML() string {
	return `# Git Rain Configuration
# Place this file at ~/.config/git-rain/config.toml

[global]
# Directory to scan for git repos
scan_path = "."

# Directories to exclude from scanning
scan_exclude = [
    ".cache",
    "node_modules",
    ".venv",
    "venv",
    "vendor",
    "dist",
    "build",
    "target"
]

# Maximum directory depth to scan
scan_depth = 10

# Number of parallel workers for scanning
scan_workers = 8

# Number of parallel workers for fetching/updating repositories
fetch_workers = 4

# Default mode for repos (used by registry opt-out model)
default_mode = "push-known-branches"

# Re-scan known repos for new submodules
rescan_submodules = false

# Skip filesystem walk; only hydrate repos already in the registry.
# Use --no-scan flag to override for a single run without changing this file.
disable_scan = false

# Allow destructive local branch realignment.
# false = preserve local-only commits; true = permit hard resets to remote.
risky_mode = false

[ui]
# Show rain animation in the interactive repo selector (toggle live with 'r')
show_rain_animation = true

# Animation mode: "basic" (rain drops only) or "advanced" (clouds + rain + flowers)
rain_animation_mode = "basic"

# Show flavor quotes in the TUI banner
show_startup_quote = true

# What to do when the quote timer expires: "refresh" or "hide"
startup_quote_behavior = "refresh"

# Seconds between quote refreshes (or before hiding)
startup_quote_interval_sec = 10

# Rain animation speed in milliseconds per frame (lower = faster)
rain_tick_ms = 150

# Color profile: "storm", "drizzle", "monsoon", "rainbow", "synthwave"
color_profile = "storm"
`
}
