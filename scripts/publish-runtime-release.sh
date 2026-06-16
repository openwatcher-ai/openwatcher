#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/release-common.sh"

REPOSITORY_NAME="${OPENWATCHER_GITHUB_REPOSITORY:-${GITHUB_REPOSITORY:-openwatcher-ai/openwatcher}}"
RUNTIME_TAG="$(resolve_runtime_release_tag)"
RELEASE_SLUG="${OPENWATCHER_RUNTIME_RELEASE_SLUG:-}"
RELEASE_SUMMARY="${OPENWATCHER_RUNTIME_RELEASE_SUMMARY:-}"
RUNTIME_DIR="${OPENWATCHER_RUNTIME_OUTPUT_DIR:-$ROOT_DIR/dist/runtime-assets}"

[[ -n "$RUNTIME_TAG" ]] || die "缺少 OPENWATCHER_RUNTIME_RELEASE_TAG 或 OPENWATCHER_RUNTIME_RELEASE_VERSION"
[[ -n "$RELEASE_SLUG" ]] || die "缺少 OPENWATCHER_RUNTIME_RELEASE_SLUG"
[[ -n "$RELEASE_SUMMARY" ]] || die "缺少 OPENWATCHER_RUNTIME_RELEASE_SUMMARY"
[[ -d "$RUNTIME_DIR" ]] || die "runtime 产物目录不存在：$RUNTIME_DIR"

require_command gh
require_command jq

ensure_release() {
  local tag="$1"
  local title="$2"
  local notes="$3"
  if gh release view "$tag" >/dev/null 2>&1; then
    gh release edit "$tag" --title "$title" --notes "$notes" >/dev/null
  else
    gh release create "$tag" --title "$title" --notes "$notes" >/dev/null
  fi
}

runtime_notes=$(
  cat <<EOF
OpenWatcher runtime release

- repository: $REPOSITORY_NAME
- tag: $RUNTIME_TAG
- slug: $RELEASE_SLUG
- commit: $(git -C "$ROOT_DIR" rev-parse --short HEAD)
- summary: $RELEASE_SUMMARY
EOF
)

[[ -f "$RUNTIME_DIR/runtime-manifest.json" ]] || die "缺少 runtime-manifest.json：$RUNTIME_DIR"
[[ -f "$RUNTIME_DIR/runtime-checksums.txt" ]] || die "缺少 runtime-checksums.txt：$RUNTIME_DIR"

ensure_release "$RUNTIME_TAG" "OpenWatcher Runtime $RUNTIME_TAG" "$runtime_notes"
find "$RUNTIME_DIR" -maxdepth 1 -type f -print0 | xargs -0 gh release upload "$RUNTIME_TAG" --clobber
note "runtime release 已发布：$RUNTIME_TAG"
