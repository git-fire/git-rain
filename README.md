# git-rain — Multi-Repo Sync CLI

<p align="center">
  <img src="https://img.shields.io/badge/status-1.0-brightgreen" alt="Status: 1.0">
  <img src="https://img.shields.io/badge/go-1.24.2-blue" alt="Go 1.24.2">
  <img src="https://img.shields.io/badge/license-MIT-blue" alt="License: MIT">
</p>

> The reverse of [`git-fire`](https://github.com/git-fire/git-fire).

```
git-fire  →  commit + push everything out
git-rain  →  pull everything back down
```

`git-rain` discovers all your local git repositories and syncs them from their remotes in one command — fast-forwarding branches, updating non-checked-out refs, and skipping anything that would rewrite local-only commits (unless you ask it to).

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
- [TUI Selector](#tui-selector)
- [Safe Mode vs Risky Mode](#safe-mode-vs-risky-mode)
- [Registry](#registry)
- [Security Notes](#security-notes)
- [Contributing](#contributing)
- [License](#license)

## Quick Start

```bash
# preview first — shows what would be synced without touching anything
git-rain --dry-run

# sync all repos under the configured scan path
git-rain

# interactive TUI: pick exactly which repos to sync
git-rain --select
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

1. **Scan** — walks your configured scan path discovering git repositories
2. **Fetch** — runs `git fetch --all --prune` for each repo
3. **Sync** — for each local branch with a tracked upstream:
   - If the branch can be fast-forwarded: updates it
   - If the branch has local-only commits: skips (safe mode) or backs up and resets (risky mode)
   - If the working tree is dirty on the checked-out branch: skips
4. **Report** — per-branch outcomes: updated, up-to-date, skipped (with reason), failed

## Key Features

- **One-command sync** — discover and update all repos from a single invocation
- **Safety-first defaults** — never rewrites local-only commits; dirty worktrees are skipped, not clobbered
- **Risky mode** — opt-in destructive realignment: creates a `git-rain-backup-*` ref, then hard-resets to upstream
- **Non-checked-out branches** — updated directly without touching the worktree
- **Interactive TUI** — streaming repo selector lets you pick exactly which repos to sync
- **Registry** — discovered repos persist across runs; mark repos ignored to skip them permanently
- **Dry run** — preview all repos that would be synced without making any changes
- **Fetch-only mode** — run `git fetch --all --prune` everywhere without touching local refs

## Core Commands

```bash
# dry run — preview repos, no changes
git-rain --dry-run

# default run — scan and sync all repos
git-rain

# interactive repo selection before syncing
git-rain --select

# fetch only (no local ref updates)
git-rain --fetch-only

# sync only known registry repos, skip filesystem scan
git-rain --no-scan

# scan a specific path
git-rain --path ~/projects

# risky mode — realign local-only commits after creating backup branches
git-rain --risky

# generate example config file
git-rain --init
```

## Flags

| Flag | Description |
|---|---|
| `--dry-run` | Show what would be synced without making changes |
| `--select` | Interactive TUI repo selector before syncing |
| `--fetch-only` | Fetch from all remotes but skip local ref updates |
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
risky_mode   = false                  # enable risky mode globally
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

## TUI Selector

`git-rain --select` opens an interactive selector. Repositories stream in as the filesystem scan finds them — no waiting for the full scan to complete before you can start picking.

**Key bindings:**

| Key | Action |
|---|---|
| `space` | Toggle repo selection |
| `a` | Select all / deselect all |
| `enter` | Confirm selection and begin sync |
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

Discovered repositories are stored in `~/.config/git-rain/repos.toml`. Each entry tracks path, name, status, and last-seen time.

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
