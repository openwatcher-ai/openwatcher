#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/scripts/release-common.sh"

RELEASE_DIR="${OPENWATCHER_PUBLIC_RELEASE_DIR:-$ROOT_DIR/dist/public-release}"
RELEASE_MANIFEST_PATH="${OPENWATCHER_RELEASE_MANIFEST_PATH:-$RELEASE_DIR/release-manifest.json}"
CHANGELOG_ENTRY_PATH="${OPENWATCHER_CHANGELOG_ENTRY_PATH:-$RELEASE_DIR/changelog-entry.json}"
NOTES_PATH="${OPENWATCHER_RELEASE_NOTES_PATH:-$RELEASE_DIR/release-notes.md}"

require_command jq
[[ -f "$RELEASE_MANIFEST_PATH" ]] || die "缺少 release-manifest.json：$RELEASE_MANIFEST_PATH"
[[ -f "$CHANGELOG_ENTRY_PATH" ]] || die "缺少 changelog-entry.json：$CHANGELOG_ENTRY_PATH"

status_label() {
  case "$1" in
    updated)
      case "${2:-}" in
        runtime) printf '更新' ;;
        *) printf '更新' ;;
      esac
      ;;
    reused)
      case "${2:-}" in
        runtime) printf '复用当前 Runtime Release' ;;
        *) printf '复用上一版' ;;
      esac
      ;;
    not_included) printf '未包含' ;;
    *) printf '%s' "$1" ;;
  esac
}

release_tag="$(jq -r '.release.tag' "$RELEASE_MANIFEST_PATH")"
release_summary="$(jq -r '.release.summary // empty' "$RELEASE_MANIFEST_PATH")"
runtime_tag="$(jq -r '.runtime.releaseTag // empty' "$RELEASE_MANIFEST_PATH")"
desktop_status="$(jq -r '.components.desktop.status' "$RELEASE_MANIFEST_PATH")"
watch_status="$(jq -r '.components.watch.status' "$RELEASE_MANIFEST_PATH")"
runtime_status="$(jq -r '.components.runtime.status' "$RELEASE_MANIFEST_PATH")"
compatibility_status="$(jq -r '.components.compatibility.status' "$RELEASE_MANIFEST_PATH")"
docs_status="$(jq -r '.components.docs.status' "$RELEASE_MANIFEST_PATH")"

render_notes_section() {
  local key="$1"
  local title="$2"
  local count
  count="$(jq ".notes.$key | length" "$CHANGELOG_ENTRY_PATH")"
  if [[ "$count" -gt 0 ]]; then
    echo "## $title"
    echo
    jq -r ".notes.$key[] | \"- 【\\(.component)】\\(.text)\"" "$CHANGELOG_ENTRY_PATH"
    echo
  fi
}

{
  echo "# OpenWatcher Beta ${release_tag}"
  echo
  if [[ -n "$release_summary" ]]; then
    echo "$release_summary"
    echo
  fi
  echo "## 发布范围"
  echo
  echo "- 【桌面应用】$(status_label "$desktop_status" desktop)"
  echo "- 【手表应用】$(status_label "$watch_status" watch)"
  echo "- 【运行时依赖】$(status_label "$runtime_status" runtime)"
  echo "- 【兼容性】$(status_label "$compatibility_status" compatibility)"
  echo "- 【文档】$(status_label "$docs_status" docs)"
  echo

  render_notes_section features "新增功能"
  render_notes_section improvements "功能优化"
  render_notes_section fixes "问题修复"
  render_notes_section compatibility "兼容性与升级说明"

  echo "## 产物与校验"
  echo
  echo "- Product Release: \`${release_tag}\`"
  echo "- Runtime Release: \`${runtime_tag}\`"
  echo "- Release manifest: \`release-manifest.json\`"
  echo "- Changelog entry: \`changelog-entry.json\`"
  echo "- Checksums: \`checksums.txt\`"
} >"$NOTES_PATH"

note "已生成：$NOTES_PATH"
