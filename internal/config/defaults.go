package config

const DefaultFetchWorkers = 4

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
`
}
