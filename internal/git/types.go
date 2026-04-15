// Package git discovers repositories and runs git operations by shelling out to the git binary.
package git

import (
	"context"
	"time"
)

// Repository represents a discovered git repository
type Repository struct {
	Path         string    // Full filesystem path
	Name         string    // Repo name (from directory)
	Remotes      []Remote  // Configured remotes
	Branches     []string  // Local branch names
	IsDirty      bool      // Has uncommitted changes
	LastModified time.Time // Last commit time
	Selected     bool      // User selected for fetch
	Mode         RepoMode  // Mode for this repo

	// IsNewRegistryEntry is set by the registry upsert step.
	IsNewRegistryEntry bool
}

// Remote represents a git remote
type Remote struct {
	Name string // "origin", "upstream", etc.
	URL  string // Remote URL
}

// RepoMode defines the per-repo registry sync disposition.
type RepoMode int

const (
	ModeLeaveUntouched  RepoMode = iota // Skip this repo entirely
	ModeSyncDefault                     // Sync using global defaults
	ModeSyncAll                         // Sync all branches for this repo
	ModeSyncCurrentBranch               // Sync only the checked-out branch
)

func (m RepoMode) String() string {
	switch m {
	case ModeLeaveUntouched:
		return "leave-untouched"
	case ModeSyncDefault:
		return "sync-default"
	case ModeSyncAll:
		return "sync-all"
	case ModeSyncCurrentBranch:
		return "sync-current-branch"
	default:
		return "unknown"
	}
}

// ParseMode converts a mode string back to a RepoMode constant.
func ParseMode(s string) RepoMode {
	switch s {
	case "leave-untouched":
		return ModeLeaveUntouched
	case "sync-all":
		return ModeSyncAll
	case "sync-current-branch":
		return ModeSyncCurrentBranch
	default: // "", "sync-default", or unrecognised
		return ModeSyncDefault
	}
}

// BranchSyncMode controls which branches RainRepository will operate on.
type BranchSyncMode string

const (
	// BranchSyncMainline syncs main/master/trunk/develop/dev and gitflow
	// patterns (release/*, hotfix/*, support/*) that exist locally.
	// This is the default — low risk, no branch sprawl.
	BranchSyncMainline BranchSyncMode = "mainline"

	// BranchSyncCheckedOut syncs only branches currently checked out in
	// any worktree. Low risk; minimal surface area.
	BranchSyncCheckedOut BranchSyncMode = "checked-out"

	// BranchSyncAllLocal syncs every local branch that has a tracked
	// upstream ref.
	BranchSyncAllLocal BranchSyncMode = "all-local"

	// BranchSyncAllBranches creates local tracking refs for every branch
	// that exists on the remote but not locally, then syncs all of them.
	// Can produce many local branches — use with care.
	BranchSyncAllBranches BranchSyncMode = "all-branches"
)

// ParseBranchSyncMode converts a string to a BranchSyncMode.
// Unrecognised values fall back to BranchSyncMainline.
func ParseBranchSyncMode(s string) BranchSyncMode {
	switch BranchSyncMode(s) {
	case BranchSyncCheckedOut, BranchSyncAllLocal, BranchSyncAllBranches:
		return BranchSyncMode(s)
	default:
		return BranchSyncMainline
	}
}

// ScanOptions configures repository scanning
type ScanOptions struct {
	// Root path to start scanning from
	RootPath string

	// Exclude patterns (directories to skip)
	Exclude []string

	// Max directory depth
	MaxDepth int

	// Use cached results if available
	UseCache bool

	// Cache file path
	CacheFile string

	// Cache TTL
	CacheTTL time.Duration

	// Parallel workers
	Workers int

	// KnownPaths maps absolute repo paths already in the registry to their
	// rescan_submodules flag.
	KnownPaths map[string]bool

	// Ctx controls cancellation of the scan.
	Ctx context.Context

	// FolderProgress, when non-nil, receives the path of each directory visited
	// during the filesystem walk.
	FolderProgress chan<- string

	// DisableScan skips the filesystem walk entirely.
	DisableScan bool
}

// DefaultScanOptions returns sensible defaults
func DefaultScanOptions() ScanOptions {
	return ScanOptions{
		RootPath: ".",
		Exclude: []string{
			".cache",
			"node_modules",
			".venv",
			"venv",
			"vendor",
			"dist",
			"build",
			"target",
		},
		MaxDepth:  10,
		UseCache:  true,
		CacheFile: "",
		Workers:   8,
	}
}
