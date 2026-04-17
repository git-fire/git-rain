#!/usr/bin/env bash
set -euo pipefail

# Install git-rain from GitHub release assets with checksum verification.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/git-fire/git-rain/main/scripts/install.sh | bash
# Optional env vars:
#   VERSION=v0.9.1   (must match a GitHub release tag; bare semver like 0.9.1 tries v0.9.1 first)
#   INSTALL_DIR=$HOME/.local/bin
#   REPO=git-fire/git-rain
#   GITHUB_TOKEN or GH_TOKEN  (optional; increases api.github.com rate limits for "latest" resolution)

REPO="${REPO:-git-fire/git-rain}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VERSION:-}"
BINARY_NAME="git-rain"

log() {
  printf '[git-rain install] %s\n' "$1"
}

fail() {
  printf '[git-rain install] ERROR: %s\n' "$1" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

github_token() {
  printf '%s' "${GITHUB_TOKEN:-${GH_TOKEN:-}}"
}

download_to() {
  local src="$1"
  local dst="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$src" -o "$dst"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dst" "$src"
  else
    fail "curl or wget is required"
  fi
}

fetch_github_api() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    local token
    token="$(github_token)"
    if [ -n "$token" ]; then
      curl -fsSL \
        -H "Authorization: Bearer ${token}" \
        -H "Accept: application/vnd.github+json" \
        -H "X-GitHub-Api-Version: 2022-11-28" \
        "$url"
    else
      curl -fsSL "$url"
    fi
  elif command -v wget >/dev/null 2>&1; then
    local token
    token="$(github_token)"
    if [ -n "$token" ]; then
      wget -qO- \
        --header="Authorization: Bearer ${token}" \
        --header="Accept: application/vnd.github+json" \
        --header="X-GitHub-Api-Version: 2022-11-28" \
        "$url"
    else
      wget -qO- "$url"
    fi
  else
    fail "curl or wget is required"
  fi
}

sha256_file() {
  local target="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$target" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$target" | awk '{print $1}'
  else
    fail "sha256sum or shasum is required"
  fi
}

# First field is digest; remainder is filename (GNU text "  " or binary " *").
checksum_for_file() {
  local sums="$1"
  local want="$2"
  awk -v want="$want" '
    {
      hash = $1
      name = $0
      sub(/^[^[:space:]]+[[:space:]]+/, "", name)
      sub(/^\*/, "", name)
      if (name == want) {
        print hash
        exit
      }
    }' "$sums"
}

normalize_os() {
  local raw_os
  raw_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$raw_os" in
    linux) echo "linux" ;;
    darwin) echo "darwin" ;;
    *)
      fail "unsupported OS: $raw_os (expected linux or darwin). On Windows use WinGet or a release .zip from GitHub."
      ;;
  esac
}

normalize_arch() {
  local raw_arch
  raw_arch="$(uname -m)"
  case "$raw_arch" in
    x86_64 | amd64) echo "amd64" ;;
    aarch64 | arm64) echo "arm64" ;;
    # linux/arm is published as armv6 only; armv7l is ABI-compatible.
    armv6l | armv7l) echo "armv6" ;;
    i386 | i686) echo "386" ;;
    *)
      fail "unsupported architecture: $raw_arch"
      ;;
  esac
}

resolve_raw_version_tag() {
  if [ -n "$VERSION" ]; then
    printf '%s\n' "$VERSION"
    return
  fi

  local api_url response tag
  api_url="https://api.github.com/repos/$REPO/releases/latest"
  response="$(fetch_github_api "$api_url")"
  tag="$(printf '%s\n' "$response" | awk -F '"' '/"tag_name"[[:space:]]*:/ {print $4; exit}')"
  [ -n "$tag" ] || fail "could not resolve latest release tag from GitHub API"
  printf '%s\n' "$tag"
}

release_archive_url() {
  local tag="$1"
  local archive_name="$2"
  printf 'https://github.com/%s/releases/download/%s/%s\n' "$REPO" "$tag" "$archive_name"
}

# Return 0 if a release asset URL responds with success (follows redirects).
release_asset_head_ok() {
  local url="$1"
  local code
  if command -v curl >/dev/null 2>&1; then
    code="$(curl -gfsSIL -L -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || printf '%s' "000")"
    [ "$code" = "200" ]
  elif command -v wget >/dev/null 2>&1; then
    wget --spider -q -L "$url"
  else
    false
  fi
}

