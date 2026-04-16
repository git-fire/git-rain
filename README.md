# git-rain — Multi-Repo Sync CLI

<p align="center">
  <img src="assets/git-rain-lockup.svg#gh-light-mode-only" alt="git-rain: storm cloud, lightning bolt, falling DAG, commit node, wordmark" width="220" height="80">
  <img src="assets/git-rain-lockup-dark.svg#gh-dark-mode-only" alt="git-rain: storm cloud, lightning bolt, falling DAG, commit node, wordmark" width="220" height="80">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-1.0-brightgreen" alt="Status: 1.0">
  <img src="https://img.shields.io/badge/go-1.24.2-blue" alt="Go 1.24.2">
  <img src="https://img.shields.io/badge/license-MIT-blue" alt="License: MIT">
</p>

> The reverse of [`git-fire`](https://github.com/git-fire/git-fire).

```
git-fire  →  commit + push everything out
git-rain  →  fetch all remotes by default, or hydrate locals with --sync
```

`git-rain` discovers git repositories under your scan path (and known registry entries). **Default:** `git fetch --all` per repo (no `--prune` unless you opt in) so **remote-tracking refs** update without moving local branches. **Lighter fetch:** `--fetch-mainline`. **Local updates:** `--sync` with `--branch-mode` / config. **Destructive realignment:** `--risky` on the sync path only.

> **Warning: `--prune` is opt-in.** Passing `--prune` on `git fetch` deletes **stale remote-tracking branch refs** (for example `refs/remotes/origin/old-feature` after that branch was removed on the server). That is usually what you want for a tidy clone, but it **removes those ref names locally** until the next fetch brings them back if the branch reappears. Enable pruning only when you intend it: **`--prune`** / **`--no-prune`** for this run, **`global.fetch_prune`** in config, **`fetch_prune`** on a registry entry, or per-repo **`git config --local --bool rain.fetchprune true`**. Precedence for a given repo is: **CLI** → **repo `rain.fetchprune`** → **registry `fetch_prune`** → **global `fetch_prune`**.

Invocation note: `git-rain` and `git rain` are equivalent when `git-rain` is on your PATH.

## Table of Contents

- [Quick Start](#quick-start)
- [Install](#install)
  - [Homebrew (macOS/Linuxbrew)](#homebrew-macoslinuxbrew)
  - [WinGet (Windows)](#winget-windows)
  - [Linux native packages (.deb / .rpm)](#linux-native-packages-deb--rpm)
  - [Go install](#go-install)
  - [Binary archive (manual)](#binary-archive-manual)
  - [PATH setup (required)](#path-setup-required)
  - [Verify install](#verify-install)
  - [Build from source](#build-from-source)
- [How It Works](#how-it-works)
- [Key Features](#key-features)
- [Core Commands](#core-commands)
- [Flags](#flags)
- [Configuration](#configuration)
- [Interactive TUI](#interactive-tui)
- [Safe Mode vs Risky Mode](#safe-mode-vs-risky-mode)
- [Registry](#registry)
- [Security Notes](#security-notes)
- [Contributing](#contributing)
- [License](#license)

## Quick Start

```bash
# preview first — shows what would be synced without touching anything
git-rain --dry-run

# default: scan repos, then git fetch --all per repo (no --prune unless configured or --prune)
git-rain

# lighter: mainline remote-tracking refs only
git-rain --fetch-mainline

# full local branch sync (same safety/risky rules as before)
git-rain --sync

# interactive TUI: pick repos, then default fetch or --sync behavior
git-rain --rain
```

## Install

| Method | Command | Platform |
|---|---|---|
| Homebrew | `brew install git-fire/tap/git-rain` | macOS, Linuxbrew |
| WinGet | `winget install git-rain.git-rain` | Windows |
| Linux package | Download `.deb` or `.rpm` from [GitHub Releases](https://github.com/git-fire/git-rain/releases) | Linux |
| Go | `go install github.com/git-rain/git-rain@latest` | All (Go 1.24.2+) |
| Binary archive | [GitHub Releases](https://github.com/git-fire/git-rain/releases) | All |

### Homebrew (macOS/Linuxbrew)

```bash
brew tap git-fire/tap
brew install git-rain
```

### WinGet (Windows)

```powershell
winget install git-rain.git-rain
```

### Linux native packages (`.deb` / `.rpm`)

Download from [GitHub Releases](https://github.com/git-fire/git-rain/releases), then:

```bash
# Debian/Ubuntu
sudo dpkg -i ./git-rain_<version>_amd64.deb

# Fedora/RHEL/CentOS (dnf)
sudo dnf install ./git-rain_<version>_amd64.rpm
```

### Go install

```bash
go install github.com/git-rain/git-rain@latest
```

Or pin an explicit release:

```bash
go install github.com/git-rain/git-rain@v1.0.0
```

### Binary archive (manual)

Download and extract the right archive from [GitHub Releases](https://github.com/git-fire/git-rain/releases), then place the binary on your `PATH`.

### PATH setup (required)

**Go install (Linux/macOS):**
```bash
export PATH="$HOME/go/bin:$PATH"
```
Add that line to `~/.zshrc` or `~/.bashrc` to persist.

**Manual binary install (Linux/macOS):**
```bash
chmod +x git-rain
sudo mv git-rain /usr/local/bin/
```

**Manual binary install (Windows PowerShell):**
```powershell
New-Item -ItemType Directory -Force "$env:USERPROFILE\bin" | Out-Null
Move-Item .\git-rain.exe "$env:USERPROFILE\bin\git-rain.exe" -Force
```
Then add `$env:USERPROFILE\bin` to your user `PATH` if not already present.

### Verify install

```bash
git-rain --version
which git-rain
```

### Build from source

```bash
git clone https://github.com/git-fire/git-rain.git
cd git-rain
make build         # produces ./git-rain
make install       # installs to ~/.local/bin/git-rain
```

Requires Go 1.24.2+.

## How It Works

1. **Scan** — walks your configured scan path and includes known registry repos (`--no-scan` limits to registry only)

2. **Default: fetch all remotes** — for each repo, `git fetch --all` plus optional `--prune` (see warning above), and optional `--tags` from config or `--tags`. Updates **remote-tracking refs** under `refs/remotes/` only; **local branch refs are not created or moved** (fast path: checkout `origin/<branch>` when you need a branch).

3. **`--fetch-mainline`** — targeted `git fetch <remote>` for mainline branches (and `mainline_patterns`) only, with the same opt-in `--prune` rules as the default fetch.

4. **`--sync`** — hydrates **local** branches: `git fetch --all` (same prune/tags rules), then updates eligible locals toward upstream. Scope comes from **`--branch-mode`** or config `branch_mode`: `mainline`, `checked-out`, `all-local`, or **`all-branches`** (creates local tracking branches for remotes you do not have yet — can be many branches).

5. **`--risky`** — does not change fetch behavior by itself; on the **`--sync`** path it allows hard reset to upstream after creating `git-rain-backup-*` refs when you would otherwise skip local-only commits.

6. **Report** — one summary line per repo on the default full fetch; per-branch lines on `--fetch-mainline` and `--sync`.

## Key Features

- **One-command workflow** — default full remote fetch, optional `--fetch-mainline`, then `--sync` (+ `branch_mode`) for locals, `--risky` when you accept destructive realignment
- **Safety-first defaults** — never rewrites local-only commits; dirty worktrees are skipped, not clobbered
- **Risky mode** — opt-in destructive realignment: creates a `git-rain-backup-*` ref, then hard-resets to upstream
- **Non-checked-out branches** — updated directly without touching the worktree
- **Interactive TUI (`--rain`)** — streaming repo picker (mirrors `git-fire --fire`), then the same default fetch, `--fetch-mainline`, or `--sync` behavior
- **Registry** — discovered repos persist across runs; mark repos ignored to skip them permanently
- **Dry run** — preview all repos that would be fetched or synced without making any changes
- **`--fetch-mainline`** — mainline-only remote-tracking ref refresh instead of the default full `git fetch --all`

## Core Commands

```bash
# dry run — preview repos, no changes
git-rain --dry-run

# default run — scan repos, git fetch --all per repo
git-rain

# this run: prune stale remote-tracking refs for every repo
git-rain --prune

# mainline-only remote-tracking ref refresh (lighter than default)
git-rain --fetch-mainline

# full local branch sync after scan
git-rain --sync

# interactive TUI before default fetch or sync
git-rain --rain

# sync only known registry repos, skip filesystem scan
git-rain --no-scan

# scan a specific path
git-rain --path ~/projects

# risky full sync — realign local-only commits after creating backup branches
git-rain --sync --risky

# generate example config file
git-rain --init
```

## Flags

| Flag | Description |
|---|---|
| `--dry-run` | Show what would run without making changes |
| `--rain` | Interactive TUI repo picker before running (like `git-fire --fire`) |
| `--sync` | Update local branches from remotes (after `git fetch --all`; default run does not sync locals) |
| `--fetch-mainline` | Mainline-only remote `git fetch` per remote instead of default `git fetch --all` |
| `--branch-mode` | With `--sync`: `mainline`, `checked-out`, `all-local`, or `all-branches` (overrides config for this run) |
| `--prune` | Pass `--prune` on fetch for this run (highest precedence; cannot combine with `--no-prune`) |
| `--no-prune` | Never pass `--prune` on fetch for this run (overrides `--prune`, config, registry, and `rain.fetchprune`) |
| `--tags` | Also pass `--tags` on fetch operations |
| `--path <dir>` | Scan path override (default: config `global.scan_path`) |
| `--no-scan` | Skip filesystem scan; hydrate only known registry repos |
| `--risky` | Allow destructive local branch realignment after creating backup refs |
| `--init` | Generate example `~/.config/git-rain/config.toml` |
| `--config <file>` | Use an explicit config file path |
| `--force-unlock-registry` | Remove stale registry lock file without prompting |
| `--version` | Print version and exit |

## Configuration

Config file: `~/.config/git-rain/config.toml`

Generate an example:

```bash
git-rain --init
```

Key options:

```toml
[global]
scan_path    = "/home/you/projects"   # root to discover repos under
scan_depth   = 5                      # max directory depth
scan_workers = 8                      # parallel scan workers
risky_mode   = false                  # destructive realignment on --sync path only
branch_mode  = "mainline"             # used with --sync: mainline | checked-out | all-local | all-branches
fetch_prune  = false                  # pass --prune on fetch when true (default off; see README warning)
default_mode = "safe"                 # "safe" or "risky"
disable_scan = false                  # skip scan; use registry only

scan_exclude = [
  "node_modules",
  ".cache",
  "vendor",
]
```

All options can be overridden with environment variables using the `GIT_RAIN_` prefix:

```bash
GIT_RAIN_GLOBAL_RISKY_MODE=true git-rain
GIT_RAIN_GLOBAL_SCAN_PATH=/tmp/repos git-rain
```

## Interactive TUI

`git-rain --rain` opens an interactive picker. Repositories stream in as the filesystem scan finds them — no waiting for the full scan to complete before you can start picking. After you confirm, the tool runs the **default full fetch** (`git fetch --all`, prune opt-in) unless you passed **`--fetch-mainline`**, or **`--sync`** / config implies full branch hydration.

**Key bindings:**

| Key | Action |
|---|---|
| `space` | Toggle repo selection |
| `a` | Select all / deselect all |
| `enter` | Confirm selection and begin fetch or sync |
| `q` / `esc` | Abort |
| `↑` / `↓` | Navigate |

## Safe Mode vs Risky Mode

| Situation | Safe mode (default) | Risky mode (`--risky`) |
|---|---|---|
| Branch is fast-forwardable | ✓ Updated | ✓ Updated |
| Branch has local-only commits | ⊘ Skipped | ⚠ Backed up + reset |
| Checked-out branch, dirty worktree | ⊘ Skipped | ⊘ Skipped |
| No upstream tracked | ⊘ Skipped | ⊘ Skipped |

In risky mode, a `git-rain-backup-<branch>-<timestamp>` ref is created before any hard reset so local work is always recoverable.

## Registry

Discovered repositories are stored in `~/.config/git-rain/repos.toml`. Each entry tracks path, name, status, and last-seen time. Optional per-repo **`fetch_prune`** (boolean) opts that repository into `--prune` on fetch when set; it is overridden by that repo’s local **`git config rain.fetchprune`**, and both are overridden by **`--prune`** / **`--no-prune`** on the CLI for that run.

Repo statuses:
- `active` — present on disk and eligible for sync
- `missing` — was discovered previously but the directory is gone
- `ignored` — permanently excluded from sync

The registry uses a file lock to prevent concurrent `git-rain` instances from corrupting it. If a previous run exited uncleanly, `git-rain` detects the stale lock and prompts to remove it (or pass `--force-unlock-registry` for non-interactive use).

## Security Notes

`git-rain` shells out to the system `git` binary and inherits your existing git credentials (SSH agent, credential helper, etc.). No credentials are stored or transmitted by `git-rain` itself.

Secret detection: `git-rain` sanitizes error messages and log output to avoid echoing paths or git output that might contain tokens. This is a best-effort measure — keep secrets out of repo paths and remote URLs.

## Contributing

Contributions are welcome. Tests use [git-testkit](https://github.com/git-fire/git-testkit) for building git repository fixtures in integration-style tests. Prefer table-driven tests and real `git` invocations over mocks.

```bash
make test-race   # run all tests with race detector
make lint        # go vet
```

Open an issue before starting large changes.

## License

MIT. See [LICENSE](LICENSE).
