#!/usr/bin/env bash
# UAT script for git-rain — end-to-end smoke tests using real git repos.
# Requires: bash 4+, git, go (to build the binary).
#
# Usage:
#   ./scripts/uat.sh             # run all scenarios
#   ./scripts/uat.sh --keep-tmp  # keep temp dir for inspection on failure
#
# Exit code: 0 = all pass, non-zero = at least one failure.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY="${REPO_ROOT}/git-rain"
KEEP_TMP=false

for arg in "$@"; do
  case "$arg" in
    --keep-tmp) KEEP_TMP=true ;;
    *) echo "unknown arg: $arg" >&2; exit 1 ;;
  esac
done

# ── Build ──────────────────────────────────────────────────────────────────
echo "==> building git-rain"
(cd "$REPO_ROOT" && go build -o git-rain .)

# ── Temp workspace ─────────────────────────────────────────────────────────
# Use UAT_TMPDIR (not TMPDIR) to avoid shadowing the shell env var used by mktemp.
UAT_TMPDIR="$(mktemp -d)"
cleanup() {
  if [[ "$KEEP_TMP" == false ]]; then
    rm -rf "$UAT_TMPDIR"
  else
    echo "tmp dir preserved: $UAT_TMPDIR"
  fi
}
trap cleanup EXIT

export GIT_AUTHOR_NAME="UAT Bot"
export GIT_AUTHOR_EMAIL="uat@git-rain.test"
export GIT_COMMITTER_NAME="UAT Bot"
export GIT_COMMITTER_EMAIL="uat@git-rain.test"

PASS=0
FAIL=0

pass() { echo "  ✓ $1"; PASS=$((PASS + 1)); }
fail() { echo "  ✗ $1"; FAIL=$((FAIL + 1)); }
assert_contains() {
  local label="$1" haystack="$2" needle="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    pass "$label"
  else
    fail "$label (missing: '$needle')"
    echo "    output was:"
    echo "$haystack" | sed 's/^/      /'
  fi
}
assert_not_contains() {
  local label="$1" haystack="$2" needle="$3"
  if ! echo "$haystack" | grep -qF "$needle"; then
    pass "$label"
  else
    fail "$label (unexpectedly found: '$needle')"
    echo "    output was:"
    echo "$haystack" | sed 's/^/      /'
  fi
}

git_init_bare() {
  git init --bare --quiet "$1"
}

git_clone_local() {
  git clone --quiet "$1" "$2"
}

commit_file() {
  local repo="$1" name="$2" content="$3" msg="$4"
  echo "$content" > "${repo}/${name}"
  git -C "$repo" add "$name"
  git -C "$repo" commit --quiet -m "$msg"
}

# scenario_begin <name>
# Sets the current scenario directory ($SCENARIO_D) and isolates HOME so
# the registry and config of each scenario don't bleed into the next.
# Must be called directly (not via $()), so exports propagate to the shell.
SCENARIO_D=""
scenario_begin() {
  local name="$1"
  SCENARIO_D="${UAT_TMPDIR}/${name}"
  local h="${SCENARIO_D}/home"
  mkdir -p "$h"
  export HOME="$h"
  export XDG_CONFIG_HOME="${h}/.config"
  export XDG_CACHE_HOME="${h}/.cache"
}

# ── SCENARIO 1: fast-forward mainline branch ───────────────────────────────
echo
echo "── scenario 1: fast-forward mainline branch"
{
  scenario_begin s1
  d="$SCENARIO_D"
  remote="${d}/remote.git"
  local="${d}/workspace/myrepo"

  git_init_bare "$remote"
  git_clone_local "$remote" "$local"
  git -C "$local" checkout --quiet -b main 2>/dev/null || true
  commit_file "$local" "init.txt" "v1" "init"
  git -C "$local" push --quiet origin HEAD:main
  commit_file "$local" "update.txt" "v2" "upstream update"
  git -C "$local" push --quiet origin HEAD:main

  # Reset local to one commit behind
  git -C "$local" reset --quiet --hard HEAD~1

  out="$("$BINARY" --path "${d}/workspace" --branch-mode mainline 2>&1)"
  assert_contains "s1: main fast-forwarded" "$out" "synced"
  assert_not_contains "s1: no failures" "$out" "failed"
  assert_not_contains "s1: no frozen" "$out" "frozen"
}