# Map user input or "latest" API tag to the GitHub release tag that owns the archive.
pick_release_tag() {
  local raw="$1"
  local -a candidates=()
  case "$raw" in
    v*)
      candidates=("$raw" "${raw#v}")
      ;;
    *)
      candidates=("v${raw}" "${raw}")
      ;;
  esac

  local tag archive_version archive_name url
  for tag in "${candidates[@]}"; do
    archive_version="${tag#v}"
    archive_name="${BINARY_NAME}_${archive_version}_${os}_${arch}.tar.gz"
    url="$(release_archive_url "$tag" "$archive_name")"
    if release_asset_head_ok "$url"; then
      printf '%s\n' "$tag"
      return
    fi
  done

  fail "no GitHub release matched VERSION=${raw} for ${os}/${arch} (check tag spelling and that this platform is published)"
}

normalize_path_dir() {
  local d="$1"
  while [ "${#d}" -gt 1 ] && [ "${d%/}" != "$d" ]; do
    d="${d%/}"
  done
  printf '%s\n' "$d"
}

install_binary() {
  local src_bin="$1"
  local target_dir="$2"
  local target_bin="$target_dir/$BINARY_NAME"

  if [ -e "$target_dir" ] && [ ! -d "$target_dir" ]; then
    fail "install path exists but is not a directory: $target_dir"
  fi

  if [ ! -d "$target_dir" ]; then
    if mkdir -p "$target_dir" 2>/dev/null; then
      :
    elif command -v sudo >/dev/null 2>&1; then
      sudo mkdir -p "$target_dir"
    else
      fail "could not create install directory: $target_dir"
    fi
  fi

  if [ -w "$target_dir" ]; then
    install -m 0755 "$src_bin" "$target_bin"
    return
  fi

  if command -v sudo >/dev/null 2>&1; then
    sudo install -m 0755 "$src_bin" "$target_bin"
    return
  fi

  fail "install directory is not writable and sudo is unavailable: $target_dir"
}

path_has_dir() {
  local dir
  dir="$(normalize_path_dir "$1")"
  case ":${PATH:-}:" in
    *":${dir}:"*) return 0 ;;
    *) return 1 ;;
  esac
}

# Preflight: fail fast before downloads (curl/wget checked in download_to).
need_cmd tar
need_cmd install
os="$(normalize_os)"
arch="$(normalize_arch)"
version_tag="$(pick_release_tag "$(resolve_raw_version_tag)")"
version="${version_tag#v}"

archive_name="${BINARY_NAME}_${version}_${os}_${arch}.tar.gz"
archive_url="$(release_archive_url "$version_tag" "$archive_name")"
checksums_url="$(release_archive_url "$version_tag" "checksums.txt")"

log "installing ${BINARY_NAME} ${version_tag} for ${os}/${arch}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

archive_path="$tmp_dir/$archive_name"
checksums_path="$tmp_dir/checksums.txt"

log "downloading release archive"
download_to "$archive_url" "$archive_path"

log "downloading checksum file"
download_to "$checksums_url" "$checksums_path"

expected_sum="$(checksum_for_file "$checksums_path" "$archive_name")"
[ -n "$expected_sum" ] || fail "could not find checksum entry for $archive_name (no asset for this OS/arch on this release?)"

actual_sum="$(sha256_file "$archive_path")"
if [ "$expected_sum" != "$actual_sum" ]; then
  fail "checksum mismatch for $archive_name"
fi

log "checksum verified"
tar -xzf "$archive_path" -C "$tmp_dir"
[ -f "$tmp_dir/$BINARY_NAME" ] || fail "archive did not contain $BINARY_NAME"

install_binary "$tmp_dir/$BINARY_NAME" "$INSTALL_DIR"

log "installed to $INSTALL_DIR/$BINARY_NAME"
if ! path_has_dir "$INSTALL_DIR"; then
  log "warning: $INSTALL_DIR is not on PATH; add it to your shell profile, e.g. export PATH=\"$INSTALL_DIR:\$PATH\""
fi
log "verify with: $BINARY_NAME --version"
