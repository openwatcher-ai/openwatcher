#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/release-common.sh"

PLATFORM="${1:-$(current_platform_id)}"
read -r GOOS_VALUE GOARCH_VALUE <<<"$(platform_go_values "$PLATFORM")"

ensure_release_dir
require_command go

OUTPUT_NAME="openwatcher-${PLATFORM}"
if [[ "$GOOS_VALUE" == "windows" ]]; then
  OUTPUT_NAME+=".exe"
fi
OUTPUT_PATH="$RELEASE_DIR/$OUTPUT_NAME"
BACKEND_BUILD_VERSION="$(trim_value "${OPENWATCHER_BACKEND_VERSION:-${OPENWATCHER_DESKTOP_VERSION:-}}")"
if [[ -z "$BACKEND_BUILD_VERSION" ]]; then
  BACKEND_BUILD_VERSION="$(dev_build_version)"
fi

note "构建后端发布产物：$OUTPUT_NAME"
(
  cd "$ROOT_DIR"
  GOOS="$GOOS_VALUE" GOARCH="$GOARCH_VALUE" \
    go build -trimpath \
      -ldflags "-X openwatcher/internal/buildinfo.Version=$BACKEND_BUILD_VERSION -X openwatcher/internal/buildinfo.Commit=$CURRENT_COMMIT -X openwatcher/internal/buildinfo.BuiltAt=$BUILT_AT_UTC" \
      -o "$OUTPUT_PATH" \
      ./cmd/openwatcher
)

chmod +x "$OUTPUT_PATH" 2>/dev/null || true
note "已生成：$OUTPUT_PATH"
printf '%s\n' "$OUTPUT_PATH"
