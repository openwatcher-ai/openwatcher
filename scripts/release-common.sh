#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="${OPENWATCHER_RELEASE_DIR:-${RELEASE_DIR:-$ROOT_DIR/dist/release}}"
CURRENT_COMMIT="$(git -C "$ROOT_DIR" rev-parse --short HEAD)"
BUILT_AT_UTC="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

note() {
  printf '%s\n' "$*" >&2
}

die() {
  printf '%s\n' "$*" >&2
  exit 2
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "缺少命令：$1"
}

trim_value() {
  printf '%s' "${1:-}" | awk '{$1=$1; print}'
}

git_short_commit() {
  git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || printf 'local'
}

dev_build_version() {
  printf 'dev-%s' "$(git_short_commit)"
}

validate_semver_version() {
  local label="$1"
  local value="$2"
  [[ "$value" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.+][0-9A-Za-z][0-9A-Za-z.+-]*)?$ ]] \
    || die "$label 不符合版本规则：$value"
}

validate_version_code() {
  local label="$1"
  local value="$2"
  [[ "$value" =~ ^[1-9][0-9]*$ ]] \
    || die "$label 必须是正整数：$value"
}

require_desktop_version() {
  local value
  value="$(trim_value "${OPENWATCHER_DESKTOP_VERSION:-}")"
  [[ -n "$value" ]] || die "缺少 Desktop 版本，请设置 OPENWATCHER_DESKTOP_VERSION"
  validate_semver_version "Desktop 版本" "$value"
  printf '%s' "$value"
}

require_watch_version_name() {
  local value
  value="$(trim_value "${OPENWATCHER_WATCH_VERSION_NAME:-}")"
  [[ -n "$value" ]] || die "缺少 Watch versionName，请设置 OPENWATCHER_WATCH_VERSION_NAME"
  validate_semver_version "Watch versionName" "$value"
  printf '%s' "$value"
}

require_watch_version_code() {
  local value
  value="$(trim_value "${OPENWATCHER_WATCH_VERSION_CODE:-}")"
  [[ -n "$value" ]] || die "缺少 Watch versionCode，请设置 OPENWATCHER_WATCH_VERSION_CODE"
  validate_version_code "Watch versionCode" "$value"
  printf '%s' "$value"
}

ensure_release_dir() {
  mkdir -p "$RELEASE_DIR"
}

current_platform_id() {
  printf '%s-%s\n' "$(uname -s | tr '[:upper:]' '[:lower:]')" "$(uname -m | sed 's/aarch64/arm64/; s/x86_64/amd64/')"
}

platform_go_values() {
  case "$1" in
    darwin-arm64) printf 'darwin arm64\n' ;;
    darwin-amd64) printf 'darwin amd64\n' ;;
    windows-amd64) printf 'windows amd64\n' ;;
    windows-arm64) printf 'windows arm64\n' ;;
    *)
      die "不支持的平台标识：$1"
      ;;
  esac
}

sha256_file() {
  shasum -a 256 "$1" | awk '{print $1}'
}

file_size_bytes() {
  if stat -c '%s' "$1" >/dev/null 2>&1; then
    stat -c '%s' "$1"
  else
    stat -f '%z' "$1"
  fi
}

watch_version_name() {
  trim_value "${OPENWATCHER_WATCH_VERSION_NAME:-}"
}

watch_version_code() {
  trim_value "${OPENWATCHER_WATCH_VERSION_CODE:-}"
}

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/ }"
  value="${value//$'\r'/ }"
  printf '%s' "$value"
}

trim_trailing_slash() {
  local value="$1"
  while [[ "$value" == */ ]]; do
    value="${value%/}"
  done
  printf '%s' "$value"
}

normalize_product_release_tag() {
  local value
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  [[ -n "$value" ]] || return 0
  if [[ "$value" != v* ]]; then
    value="v$value"
  fi
  printf '%s' "$value"
}

normalize_beta_release_tag() {
  local value
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  [[ -n "$value" ]] || return 0
  [[ "$value" =~ ^beta-[0-9]{4}\.[0-9]{2}\.[0-9]{2}\.[0-9]+$ ]] || die "beta release tag 不符合规则：$value"
  printf '%s' "$value"
}

normalize_product_version() {
  local value
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  value="${value#v}"
  printf '%s' "$value"
}

normalize_runtime_release_tag() {
  local value
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  [[ -n "$value" ]] || return 0
  if [[ "$value" != runtime-* ]]; then
    value="runtime-$(normalize_product_release_tag "$value")"
  fi
  printf '%s' "$value"
}

normalize_release_scope() {
  local value
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  case "$value" in
    full|desktop|watch|runtime-pointer|docs|compatibility|metadata)
      printf '%s' "$value"
      ;;
    *)
      die "不支持的 release_scope：${1:-}"
      ;;
  esac
}

release_asset_base_url() {
  local repository="$1"
  local tag="$2"
  printf 'https://github.com/%s/releases/download/%s' "$repository" "$tag"
}

resolve_runtime_release_tag() {
  if [[ -n "${OPENWATCHER_RUNTIME_RELEASE_TAG:-}" ]]; then
    normalize_runtime_release_tag "$OPENWATCHER_RUNTIME_RELEASE_TAG"
    return 0
  fi
  if [[ -n "${OPENWATCHER_RUNTIME_RELEASE_VERSION:-}" ]]; then
    normalize_runtime_release_tag "$OPENWATCHER_RUNTIME_RELEASE_VERSION"
    return 0
  fi
  return 0
}

resolve_runtime_manifest_url() {
  local repository="$1"
  local runtime_tag
  if [[ -n "${OPENWATCHER_RUNTIME_MANIFEST_URL:-}" ]]; then
    trim_trailing_slash "$OPENWATCHER_RUNTIME_MANIFEST_URL"
    return 0
  fi
  runtime_tag="$(resolve_runtime_release_tag)"
  if [[ -n "$runtime_tag" ]]; then
    printf '%s/manifest.json' "$(release_asset_base_url "$repository" "$runtime_tag")"
  fi
}

resolve_runtime_asset_base_url() {
  local repository="$1"
  local runtime_tag
  if [[ -n "${OPENWATCHER_RUNTIME_ASSET_BASE_URL:-}" ]]; then
    trim_trailing_slash "$OPENWATCHER_RUNTIME_ASSET_BASE_URL"
    return 0
  fi
  if [[ -n "${OPENWATCHER_RUNTIME_MANIFEST_URL:-}" ]]; then
    printf '%s' "$(trim_trailing_slash "${OPENWATCHER_RUNTIME_MANIFEST_URL%/*}")"
    return 0
  fi
  runtime_tag="$(resolve_runtime_release_tag)"
  if [[ -n "$runtime_tag" ]]; then
    release_asset_base_url "$repository" "$runtime_tag"
  fi
}
