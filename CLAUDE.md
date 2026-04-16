# CLAUDE.md — git-rain

## Project Overview

`git-rain` is a standalone Go CLI for pulling remote state back down — the reverse of `git-fire`. By default it runs **`git fetch --all --prune`** per repo after scanning (remote-tracking refs update; local branches are not moved). Use **`--mainline-fetch`** for targeted mainline remote-tracking fetches only; **`--sync`** runs full local branch hydration (fast-forward / safe skip / risky reset). It is extracted from the `git-fire` codebase and promoted to a first-class tool.

Module: `github.com/git-rain/git-rain`
Go version: 1.24.2
Config: `~/.config/git-rain/config.toml`
Registry: `~/.config/git-rain/repos.toml`
Logs: `~/.cache/git-rain/` (not yet wired up)

---

## Commands

```bash
make build       # compile binary to ./git-rain
make run ARGS="--dry-run"  # build and run with flags
make test        # run all tests
make test-race   # run tests with race detector (used in CI)
make lint        # go vet ./...
make install     # install to $GOPATH/bin
make clean       # remove binary
```

Run directly:

```bash
go build ./...
go test -race -count=1 ./...
go vet ./...
```

---

## Architecture

```
main.go
  └── cmd/root.go            # Cobra CLI: flags, orchestration (rain is the root command)
      ├── internal/config    # Load config (~/.config/git-rain/config.toml)
      ├── internal/git       # Repo scanning + git operations (shells out to git binary)
      ├── internal/registry  # Persistent repo registry (~/.config/git-rain/repos.toml)
      ├── internal/safety    # Secret detection + error/log sanitization
      └── internal/flavor    # Rain-themed startup quotes
```

**Key design decisions:**
- Uses native `git` binary via `exec.Command` — not go-git.
- Default run: `git fetch --all --prune` (optional `--tags` from config/flag). Mainline-only remote fetch: `internal/git.MainlineFetchRemotes` when `--mainline-fetch`. Full hydrate: `RainRepository` when `--sync`, non-mainline `branch_mode`, or risky mode is active.
- Interactive picker: `--rain` (mirrors `git-fire --fire`).
- Backup branch prefix: `git-rain-backup-` (was `git-fire-rain-backup-` in git-fire).
- Config env prefix: `GIT_RAIN_`.
- Safe mode (default): never rewrites local-only commits (applies to `--sync` path).
- Risky mode (`--risky` / `config: global.risky_mode`): allows hard reset to upstream after creating a `git-rain-backup-*` ref (implies full sync).

---

## Testing

- Run `make test-race` before considering a change done.
- Prefer table-driven tests for multi-case functions.
- Integration-style tests that shell out to `git` are preferred for `internal/git`.
- Use `github.com/git-fire/git-testkit` helpers.

---

## Conventions

- **No go-git**: all git interactions shell out to the system `git` binary.
- **Cobra for CLI**: root command lives in `cmd/root.go`.
- **Config via Viper/TOML**: user config at `~/.config/git-rain/config.toml`; env vars override.
- **Error handling**: return errors up to the caller; only `log.Fatal`/`os.Exit` in `main.go`.