# ── SCENARIO 2: local-ahead branch is preserved in safe mode ──────────────
echo
echo "── scenario 2: local-ahead branch preserved (safe mode)"
{
  scenario_begin s2
  d="$SCENARIO_D"
  remote="${d}/remote.git"
  local="${d}/workspace/myrepo"

  git_init_bare "$remote"
  git_clone_local "$remote" "$local"
  git -C "$local" checkout --quiet -b main 2>/dev/null || true
  commit_file "$local" "init.txt" "v1" "init"
  git -C "$local" push --quiet origin HEAD:main
  commit_file "$local" "local.txt" "extra" "local-only commit"
  local_sha="$(git -C "$local" rev-parse HEAD)"

  out="$("$BINARY" --path "${d}/workspace" --branch-mode all-local 2>&1)"
  assert_contains "s2: local ahead label" "$out" "local ahead"
  current_sha="$(git -C "$local" rev-parse HEAD)"
  if [[ "$current_sha" == "$local_sha" ]]; then
    pass "s2: local-ahead SHA preserved"
  else
    fail "s2: local-ahead SHA changed (was $local_sha, now $current_sha)"
  fi
}

# ── SCENARIO 3: up-to-date branch shows current ───────────────────────────
echo
echo "── scenario 3: up-to-date branch"
{
  scenario_begin s3
  d="$SCENARIO_D"
  remote="${d}/remote.git"
  local="${d}/workspace/myrepo"

  git_init_bare "$remote"
  git_clone_local "$remote" "$local"
  git -C "$local" checkout --quiet -b main 2>/dev/null || true
  commit_file "$local" "init.txt" "v1" "init"
  git -C "$local" push --quiet origin HEAD:main

  out="$("$BINARY" --path "${d}/workspace" --branch-mode mainline 2>&1)"
  assert_contains "s3: current shown" "$out" "current"
  assert_not_contains "s3: no updates" "$out" "synced"
}

# ── SCENARIO 4: dry-run shows repos without modifying ─────────────────────
echo
echo "── scenario 4: dry-run"
{
  scenario_begin s4
  d="$SCENARIO_D"
  remote="${d}/remote.git"
  local="${d}/workspace/myrepo"

  git_init_bare "$remote"
  git_clone_local "$remote" "$local"
  git -C "$local" checkout --quiet -b main 2>/dev/null || true
  commit_file "$local" "init.txt" "v1" "init"
  git -C "$local" push --quiet origin HEAD:main
  commit_file "$local" "update.txt" "v2" "upstream update"
  git -C "$local" push --quiet origin HEAD:main
  git -C "$local" reset --quiet --hard HEAD~1
  local_sha="$(git -C "$local" rev-parse HEAD)"

  out="$("$BINARY" --path "${d}/workspace" --dry-run 2>&1)"
  assert_contains "s4: dry-run mentions repo" "$out" "myrepo"
  current_sha="$(git -C "$local" rev-parse HEAD)"
  if [[ "$current_sha" == "$local_sha" ]]; then
    pass "s4: dry-run did not modify repo"
  else
    fail "s4: dry-run modified repo (sha changed)"
  fi
}

# ── SCENARIO 5: mainline mode skips feature branches ──────────────────────
echo
echo "── scenario 5: mainline mode skips feature branches"
{
  scenario_begin s5
  d="$SCENARIO_D"
  remote="${d}/remote.git"
  local="${d}/workspace/myrepo"

  git_init_bare "$remote"
  git_clone_local "$remote" "$local"
  git -C "$local" checkout --quiet -b main 2>/dev/null || true
  commit_file "$local" "init.txt" "v1" "init"
  git -C "$local" push --quiet origin HEAD:main

  # Create a feature branch that is behind its remote
  git -C "$local" checkout --quiet -b feature/my-work
  commit_file "$local" "feat.txt" "feature" "feature commit"
  git -C "$local" push --quiet origin feature/my-work
  commit_file "$local" "feat2.txt" "more" "upstream feature"
  git -C "$local" push --quiet origin feature/my-work
  git -C "$local" reset --quiet --hard HEAD~1
  feature_sha="$(git -C "$local" rev-parse HEAD)"

  git -C "$local" checkout --quiet main

  out="$("$BINARY" --path "${d}/workspace" --branch-mode mainline 2>&1)"
  current_feature_sha="$(git -C "$local" rev-parse feature/my-work)"
  if [[ "$current_feature_sha" == "$feature_sha" ]]; then
    pass "s5: feature branch not touched by mainline mode"
  else
    fail "s5: feature branch was modified by mainline mode"
  fi
}

