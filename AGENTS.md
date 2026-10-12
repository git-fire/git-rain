# AGENTS.md

<!-- agentic-guardrails:start -->
## Agentic guardrails

These notes apply to humans and to automated agents (including Cloud Agents).

1. **Autonomous work comes first**  
   The default posture is to **finish useful work without unnecessary human gates**. Prefer actions that keep momentum (fetch, build, test, open PRs) over asking for permission for routine steps—while still honoring the hard safety rules below.

2. **Multi-repository work is encouraged**  
   When a task genuinely spans more than one codebase (services, libraries, docs, sibling CLIs), work across **multiple repositories or local clones** as needed. Open **one PR per repository**, keep each PR scoped, and **link related PRs** in every description so reviewers see the coordinated change.

3. **Stay on a fresh enough tip; rebase is optional**  
   Before substantive work: `git fetch origin`, check out your branch, then prefer **`git merge --ff-only origin/<branch>`** (or `git pull --ff-only` when upstream exists). If you cannot fast-forward, pause and use the repository’s normal merge or rebase workflow—**do not** silently work on a stale checkout. **`git pull --rebase`** is *ideal* when updating an active feature branch, but it is **not required**; a merge commit or team-standard flow is fine when it avoids needless churn.

4. **Force-push is never automatic and needs explicit human buy-in**  
   Do **not** run `git push --force`, `git push --force-with-lease`, or rewrite published history on your own. If you believe it might be warranted, **stop** and give the **human** explicit **reasoning**, **effects** on collaborators, CI, and open PRs, and **why** a force-push would be needed versus safer alternatives (new branch, revert, merge). Proceed **only** after they **explicitly approve** that exact repository and branch.

5. **Focused changes and verification**  
   Keep pull requests focused; run this repository’s standard build, test, and lint commands (see `README`, `Makefile`, or `CLAUDE.md`) before requesting review.

6. **Workflow shape is yours**  
   Using **git worktree** versus a single working directory is an **operator choice**; these docs do **not** require worktrees.

<!-- agentic-guardrails:end -->

---

## What git-rain does

