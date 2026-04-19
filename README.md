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

`git-rain` discovers git repositories under your scan path (and known registry entries). From lightest to heaviest:

| Mode | What it does |
| --- | --- |
| **Default** | `git fetch --all` per repo. `--prune` is opt-in (see below). Updates **remote-tracking refs**; does not move local branches. |
| **Lighter fetch** | `--fetch-mainline` — mainline remote-tracking refs only. |
| **Local updates** | `--sync` — hydrate locals; scope from `--branch-mode` or config. |
| **Destructive realignment** | `--risky` or config `risky_mode` on the **full branch-hydration** path (same machinery as `--sync`). That path also runs without `--sync` when you pass **`--risky`**, set **`risky_mode`**, use a **non-mainline `branch_mode`** in config, or pass **any `--branch-mode` value** on the CLI (even `mainline`). Hard-reset to upstream only after backup refs. |

> **Warning: `--prune` is opt-in.** On `git fetch`, `--prune` deletes **stale remote-tracking branch refs** (for example `refs/remotes/origin/old-feature` after that branch was removed on the server). That is usually what you want for a tidy clone, but it **removes those ref names locally** until a later fetch brings them back if the branch reappears. Turn pruning on only when you mean to.

**Where `--prune` is decided** (per repo, first applicable source wins):

1. **CLI** — `--prune` or `--no-prune` for this run
2. **Local git config** — `git config --local --bool rain.fetchprune`
3. **Registry** — `fetch_prune` on the repo entry
4. **User config** — `global.fetch_prune`

Precedence chain: **CLI** → **`rain.fetchprune`** → **registry `fetch_prune`** → **global `fetch_prune`**.

Invocation note: `git-rain` and `git rain` are equivalent when `git-rain` is on your PATH.

## Table of Contents

