#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/release-common.sh"

WORK_DIR="${OPENWATCHER_DESKTOP_PREPARE_TMP:-$ROOT_DIR/.tmp/desktop-bundled-deps}"
BUNDLED_DIR="$ROOT_DIR/desktop-app/bundled"
PLATFORMS=("$@")
if [[ ${#PLATFORMS[@]} -eq 0 ]]; then
  PLATFORMS=(windows-amd64 windows-arm64 darwin-amd64 darwin-arm64)
fi

require_command curl
require_command unzip
require_command tar

rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR"

platform_tools_url() {
  case "$1" in
    windows)
      printf '%s\n' "${OPENWATCHER_PLATFORM_TOOLS_WINDOWS_URL:-https://dl.google.com/android/repository/platform-tools-latest-windows.zip}"
      ;;
    darwin)
      printf '%s\n' "${OPENWATCHER_PLATFORM_TOOLS_DARWIN_URL:-https://dl.google.com/android/repository/platform-tools-latest-darwin.zip}"
      ;;
    *)
      die "不支持的 platform-tools 系统：$1"
      ;;
  esac
}

cloudflared_url() {
  case "$1" in
    windows-amd64)
      printf '%s\n' "${OPENWATCHER_CLOUDFLARED_WINDOWS_AMD64_URL:-https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe}"
      ;;
    windows-arm64)
      printf '%s\n' "${OPENWATCHER_CLOUDFLARED_WINDOWS_ARM64_URL:-https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe}"
      ;;
    darwin-amd64)
      printf '%s\n' "${OPENWATCHER_CLOUDFLARED_DARWIN_AMD64_URL:-https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-amd64.tgz}"
      ;;
    darwin-arm64)
      printf '%s\n' "${OPENWATCHER_CLOUDFLARED_DARWIN_ARM64_URL:-https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-arm64.tgz}"
      ;;
    *)
      die "不支持的 cloudflared 平台：$1"
      ;;
  esac
}

prepare_platform_tools() {
  local os_name="$1"
  local platform="$2"
  local target_dir="$BUNDLED_DIR/platform-tools/$platform"
  local zip_path="$WORK_DIR/platform-tools-$os_name.zip"
  local extract_dir="$WORK_DIR/platform-tools-$os_name"

  if [[ ! -f "$zip_path" ]]; then
    note "下载 Android platform-tools：$os_name"
    curl -fsSL "$(platform_tools_url "$os_name")" -o "$zip_path"
    mkdir -p "$extract_dir"
    unzip -q "$zip_path" -d "$extract_dir"
  fi

  rm -rf "$target_dir"
  mkdir -p "$target_dir"
  cp -R "$extract_dir/platform-tools/." "$target_dir/"
  if [[ "$os_name" == "windows" ]]; then
    for name in adb.exe AdbWinApi.dll AdbWinUsbApi.dll; do
      [[ -f "$target_dir/$name" ]] || die "platform-tools Windows 包缺少：$name"
    done
  else
    [[ -f "$target_dir/adb" ]] || die "platform-tools macOS 包缺少：adb"
    chmod +x "$target_dir/adb" 2>/dev/null || true
  fi
}

prepare_cloudflared() {
  local platform="$1"
  local target_dir="$BUNDLED_DIR/cloudflared/$platform"
  local url
  url="$(cloudflared_url "$platform")"
  rm -rf "$target_dir"
  mkdir -p "$target_dir"

  case "$platform" in
    windows-amd64)
      note "下载 cloudflared：$platform"
      curl -fsSL "$url" -o "$target_dir/cloudflared.exe"
      ;;
    darwin-*)
      local tgz_path="$WORK_DIR/cloudflared-$platform.tgz"
      local extract_dir="$WORK_DIR/cloudflared-$platform"
      note "下载 cloudflared：$platform"
      curl -fsSL "$url" -o "$tgz_path"
      mkdir -p "$extract_dir"
      tar -xzf "$tgz_path" -C "$extract_dir"
      local binary_path
      binary_path="$(find "$extract_dir" -type f -name 'cloudflared' | head -1)"
      [[ -n "$binary_path" ]] || die "cloudflared 包缺少可执行文件：$platform"
      cp "$binary_path" "$target_dir/cloudflared"
      chmod +x "$target_dir/cloudflared"
      ;;
  esac
}

for platform in "${PLATFORMS[@]}"; do
  read -r os_name _arch <<<"$(printf '%s\n' "$platform" | tr '-' ' ')"
  case "$platform" in
    windows-amd64|windows-arm64|darwin-amd64|darwin-arm64) ;;
    *) die "不支持的 Desktop bundled 平台：$platform" ;;
  esac
  prepare_platform_tools "$os_name" "$platform"
  prepare_cloudflared "$platform"
done

note "Desktop bundled 依赖已准备完成：$BUNDLED_DIR"
