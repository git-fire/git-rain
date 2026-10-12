# Agent integration sketch (design only)

This document describes how a future **MCP server** or **agent skill** could wrap the `git-rain` CLI. Nothing here is implemented in this repository yet; it is guidance for tooling authors and for parity with [git-fire](https://github.com/git-fire/git-fire) agent docs.

## Goals

- Let coding agents refresh many local clones without bespoke shell loops.
- Never hang on credentials, registry locks, or TUI prompts.
- Return structured results agents can branch on (success, skipped, frozen network, failed).

## Recommended CLI surface

Always invoke with:

```bash
git-rain --non-interactive --path "$SCAN_ROOT" --json …
```

| User intent | Suggested flags |
|-------------|-----------------|
| Preview repos / plan | `--dry-run --json` |
| Safe remote refresh (default) | `--json` |
| Lighter fetch | `--fetch-mainline --json` |
| Update local branches | `--sync --json` |
| Destructive realignment (human-approved only) | `--sync --risky --json` |
| Registry lock from crashed run | `--force-unlock-registry` (only when no other instance is running) |

Avoid `--rain` in agent flows; it requires an interactive terminal.

Set `GIT_RAIN_NON_INTERACTIVE=1` in the environment when stdin may not be a TTY (CI, Cloud Agents, headless sandboxes). Network git subprocesses already disable terminal credential prompts (via shared git-harness behavior with git-fire).

## MCP server shape (proposed)

**Server name:** `git-rain` (or hosted alongside a multi-tool `git-sync` server with `git-fire` push tools).

### Tools

1. **`rain_dry_run`**
   - Args: `scan_path` (optional), `no_scan` (bool), `sync` (bool), `fetch_mainline` (bool), `branch_mode` (optional enum)
   - Runs: `git-rain --dry-run --non-interactive --json --path …`
   - Returns: parsed JSON document (`schema_version` 1)

2. **`rain_fetch`**
   - Args: same scope flags + `prune` / `no_prune` / `tags`
   - Runs: default fetch-all path with `--json`

3. **`rain_sync`**
   - Args: scope flags + `risky` (bool, default false) + `branch_mode`
   - Runs: `--sync --json` (refuses `--risky` unless a separate `confirm_risky: true` arg is set)

4. **`rain_registry_unlock`** (dangerous)
   - Args: `force` (must be true)
   - Runs: `--force-unlock-registry` with a no-op dry path or explicit subcommand in a future release

### Resources (optional)

- `config://git-rain` — read-only snapshot of `~/.config/git-rain/config.toml` (redact paths if policy requires)
- `registry://git-rain` — read-only `repos.toml` for known paths

## Skill shape (proposed)

A Cursor / Claude **skill** would:

1. Check `git-rain --version` or install via `go install github.com/git-rain/git-rain@latest`.
2. Prefer a dedicated `scan_path` under the user’s workspace rather than the user’s entire home directory.
3. Run `--dry-run --json` before mutating remotes when the user’s task is exploratory.
4. Parse `summary.failed` and `summary.frozen` from JSON; surface auth/network errors without retry loops that hammer remotes.
5. Link to **git-fire** when the task is outbound (commit/push) after rain succeeded.

## JSON contract

Agents should depend on `--json` output (`schema_version: 1`), not emoji log lines.

Top-level fields:

- `schema_version`, `tool`, `version`, `finished_at`
- `run`: `dry_run`, `full_sync`, `fetch_mainline`, `risky_mode`, `branch_mode`, `fetch_prune`
- `summary`: `repos`, `updated`, `skipped`, `frozen`, `failed`
- `repositories[]`: `name`, `path`, `operation` (`fetch-all` | `fetch-mainline` | `hydrate` | `error`), `summary_outcome`, optional `branches[]` with `outcome` / `label` / `message`

Dry-run JSON uses the same header fields with `run.dry_run: true` and a flat `repositories` list (`name`, `path` only).

Exit code: `0` on success (including some frozen fetches); `1` on validation errors or when `summary.failed > 0` on a live run.

## Worker pool note

Live runs use a **scan → worker pool** pipeline (`fetch_workers` from config, default 4). Workers start fetching as repos are discovered; there is **no per-repo timeout or retry** in the CLI today. A wrapper should not assume serial execution or deterministic ordering of log lines in human mode; use `--json` for ordered `index` fields.

## Security

- Inherits git credentials from the environment (SSH agent, credential helper).
- `--sync --risky` can hard-reset branches after creating `git-rain-backup-*` refs; gate behind human confirmation in any MCP tool.
- `--prune` removes stale remote-tracking refs; keep off unless the user wants pruning.

## Related work

- [git-fire agent integration](https://github.com/git-fire/git-fire/blob/main/docs/agent-integration.md) (when published on git-fire `main`)
- [llms.txt](../llms.txt) and [AGENTS.md](../AGENTS.md) in this repo
