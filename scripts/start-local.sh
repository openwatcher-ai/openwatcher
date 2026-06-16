#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

LISTEN="${OPENWATCHER_LISTEN:-127.0.0.1:18787}"
CONFIG_PATH="${OPENWATCHER_CONFIG:-$HOME/.openwatcher/config.json}"
PAIRING_SLOT="${OPENWATCHER_PAIRING_SLOT:-dev}"
PUBLIC_BASE_URL="${OPENWATCHER_PUBLIC_BASE_URL:-}"
NO_AUTH="${OPENWATCHER_NO_AUTH:-0}"
GO_BIN="${OPENWATCHER_GO_BIN:-go}"
mkdir -p "$ROOT_DIR/bin"

LISTEN_HOST="${LISTEN%:*}"
LISTEN_PORT="${LISTEN##*:}"
if [[ -z "$PUBLIC_BASE_URL" ]]; then
  case "$LISTEN_HOST" in
    127.0.0.1|localhost|::1|\[::1\])
      PUBLIC_BASE_URL="http://10.0.2.2:${LISTEN_PORT}"
      ;;
    *)
      PUBLIC_BASE_URL="http://${LISTEN_HOST}:${LISTEN_PORT}"
      ;;
  esac
fi

COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo dev)"
BUILT_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

"$GO_BIN" build -trimpath \
  -ldflags "-X openwatcher/internal/buildinfo.Commit=$COMMIT -X openwatcher/internal/buildinfo.BuiltAt=$BUILT_AT" \
  -o "$ROOT_DIR/bin/openwatcher" ./cmd/openwatcher
echo "OpenWatcher 开发服务监听 ${LISTEN}，对手表暴露 ${PUBLIC_BASE_URL}，配置文件 ${CONFIG_PATH}，配对槽位 ${PAIRING_SLOT}" >&2

if [[ "$NO_AUTH" == "1" ]]; then
  echo "OpenWatcher 开发服务已启用 no-auth，仅用于本地接口调试" >&2
fi

if [[ "$NO_AUTH" == "1" ]]; then
  exec env OPENWATCHER_CONFIG="${CONFIG_PATH}" "$ROOT_DIR/bin/openwatcher" --listen "${LISTEN}" --public-base-url "${PUBLIC_BASE_URL}" --pairing-slot "${PAIRING_SLOT}" --no-auth "$@"
fi

exec env OPENWATCHER_CONFIG="${CONFIG_PATH}" "$ROOT_DIR/bin/openwatcher" --listen "${LISTEN}" --public-base-url "${PUBLIC_BASE_URL}" --pairing-slot "${PAIRING_SLOT}" "$@"
