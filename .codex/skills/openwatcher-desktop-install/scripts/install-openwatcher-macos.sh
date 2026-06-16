#!/usr/bin/env bash
set -euo pipefail

CHANNEL_URL="https://openwatcher.ai/channels/beta.json"
INSTALL_DIR="/Applications"
DRY_RUN=0

usage() {
  cat <<'USAGE'
用法：install-openwatcher-macos.sh [选项]

选项：
  --channel-url URL   使用指定 OpenWatcher channel manifest
  --install-dir DIR   安装目录，默认 /Applications
  --dry-run           只解析和打印计划，不下载或安装
  --help              显示帮助
USAGE
}

log() {
  printf '[openwatcher-install] %s\n' "$*"
}

die() {
  printf '[openwatcher-install] 错误：%s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "缺少命令：$1"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --channel-url)
      CHANNEL_URL="${2:-}"
      [[ -n "$CHANNEL_URL" ]] || die "--channel-url 需要 URL"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="${2:-}"
      [[ -n "$INSTALL_DIR" ]] || die "--install-dir 需要目录"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die "未知参数：$1"
      ;;
  esac
done

[[ "$(uname -s)" == "Darwin" ]] || die "本脚本只能在 macOS 上运行"
require_command curl
require_command python3
require_command shasum

arch="$(uname -m)"
case "$arch" in
  arm64) platform="darwin-arm64" ;;
  x86_64) platform="darwin-amd64" ;;
  *) die "不支持的 macOS 架构：$arch" ;;
esac

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/openwatcher-install.XXXXXX")"
mount_dir=""

cleanup() {
  if [[ -n "$mount_dir" && -d "$mount_dir" ]]; then
    hdiutil detach "$mount_dir" >/dev/null 2>&1 || true
  fi
  rm -rf "$work_dir"
}
trap cleanup EXIT

manifest_path="$work_dir/channel-beta.json"
log "读取 channel manifest：$CHANNEL_URL"
curl -fsSL "$CHANNEL_URL" -o "$manifest_path"

asset_tsv="$(
  python3 - "$manifest_path" "$platform" <<'PY'
import json
import sys

manifest_path, platform = sys.argv[1], sys.argv[2]
with open(manifest_path, "r", encoding="utf-8") as handle:
    manifest = json.load(handle)

desktop = manifest.get("desktop") or {}
platforms = desktop.get("platforms") or {}
asset = platforms.get(platform)
if not asset:
    available = ", ".join(sorted(platforms.keys())) or "无"
    raise SystemExit(f"manifest 缺少平台 {platform}，可用平台：{available}")

fields = [
    asset.get("artifact") or "",
    asset.get("downloadUrl") or "",
    asset.get("sha256") or "",
    desktop.get("version") or "",
    (manifest.get("release") or {}).get("tag") or "",
]
if not fields[1]:
    raise SystemExit(f"平台 {platform} 缺少 downloadUrl")
if not fields[2]:
    raise SystemExit(f"平台 {platform} 缺少 sha256")
print("\t".join(fields))
PY
)"

IFS=$'\t' read -r artifact download_url expected_sha desktop_version release_tag <<<"$asset_tsv"
[[ -n "$artifact" ]] || artifact="$(basename "${download_url%%\?*}")"
[[ -n "$artifact" ]] || die "无法确定安装包文件名"

target_app="$INSTALL_DIR/OpenWatcher.app"
log "平台：$platform"
log "Desktop 版本：${desktop_version:-未知}"
log "Release：${release_tag:-未知}"
log "安装包：$artifact"
log "目标路径：$target_app"

if [[ "$DRY_RUN" == "1" ]]; then
  log "dry-run：不会下载、安装或启动应用"
  exit 0
fi

require_command ditto
require_command open
mkdir -p "$INSTALL_DIR"

download_path="$work_dir/$artifact"
log "下载安装包"
curl -fL "$download_url" -o "$download_path"

actual_sha="$(shasum -a 256 "$download_path" | awk '{print tolower($1)}')"
expected_sha="$(printf '%s' "$expected_sha" | tr '[:upper:]' '[:lower:]')"
[[ "$actual_sha" == "$expected_sha" ]] || die "SHA-256 校验失败：期望 $expected_sha，实际 $actual_sha"
log "SHA-256 校验通过"

extract_dir="$work_dir/extract"
mkdir -p "$extract_dir"

case "$artifact" in
  *.dmg)
    require_command hdiutil
    mount_dir="$work_dir/mount"
    mkdir -p "$mount_dir"
    hdiutil attach -nobrowse -readonly -mountpoint "$mount_dir" "$download_path" >/dev/null
    app_source="$(find "$mount_dir" -maxdepth 3 -name 'OpenWatcher.app' -type d -print -quit)"
    ;;
  *.zip)
    ditto -x -k "$download_path" "$extract_dir"
    app_source="$(find "$extract_dir" -maxdepth 4 -name 'OpenWatcher.app' -type d -print -quit)"
    ;;
  *)
    die "不支持的 macOS 安装包类型：$artifact"
    ;;
esac

[[ -n "${app_source:-}" && -d "$app_source" ]] || die "安装包中未找到 OpenWatcher.app"

rm -rf "$target_app"
ditto "$app_source" "$target_app"
log "已安装到 $target_app"

if command -v xattr >/dev/null 2>&1; then
  xattr -dr com.apple.quarantine "$target_app" 2>/dev/null || true
  log "已移除 OpenWatcher.app 的 quarantine 属性"
fi

open "$target_app"
log "已启动 OpenWatcher Desktop"
