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

## Cursor Cloud specific instructions

`git-rain` is a single standalone Go CLI (no servers, DBs, or external services). Standard commands live in `Makefile` / `CLAUDE.md` (`make build`, `make test-race`, `make lint`, `make run ARGS=...`). Non-obvious caveats for this environment:

- **`make lint` is only `go vet`.** CI's full `lint` job uses `golangci-lint` (pinned `v2.11.4`) and `shellcheck` on `scripts/install.sh`; neither is in the update script. Install on demand for CI parity: `curl -fsSL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.11.4` and `sudo apt-get install -y shellcheck`. Then `golangci-lint run --timeout=5m`.
- **`make test-race` is slow (~60–75s)** because `internal/git` tests shell out to the real `git` binary. This is expected, not a hang.
- **Running the CLI mutates user state.** It persists a registry at `~/.config/git-rain/repos.toml` and config at `~/.config/git-rain/config.toml`, so repos discovered in one run are remembered on the next. For isolated runs/tests, always pass `--path <sandbox>` (and `--dry-run` to preview without touching git).
- **Manual end-to-end check:** create a bare remote + a clone under a scan dir, advance the remote, then `./git-rain --sync --path <dir>` fast-forwards the local branch. Requires `git user.name`/`user.email` to be set (already configured here).

