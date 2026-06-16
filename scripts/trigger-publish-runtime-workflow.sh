#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

RUNTIME_VERSION="${OPENWATCHER_RUNTIME_VERSION:-${1:-}}"
DESKTOP_MIN_VERSION="${OPENWATCHER_DESKTOP_MIN_VERSION:-${2:-}}"
WATCH_VERSION_NAME="${OPENWATCHER_WATCH_VERSION_NAME:-${3:-}}"
WATCH_VERSION_CODE="${OPENWATCHER_WATCH_VERSION_CODE:-${4:-}}"
PLATFORM_TOOLS_VERSION="${OPENWATCHER_PLATFORM_TOOLS_VERSION:-${5:-}}"
CLOUDFLARED_VERSION="${OPENWATCHER_CLOUDFLARED_VERSION:-${6:-}}"
REF_NAME="${OPENWATCHER_WORKFLOW_REF:-main}"

if [[ -z "$RUNTIME_VERSION" || -z "$DESKTOP_MIN_VERSION" || -z "$WATCH_VERSION_NAME" || -z "$WATCH_VERSION_CODE" || -z "$PLATFORM_TOOLS_VERSION" || -z "$CLOUDFLARED_VERSION" ]]; then
  echo "用法: $0 <runtime-version> <desktop-min-version> <watch-version-name> <watch-version-code> <platform-tools-version> <cloudflared-version>" >&2
  exit 2
fi

echo "触发 OpenWatcher Publish Runtime"
echo "  ref: $REF_NAME"
echo "  runtime_version: $RUNTIME_VERSION"
echo "  desktop_min_version: $DESKTOP_MIN_VERSION"
echo "  watch_version_name: $WATCH_VERSION_NAME"
echo "  watch_version_code: $WATCH_VERSION_CODE"
echo "  platform_tools_version: $PLATFORM_TOOLS_VERSION"
echo "  cloudflared_version: $CLOUDFLARED_VERSION"

gh workflow run "OpenWatcher Publish Runtime" \
  --ref "$REF_NAME" \
  -f runtime_version="$RUNTIME_VERSION" \
  -f desktop_min_version="$DESKTOP_MIN_VERSION" \
  -f watch_version_name="$WATCH_VERSION_NAME" \
  -f watch_version_code="$WATCH_VERSION_CODE" \
  -f platform_tools_version="$PLATFORM_TOOLS_VERSION" \
  -f cloudflared_version="$CLOUDFLARED_VERSION"

echo "已触发。可用以下命令查看最近运行："
echo "  gh run list --workflow 'OpenWatcher Publish Runtime' --limit 5"