- [Quick Start](#quick-start)
- [Install](#install)
  - [Homebrew (macOS/Linuxbrew)](#homebrew-macoslinuxbrew)
  - [WinGet (Windows)](#winget-windows)
  - [curl installer (Linux / macOS)](#curl-installer-linux--macos)
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
# preview first — lists repos and whether each would get fetch-only vs branch hydration, without running git
# (still does a filesystem scan; the flag name is a little ironic — "dry" rain that still kicks up dust)
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
| curl script | [curl installer](#curl-installer-linux--macos) | Linux, macOS |
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

### curl installer (Linux / macOS)

First-party install script (same idea as [`git-fire/scripts/install.sh`](https://github.com/git-fire/git-fire/blob/main/scripts/install.sh)): downloads the matching `.tar.gz` from [Releases](https://github.com/git-fire/git-rain/releases), verifies `checksums.txt`, and installs to `$INSTALL_DIR` (default `~/.local/bin`).

The `main` URL below always runs the installer script from the latest commit on that branch, while the binary itself comes from the latest GitHub release (or from `VERSION` if you set it). That is convenient for copy-paste installs, but it means the script can drift ahead of any given release. For a fully pinned install, use the release tag in the URL (as in each release’s notes) and set `VERSION` to the same tag.

For repeated automation against the GitHub API (resolving `latest`), set **`GITHUB_TOKEN`** or **`GH_TOKEN`** so authenticated rate limits apply. `VERSION` may be a bare semver (`0.9.1`); the installer tries the `v`-prefixed release tag first, then the exact string you passed.

```bash
curl -fsSL https://raw.githubusercontent.com/git-fire/git-rain/main/scripts/install.sh | bash
```

Pin a version or install directory (environment variables must apply to `bash`, not `curl`):

```bash
curl -fsSL https://raw.githubusercontent.com/git-fire/git-rain/main/scripts/install.sh | VERSION=v0.9.1 INSTALL_DIR=/usr/local/bin bash
```

If your shell does not already include `~/.local/bin` on `PATH`, add it (the installer prints a reminder). Example for bash (skips the line if `.local/bin` is already mentioned in `~/.bashrc`):

```bash
if ! grep -qF '.local/bin' ~/.bashrc 2>/dev/null; then
  echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
fi
source ~/.bashrc
```

Windows is not supported by this script — use **WinGet** or download a `.zip` from Releases.

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

5. **`--risky`** — does not change fetch behavior by itself; on the **full branch-hydration** path (see `--sync` above — entered by `--sync`, **`--risky`**, config **`risky_mode`**, a **non-mainline `branch_mode`**, or **any `--branch-mode` flag** on the CLI) it allows hard reset to upstream after creating `git-rain-backup-*` refs when you would otherwise skip local-only commits.

6. **Report** — one summary line per repo on the default full fetch; per-branch lines on `--fetch-mainline` and `--sync`.

## Key Features

- **One-command workflow** — default full remote fetch, optional `--fetch-mainline`, then `--sync` (+ `branch_mode`) for locals, `--risky` when you accept destructive realignment
- **Safety-first defaults** — never rewrites local-only commits; dirty worktrees are skipped, not clobbered
- **Risky mode** — opt-in destructive realignment: creates a `git-rain-backup-*` ref, then hard-resets to upstream
- **Non-checked-out branches** — updated directly without touching the worktree
- **Interactive TUI (`--rain`)** — streaming repo picker, then the same default fetch, `--fetch-mainline`, or `--sync` behavior
- **Registry** — discovered repos persist across runs; mark repos ignored to skip them permanently
- **Dry run** — preview all repos that would be fetched or synced without making any changes
- **`--fetch-mainline`** — mainline-only remote-tracking ref refresh instead of the default full `git fetch --all`

## Core Commands

```bash
# dry run — preview repos, no changes (still scans disk unless you add --no-scan)
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
| `--dry-run` | No `git fetch` / branch updates — still scans disk to list repos **unless `--no-scan`** (then only registry-known paths are considered). The name is weather-themed irony: no “wet” git work, but not a no-op. |
| `--rain` | Interactive TUI repo picker before running |
| `--sync` | Update local branches from remotes (after `git fetch --all`; default run does not sync locals) |
| `--fetch-mainline` | Mainline-only remote `git fetch` per remote instead of default `git fetch --all` (not with `--sync` or other full-sync triggers) |
| `--branch-mode` | On the **full branch-hydration** path (same triggers as `--sync` — see table above): `mainline`, `checked-out`, `all-local`, or `all-branches` (overrides config `branch_mode` for this run) |
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
scan_depth   = 5                      # max directory depth (default in app: 10)
scan_workers = 8                    # parallel scan workers
fetch_workers = 4                   # parallel per-repo operations (default in app: 4)
risky_mode   = false                # allow destructive realignment on full hydration path
branch_mode  = "mainline"           # full hydration: mainline | checked-out | all-local | all-branches
fetch_prune  = false                # pass --prune on fetch when true (default off; see README warning)
sync_tags    = false                # pass --tags on fetch when true; CLI --tags still forces tags for the run
# Registry default for new repos (TUI / opt-out): leave-untouched | sync-default | sync-all | sync-current-branch
default_mode = "sync-default"
disable_scan = false                # skip scan; use registry only
mainline_patterns = []              # extra mainline names/prefixes when branch_mode = mainline

scan_exclude = [
  "node_modules",
  ".cache",
  "vendor",
]
```

`global.default_mode` must be exactly one of the four values listed above; anything else fails config load.

All options can be overridden with environment variables using the `GIT_RAIN_` prefix:

```bash
GIT_RAIN_GLOBAL_RISKY_MODE=true git-rain
GIT_RAIN_GLOBAL_SCAN_PATH=/tmp/repos git-rain
```

### Garden mode tuning (advanced)

Picking `rain_animation_mode = "garden"` swaps the rain background for a
slow-paced lifecycle: seeds drift down with the rain, plants progress through
sprout, bud, and bloom, then wither and scatter 2–3 new seeds nearby. When the
visible plants cover roughly 80% of the rain strip, the storm clears, the sun
comes out, and the surviving flowers stay forever.

The defaults aim for a calm pace, but a few advanced TOML keys are available
under `[ui]` for tweaking. They are intentionally **not** surfaced in the
in-app settings TUI — leave any unset (or set to `0`) to keep the default.

```toml
[ui]
rain_animation_mode = "garden"

# garden_seed_rate            = 0.055  # fraction of new sky drops that fall as seeds (0..1)
# garden_growth_pace          = 1.0    # multiplier on stage moisture thresholds (>1 = slower)
# garden_bloom_duration_base  = 60     # min frames a flower lingers in full bloom
# garden_bloom_duration_jitter = 40    # extra random frames added to bloom lifetime
# garden_wither_duration      = 28     # frames a withered plant lingers before re-seeding
# garden_offspring_min        = 2      # minimum seeds a dying plant scatters
# garden_offspring_max        = 3      # maximum seeds a dying plant scatters
# garden_offspring_spread     = 3      # X-jitter half-width around the parent column
```

`garden_growth_pace` is the most useful single dial: set it to `2.0` to roughly
halve growth speed, or `0.5` to roughly double it. The other knobs trade
visual density (more or fewer seeds, longer or shorter blooms) for clarity.

### Snow mode and rain panel size

`rain_animation_mode = "snow"` uses the same animation strip for a winter scene:
falling snowflakes, snow that keeps piling on the ground, a small log cabin
with chimney smoke and lit windows, occasional evergreen trees that pick up
frost, and a snowman that grows in stages (two spheres, then face, pipe, and
top hat).

`rain_panel_size` controls how many terminal rows the animation canvas uses:
`compact` (5), `comfortable` (8, default), or `tall` (11). The TUI clamps the
height automatically so the bordered panel still fits short terminals.

```toml
[ui]
rain_animation_mode = "snow"
rain_panel_size = "comfortable"
```

### Config file, locks, and crashes

**Registry (`repos.toml`)** — Writes use a cross-process lock file (`repos.toml.lock`), atomic replace, and stale-lock detection (owner PID). If a process dies mid-run you may still see a leftover lock: the CLI prompts to remove it when safe, or you can use **`--force-unlock-registry`** in scripts. This is the same class of “stale lock / don’t corrupt the database” problem as other multi-repo tools; treat lock removal like any other forced unlock — only when you are sure no other `git-rain` is running.

**User config (`config.toml`)** — Writes use **`config.toml.lock`** with a **bounded wait** (so the `--rain` settings UI does not hang forever if another process holds the lock), then an **atomic replace** (PID-scoped temp file + rename). If the lock cannot be acquired in time, the TUI shows a save error and keeps in-memory settings. Avoid hand-editing `config.toml` while a session is saving; you might leave an orphan `*.tmp` after a crash — safe to delete if present.

## Interactive TUI

`git-rain --rain` opens an interactive picker. Repositories stream in as the filesystem scan finds them — no waiting for the full scan to complete before you can start picking. After you confirm, the tool runs the **default full fetch** (`git fetch --all`, prune opt-in) unless you passed **`--fetch-mainline`**, or **full branch hydration** is implied by **`--sync`**, **`--risky`**, **`risky_mode`** in config, a **non-mainline `branch_mode`**, or **any `--branch-mode`** on the CLI. Quitting (**`q`** or **`ctrl+c`**) cancels the in-progress scan (in-flight `git` subprocesses are aborted via the scan context); **`ctrl+c`** outside raw TTY mode is treated like cancel.

**Key bindings:**

| Key | Action |
|---|---|
| `space` | Toggle repo selection |
| `a` | Select all / deselect all |
| `enter` | Confirm selection and begin fetch or sync |
| `q` / `ctrl+c` | Abort picker |
| `c` / `Esc` | Back from settings (ignored list uses `Esc` / `i` / `b`) |
| `↑` / `↓` | Navigate |

## Safe Mode vs Risky Mode

| Situation | Safe mode (default) | Risky mode (`--risky`) |
|---|---|---|
| Branch is fast-forwardable | ✓ Updated | ✓ Updated |
| Branch has local-only commits | ⊘ Skipped | ⚠ Backed up + reset |
| Checked-out branch, dirty worktree | ⊘ Skipped | ⊘ Skipped |
| No upstream tracked | ⊘ Skipped | ⊘ Skipped |

In risky mode, a backup ref named like `git-rain-backup-<sanitized-branch>-<timestamp>-<short-sha>` is created before any hard reset so local work is always recoverable.

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