`git-rain` discovers git repositories under a scan path (and in the persistent registry), then pulls remote state **inward** — the counterpart to [`git-fire`](https://github.com/git-fire/git-fire), which pushes commits **outward**.

| Mode | Command shape | Mutates local branch refs? | Network |
|------|----------------|----------------------------|---------|
| Default | `git-rain` | No — `git fetch --all` only | Yes (per remote) |
| Mainline fetch | `git-rain --fetch-mainline` | No | Yes |
| Hydrate locals | `git-rain --sync` | Yes (safe by default) | Yes |
| Risky hydrate | `git-rain --sync --risky` | Yes — may hard-reset after backup ref | Yes |
| Preview | `git-rain --dry-run` | No | No git network (scan only) |

See [README.md](README.md) for flags, config, and TUI (`--rain`).

## When an agent should use it

Use **git-rain** when the user wants to:

- Refresh **many clones** after upstream changed (monorepo sandboxes, `~/projects`, CI checkouts).
- **Fetch all remotes** without moving local branches (default) before running tests or merges.
- **Fast-forward local branches** where safe (`--sync`), instead of hand-running `git pull` per repo.

Prefer **single-repo** `git fetch` / `git pull` when only one repository matters.

Use **git-fire** (not git-rain) when the task is to **commit and push** local work across repos. Typical pair: `git-rain` to pull remote state down, then `git-fire` to push commits up.

## Install

```bash
# Go toolchain (Go 1.24.2+)
go install github.com/git-rain/git-rain@latest

# macOS / Linuxbrew
brew tap git-fire/tap && brew install git-rain

# Script install (binary from GitHub Releases)
curl -fsSL https://raw.githubusercontent.com/git-fire/git-rain/main/scripts/install.sh | bash
```

Verify: `git-rain --version` and `git-rain --help`.

Requires **`git` on PATH**. Config template: `git-rain --init` → `~/.config/git-rain/config.toml`.

## Agent-friendly commands (copy-paste)

Scope runs to a sandbox and prefer machine-readable output:

```bash
# Preview which repos would be touched (no git fetch)
git-rain --dry-run --non-interactive --json --path /path/to/scan-root

# Default: git fetch --all per repo (remote-tracking refs only)
git-rain --non-interactive --json --path /path/to/scan-root

# Lighter: mainline remote-tracking refs only
git-rain --non-interactive --json --fetch-mainline --path /path/to/scan-root

# Hydrate local branches (safe mode — skips local-only commits)
git-rain --non-interactive --json --sync --path /path/to/scan-root

# Registry-only (no filesystem walk)
git-rain --non-interactive --json --no-scan

# Stale registry lock (only if sure no other git-rain is running)
git-rain --force-unlock-registry --non-interactive --path /path/to/scan-root
```

Environment:

- `GIT_RAIN_NON_INTERACTIVE=1` — treat the run as non-interactive (recommended in CI / Cloud Agents).
- `GIT_RAIN_GLOBAL_SCAN_PATH`, `GIT_RAIN_GLOBAL_RISKY_MODE`, etc. — override config (see README).

**Do not** use `--rain` in scripted or agent runs; it opens the Bubble Tea TUI and requires an interactive terminal.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success; or completed with **frozen** fetches (network/auth) but no hard failures |
| `1` | Error: missing `git`, bad flags, config load failure, registry lock, scan failure, or one or more repos/branches **failed** on a live run |

Human mode prints a summary line; **`--json`** is the stable contract for agents.

## `--json` output (`schema_version` 1)

Emit with `--json` on stdout (no emoji banners). Fields:

```json
{
  "schema_version": 1,
  "tool": "git-rain",
  "version": "v1.0.0",
  "finished_at": "2026-10-11T12:00:00Z",
  "run": {
    "dry_run": false,
    "full_sync": false,
    "fetch_mainline": false,
    "risky_mode": false,
    "branch_mode": "mainline",
    "fetch_prune": false
  },
  "summary": {
    "repos": 1,
    "updated": 1,
    "skipped": 0,
    "frozen": 0,
    "failed": 0
  },
  "repositories": [
    {
      "index": 1,
      "total": 1,
      "name": "my-repo",
      "path": "/abs/path/my-repo",
      "operation": "fetch-all",
      "summary_outcome": "fetched"
    }
  ]
}
```

- `operation`: `fetch-all`, `fetch-mainline`, `hydrate`, or `error`.
- `summary_outcome`: machine outcome (`fetched`, `updated`, `frozen`, `failed`, `up-to-date`, …).
- Hydrate / mainline runs include `branches[]` with `outcome`, `label`, optional `message` and `backup_branch`.

Dry-run JSON sets `run.dry_run: true` and lists `repositories` with `name` and `path` only.

Future MCP / skill wrappers: [docs/agent-integration.md](docs/agent-integration.md).

## Safety and side effects

**Usually safe (default fetch):** updates remote-tracking refs; does not move local branches.

**`--sync` (safe mode):** fast-forwards locals when possible; **skips** local-only commits, dirty worktrees, and unsafe merges.

**`--sync --risky`:** may create `git-rain-backup-*` refs and **hard-reset** branches to upstream — require explicit human approval before agents use this.

**`--prune`:** removes stale `refs/remotes/*` names locally — opt-in only.

**Registry:** `~/.config/git-rain/repos.toml` is updated as repos are discovered (paths, last-seen). Use `--path` to limit scope.

**Locks:** concurrent runs use `repos.toml.lock`. Stale locks are removed automatically; live locks need `--force-unlock-registry` when non-interactive.

**Network / auth:** shells out to system `git` with credential prompts disabled for fetch operations; auth failures surface as **frozen** or error messages — configure SSH/credential helper; do not expect password prompts.

**Secrets:** errors are sanitized; still avoid putting tokens in remote URLs or paths.

## Worker pool (current behavior)

Live runs pipeline **filesystem scan → registry upsert → parallel per-repo work** (`global.fetch_workers`, default 4). Repos may be processed concurrently; human log lines can interleave unless you use `--json`. There is **no per-repo timeout or automatic retry** today.

## Maintainer commands (this repo)

| Task | Command |
|------|---------|
| Build | `make build` |
| Lint | `make lint` (`go vet`) |
| Test | `make test-race` (~60–75s; shells out to real `git`) |
| Run | `make run ARGS="--dry-run --path …"` |

---

## Cursor Cloud specific instructions

`git-rain` is a single standalone Go CLI (no servers, DBs, or external services). Non-obvious caveats for this environment:

- **`make lint` is only `go vet`.** CI's full `lint` job uses `golangci-lint` (pinned `v2.11.4`) and `shellcheck` on `scripts/install.sh`; neither is in the update script. Install on demand for CI parity: `curl -fsSL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.11.4` and `sudo apt-get install -y shellcheck`. Then `golangci-lint run --timeout=5m`.
- **`make test-race` is slow (~60–75s)** because `internal/git` tests shell out to the real `git` binary. This is expected, not a hang.
- **Running the CLI mutates user state.** It persists a registry at `~/.config/git-rain/repos.toml` and config at `~/.config/git-rain/config.toml`, so repos discovered in one run are remembered on the next. For isolated runs/tests, always pass `--path <sandbox>` (and `--dry-run` to preview without touching git).
- **Manual end-to-end check:** create a bare remote + a clone under a scan dir, advance the remote, then `./git-rain --sync --path <dir>` fast-forwards the local branch. Requires `git user.name`/`user.email` to be set (already configured here).