# ── SCENARIO 6: risky mode resets local-ahead branch ──────────────────────
echo
echo "── scenario 6: risky mode resets local-ahead branch"
{
  scenario_begin s6
  d="$SCENARIO_D"
  remote="${d}/remote.git"
  local="${d}/workspace/myrepo"

  git_init_bare "$remote"
  git_clone_local "$remote" "$local"
  git -C "$local" checkout --quiet -b main 2>/dev/null || true
  commit_file "$local" "init.txt" "v1" "init"
  git -C "$local" push --quiet origin HEAD:main
  remote_sha="$(git -C "$local" rev-parse HEAD)"
  commit_file "$local" "local.txt" "extra" "local-only"

  out="$("$BINARY" --path "${d}/workspace" --branch-mode all-local --risky 2>&1)"
  assert_contains "s6: realigned shown" "$out" "realigned"
  current_sha="$(git -C "$local" rev-parse HEAD)"
  if [[ "$current_sha" == "$remote_sha" ]]; then
    pass "s6: branch reset to remote SHA"
  else
    fail "s6: branch not reset (want $remote_sha, got $current_sha)"
  fi
  if git -C "$local" branch | grep -q "git-rain-backup-"; then
    pass "s6: backup branch created"
  else
    fail "s6: no backup branch found"
  fi
}

# ── SCENARIO 7: default full fetch (no local branch moves) ─────────────────
echo
echo "── scenario 7: default git fetch --all --prune"
{
  scenario_begin s7
  d="$SCENARIO_D"
  remote="${d}/remote.git"
  local="${d}/workspace/myrepo"

  git_init_bare "$remote"
  git_clone_local "$remote" "$local"
  git -C "$local" checkout --quiet -b main 2>/dev/null || true
  commit_file "$local" "init.txt" "v1" "init"
  git -C "$local" push --quiet origin HEAD:main
  local_sha="$(git -C "$local" rev-parse HEAD)"

  out="$("$BINARY" --path "${d}/workspace" 2>&1)"
  assert_contains "s7: fetched shown" "$out" "fetched"
  current_sha="$(git -C "$local" rev-parse HEAD)"
  if [[ "$current_sha" == "$local_sha" ]]; then
    pass "s7: default fetch did not move local ref"
  else
    fail "s7: default fetch modified local ref"
  fi
}

# ── SCENARIO 8: no repos found ────────────────────────────────────────────
echo
echo "── scenario 8: no repos found"
{
  scenario_begin s8
  d="$SCENARIO_D"
  mkdir -p "${d}/workspace"

  out="$("$BINARY" --path "${d}/workspace" 2>&1)"
  assert_contains "s8: no repos message" "$out" "No git repositories found"
}

# ── SCENARIO 9: user forecast patterns extend mainline ────────────────────
echo
echo "── scenario 9: forecast patterns extend mainline"
{
  scenario_begin s9
  d="$SCENARIO_D"
  remote="${d}/remote.git"
  local="${d}/workspace/myrepo"

  git_init_bare "$remote"
  git_clone_local "$remote" "$local"
  git -C "$local" checkout --quiet -b main 2>/dev/null || true
  commit_file "$local" "init.txt" "v1" "init"
  git -C "$local" push --quiet origin HEAD:main

  # Create a JIRA- prefixed branch (user forecast pattern)
  git -C "$local" checkout --quiet -b JIRA-123-my-ticket
  commit_file "$local" "ticket.txt" "work" "ticket commit"
  git -C "$local" push --quiet origin JIRA-123-my-ticket
  commit_file "$local" "ticket2.txt" "more" "upstream update"
  git -C "$local" push --quiet origin JIRA-123-my-ticket
  git -C "$local" reset --quiet --hard HEAD~1
  git -C "$local" checkout --quiet main

  # Write a config with JIRA- forecast pattern
  cfg_dir="${HOME}/.config/git-rain"
  mkdir -p "$cfg_dir"
  cat > "${cfg_dir}/config.toml" <<'TOML'
[global]
branch_mode = "mainline"
mainline_patterns = ["JIRA-"]
TOML

  out="$("$BINARY" --path "${d}/workspace" --sync 2>&1)"
  assert_contains "s9: JIRA branch listed" "$out" "JIRA-123-my-ticket"
  assert_contains "s9: JIRA branch fast-forwarded" "$out" "synced"
}

# ── Summary ────────────────────────────────────────────────────────────────
echo
echo "══════════════════════════════════════════"
echo "  results: ${PASS} passed, ${FAIL} failed"
echo "══════════════════════════════════════════"

if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
