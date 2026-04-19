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
			DefaultMode:      "sync-default",
			RescanSubmodules: false,
			DisableScan:      false,
			RiskyMode:        false,
			BranchMode:       "mainline",
			SyncTags:         false,
			FetchPrune:       false,
			MainlinePatterns: []string{},
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
default_mode = "sync-default"

# Re-scan known repos for new submodules
rescan_submodules = false

# Skip filesystem walk; only hydrate repos already in the registry.
# Use --no-scan flag to override for a single run without changing this file.
disable_scan = false

# Allow destructive local branch realignment.
# false = preserve local-only commits; true = permit hard resets to remote.
risky_mode = false

# Which branches to sync per run.
# "mainline"     - main/master/trunk/develop/dev + gitflow patterns (default)
# "checked-out"  - only branches currently checked out in any worktree
# "all-local"    - every local branch with a tracked upstream
# "all-branches" - all remote branches; creates local tracking refs if missing (many branches!)
branch_mode = "mainline"

# Fetch all tags from remotes. Off by default — can pull large or unwanted history.
sync_tags = false

# Pass --prune on git fetch (removes stale remote-tracking refs). Off by default.
# Enable per-run with --prune, per-repo with git config rain.fetchprune, or registry fetch_prune.
fetch_prune = false

# Additional branch name patterns treated as mainline when branch_mode = "mainline".
# Exact names and "/" prefixes are both supported.
# Examples: "develop", "feat/", "JIRA-"
mainline_patterns = []

[ui]
# Show rain animation in the interactive repo selector (toggle live with 'r')
show_rain_animation = true

# Animation mode: "basic" (rain drops), "advanced" (clouds + rain + flowers),
# "matrix" (falling code characters), or "garden" (seeds bloom into a meadow,
# then the rain stops and the sun comes out)
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

# --- Garden mode tuning (advanced) -----------------------------------------
# These keys only affect rain_animation_mode = "garden". Leave them unset
# (or at 0) to use the built-in defaults; tweak to make growth slower or
# offspring more (or less) prolific.
#
# garden_seed_rate            = 0.055  # sky seed density (0..1); runtime caps bursts per frame
# garden_growth_pace          = 1.0    # multiplier on stage moisture thresholds (>1 = slower)
# garden_bloom_duration_base  = 60     # min frames a flower lingers in full bloom
# garden_bloom_duration_jitter = 40    # extra random frames added to bloom lifetime
# garden_wither_duration      = 28     # frames a withered plant lingers before re-seeding
# garden_offspring_min        = 1      # minimum seeds a dying plant scatters
# garden_offspring_max        = 2      # maximum seeds a dying plant scatters
# garden_offspring_spread     = 2      # X-jitter half-width around the parent column
`
}
