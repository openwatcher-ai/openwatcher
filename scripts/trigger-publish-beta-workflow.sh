#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

RELEASE_SCOPE="${OPENWATCHER_RELEASE_SCOPE:-${1:-}}"
RELEASE_SUMMARY="${OPENWATCHER_RELEASE_SUMMARY:-${2:-}}"
REF_NAME="${OPENWATCHER_WORKFLOW_REF:-main}"

case "$RELEASE_SCOPE" in
  full|desktop|watch|runtime-pointer|docs|compatibility|metadata) ;;
  *)
    echo "用法: $0 <full|desktop|watch|runtime-pointer|docs|compatibility|metadata> <release-summary>" >&2
    exit 2
    ;;
esac

[[ -n "$RELEASE_SUMMARY" ]] || {
  echo "缺少 release summary" >&2
  exit 2
}

echo "触发 OpenWatcher Publish Beta"
echo "  ref: $REF_NAME"
echo "  release_scope: $RELEASE_SCOPE"
echo "  release_summary: $RELEASE_SUMMARY"

gh workflow run "OpenWatcher Publish Beta" \
  --ref "$REF_NAME" \
  -f release_scope="$RELEASE_SCOPE" \
  -f release_summary="$RELEASE_SUMMARY"

echo "已触发。可用以下命令查看最近运行："
echo "  gh run list --workflow 'OpenWatcher Publish Beta' --limit 5"
