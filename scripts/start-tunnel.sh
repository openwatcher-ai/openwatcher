#!/usr/bin/env bash
set -euo pipefail

TUNNEL_NAME="${OPENWATCHER_TUNNEL_NAME:-${1:-}}"
TUNNEL_ID="${OPENWATCHER_TUNNEL_ID:-}"
CREDENTIALS_FILE="${OPENWATCHER_CREDENTIALS_FILE:-}"
HOSTNAME="${OPENWATCHER_TUNNEL_HOSTNAME:-${2:-}}"
ORIGIN_URL="${OPENWATCHER_ORIGIN_URL:-http://127.0.0.1:8787}"

if [[ -z "$HOSTNAME" ]]; then
  echo "缺少 OpenWatcher tunnel hostname。设置 OPENWATCHER_TUNNEL_HOSTNAME 或传入第二个参数；如全局 cloudflared config 指向其他 tunnel，请改用 OPENWATCHER_TUNNEL_ID 和 OPENWATCHER_CREDENTIALS_FILE。" >&2
  exit 2
fi

if [[ -n "$TUNNEL_ID" && -n "$CREDENTIALS_FILE" ]]; then
  CONFIG_FILE="$(mktemp "${TMPDIR:-/tmp}/openwatcher-cloudflared.XXXXXX")"
  trap 'rm -f "$CONFIG_FILE"' EXIT
  cat > "$CONFIG_FILE" <<EOF
tunnel: $TUNNEL_ID
credentials-file: $CREDENTIALS_FILE
ingress:
  - hostname: $HOSTNAME
    service: $ORIGIN_URL
  - service: http_status:404
EOF
  exec cloudflared tunnel --config "$CONFIG_FILE" run
fi

if [[ -z "$TUNNEL_NAME" ]]; then
  echo "缺少 OpenWatcher tunnel 名称。设置 OPENWATCHER_TUNNEL_NAME 或传入第一个参数；如全局 cloudflared config 指向其他 tunnel，请改用 OPENWATCHER_TUNNEL_ID 和 OPENWATCHER_CREDENTIALS_FILE。" >&2
  exit 2
fi

exec cloudflared tunnel run --url "$ORIGIN_URL" "$TUNNEL_NAME"
