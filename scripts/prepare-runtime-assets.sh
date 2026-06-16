#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/release-common.sh"

RELEASE_SLUG="${1:-${OPENWATCHER_RUNTIME_RELEASE_SLUG:-}}"
[[ -n "$RELEASE_SLUG" ]] || die "用法：scripts/prepare-runtime-assets.sh <release-slug>"

REPOSITORY_NAME="${OPENWATCHER_GITHUB_REPOSITORY:-${GITHUB_REPOSITORY:-openwatcher-ai/openwatcher}}"
WATCH_APK_PATH="${OPENWATCHER_WATCH_APK_PATH:-}"
WATCH_METADATA_PATH="${OPENWATCHER_WATCH_METADATA_PATH:-}"
OUTPUT_DIR="${OPENWATCHER_RUNTIME_OUTPUT_DIR:-$ROOT_DIR/dist/runtime-assets}"
WORK_DIR="${OPENWATCHER_RUNTIME_WORK_DIR:-$ROOT_DIR/.tmp/runtime-assets}"
RUNTIME_TAG="$(resolve_runtime_release_tag)"
RUNTIME_MANIFEST_URL="$(resolve_runtime_manifest_url "$REPOSITORY_NAME")"
RUNTIME_ASSET_BASE_URL="$(resolve_runtime_asset_base_url "$REPOSITORY_NAME")"

case "$OUTPUT_DIR" in
  /*) ;;
  *) OUTPUT_DIR="$ROOT_DIR/$OUTPUT_DIR" ;;
esac
case "$WORK_DIR" in
  /*) ;;
  *) WORK_DIR="$ROOT_DIR/$WORK_DIR" ;;
esac

[[ -f "$WATCH_APK_PATH" ]] || die "缺少 watch APK：$WATCH_APK_PATH"
[[ -f "$WATCH_METADATA_PATH" ]] || die "缺少 watch APK metadata：$WATCH_METADATA_PATH"
[[ -n "$RUNTIME_MANIFEST_URL" ]] || die "缺少 runtime manifest URL，请设置 OPENWATCHER_RUNTIME_MANIFEST_URL 或 OPENWATCHER_RUNTIME_RELEASE_VERSION"
[[ -n "$RUNTIME_ASSET_BASE_URL" ]] || die "缺少 runtime 资产基地址，请设置 OPENWATCHER_RUNTIME_ASSET_BASE_URL、OPENWATCHER_RUNTIME_MANIFEST_URL 或 OPENWATCHER_RUNTIME_RELEASE_VERSION"

require_command curl
require_command unzip
require_command tar
require_command zip
require_command jq

rm -rf "$OUTPUT_DIR" "$WORK_DIR"
mkdir -p "$OUTPUT_DIR" "$WORK_DIR"

runtime_release_url() {
  printf '%s/%s\n' "$(trim_trailing_slash "$RUNTIME_ASSET_BASE_URL")" "$1"
}

copy_with_checksum() {
  local source_path="$1"
  local target_name="$2"
  cp "$source_path" "$OUTPUT_DIR/$target_name"
}

package_windows_cloudflared() {
  local source_path="$1"
  local target_name="$2"
  local temp_dir="$WORK_DIR/${target_name%.zip}"
  rm -rf "$temp_dir"
  mkdir -p "$temp_dir"
  cp "$source_path" "$temp_dir/cloudflared.exe"
  (
    cd "$temp_dir"
    zip -qry "$OUTPUT_DIR/$target_name" cloudflared.exe
  )
}

platform_tools_version() {
  unzip -p "$1" platform-tools/source.properties | sed -nE 's/^Pkg\.Revision=([0-9.]+).*/\1/p' | head -1
}

desktop_version="$(trim_value "${OPENWATCHER_DESKTOP_MIN_VERSION:-${OPENWATCHER_DESKTOP_VERSION:-}}")"
[[ -n "$desktop_version" ]] || die "缺少 Desktop 最低版本，请设置 OPENWATCHER_DESKTOP_MIN_VERSION 或 OPENWATCHER_DESKTOP_VERSION"
validate_semver_version "Desktop 最低版本" "$desktop_version"
source_commit="$(git -C "$ROOT_DIR" rev-parse HEAD)"
generated_at="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

watch_version_name="$(jq -r '.versionName' "$WATCH_METADATA_PATH")"
watch_version_code="$(jq -r '.versionCode' "$WATCH_METADATA_PATH")"
watch_artifact="$(basename "$WATCH_APK_PATH")"
copy_with_checksum "$WATCH_APK_PATH" "$watch_artifact"

note "下载 Android platform-tools"
curl -fsSL "https://dl.google.com/android/repository/platform-tools-latest-windows.zip" -o "$WORK_DIR/platform-tools-windows.zip"
curl -fsSL "https://dl.google.com/android/repository/platform-tools-latest-darwin.zip" -o "$WORK_DIR/platform-tools-darwin.zip"
platform_tools_version_windows="$(platform_tools_version "$WORK_DIR/platform-tools-windows.zip")"
platform_tools_version_darwin="$(platform_tools_version "$WORK_DIR/platform-tools-darwin.zip")"
[[ -n "$platform_tools_version_windows" ]] || die "无法读取 Windows platform-tools 版本"
[[ -n "$platform_tools_version_darwin" ]] || die "无法读取 macOS platform-tools 版本"

copy_with_checksum "$WORK_DIR/platform-tools-windows.zip" "platform-tools-windows-amd64.zip"
copy_with_checksum "$WORK_DIR/platform-tools-windows.zip" "platform-tools-windows-arm64.zip"
copy_with_checksum "$WORK_DIR/platform-tools-darwin.zip" "platform-tools-darwin-amd64.zip"
copy_with_checksum "$WORK_DIR/platform-tools-darwin.zip" "platform-tools-darwin-arm64.zip"

note "下载 cloudflared 最新 release 元数据"
curl -fsSL "https://api.github.com/repos/cloudflare/cloudflared/releases/latest" -o "$WORK_DIR/cloudflared-release.json"
cloudflared_version="$(jq -r '.tag_name' "$WORK_DIR/cloudflared-release.json")"
[[ -n "$cloudflared_version" && "$cloudflared_version" != "null" ]] || die "无法读取 cloudflared 最新版本"

cloudflared_asset_url() {
  local asset_name="$1"
  jq -r --arg asset_name "$asset_name" '.assets[] | select(.name == $asset_name) | .browser_download_url' "$WORK_DIR/cloudflared-release.json" | head -1
}

cloudflared_windows_amd64_url="$(cloudflared_asset_url "cloudflared-windows-amd64.exe")"
cloudflared_darwin_amd64_url="$(cloudflared_asset_url "cloudflared-darwin-amd64.tgz")"
cloudflared_darwin_arm64_url="$(cloudflared_asset_url "cloudflared-darwin-arm64.tgz")"
[[ -n "$cloudflared_windows_amd64_url" && "$cloudflared_windows_amd64_url" != "null" ]] || die "cloudflared release 缺少 windows-amd64.exe"
[[ -n "$cloudflared_darwin_amd64_url" && "$cloudflared_darwin_amd64_url" != "null" ]] || die "cloudflared release 缺少 darwin-amd64.tgz"
[[ -n "$cloudflared_darwin_arm64_url" && "$cloudflared_darwin_arm64_url" != "null" ]] || die "cloudflared release 缺少 darwin-arm64.tgz"

note "下载 cloudflared 产物"
curl -fsSL "$cloudflared_windows_amd64_url" -o "$WORK_DIR/cloudflared-windows-amd64.exe"
curl -fsSL "$cloudflared_darwin_amd64_url" -o "$OUTPUT_DIR/cloudflared-darwin-amd64.tgz"
curl -fsSL "$cloudflared_darwin_arm64_url" -o "$OUTPUT_DIR/cloudflared-darwin-arm64.tgz"
package_windows_cloudflared "$WORK_DIR/cloudflared-windows-amd64.exe" "cloudflared-windows-amd64.zip"
package_windows_cloudflared "$WORK_DIR/cloudflared-windows-amd64.exe" "cloudflared-windows-arm64.zip"

manifest_path="$OUTPUT_DIR/runtime-manifest.json"
cat > "$manifest_path" <<JSON
{
  "schemaVersion": 1,
  "channel": "release",
  "generatedAt": "$generated_at",
  "releaseSlug": "$RELEASE_SLUG",
  "desktopMinVersion": "$desktop_version",
  "sourceCommit": "$source_commit",
  "resources": {
    "watchApk": {
      "version": "${watch_version_name}+${watch_version_code}",
      "versionName": "$watch_version_name",
      "versionCode": ${watch_version_code},
      "artifact": "$watch_artifact",
      "url": "$(runtime_release_url "$watch_artifact")",
      "sha256": "$(sha256_file "$OUTPUT_DIR/$watch_artifact")",
      "sizeBytes": $(file_size_bytes "$OUTPUT_DIR/$watch_artifact")
    },
    "platformTools": {
      "windows-amd64": {
        "version": "$platform_tools_version_windows",
        "artifact": "platform-tools-windows-amd64.zip",
        "url": "$(runtime_release_url "platform-tools-windows-amd64.zip")",
        "sha256": "$(sha256_file "$OUTPUT_DIR/platform-tools-windows-amd64.zip")",
        "sizeBytes": $(file_size_bytes "$OUTPUT_DIR/platform-tools-windows-amd64.zip"),
        "archiveKind": "zip",
        "binRelativePath": "platform-tools/adb.exe",
        "extraFiles": [
          "platform-tools/AdbWinApi.dll",
          "platform-tools/AdbWinUsbApi.dll"
        ]
      },
      "windows-arm64": {
        "version": "$platform_tools_version_windows",
        "artifact": "platform-tools-windows-arm64.zip",
        "url": "$(runtime_release_url "platform-tools-windows-arm64.zip")",
        "sha256": "$(sha256_file "$OUTPUT_DIR/platform-tools-windows-arm64.zip")",
        "sizeBytes": $(file_size_bytes "$OUTPUT_DIR/platform-tools-windows-arm64.zip"),
        "archiveKind": "zip",
        "binRelativePath": "platform-tools/adb.exe",
        "extraFiles": [
          "platform-tools/AdbWinApi.dll",
          "platform-tools/AdbWinUsbApi.dll"
        ]
      },
      "darwin-amd64": {
        "version": "$platform_tools_version_darwin",
        "artifact": "platform-tools-darwin-amd64.zip",
        "url": "$(runtime_release_url "platform-tools-darwin-amd64.zip")",
        "sha256": "$(sha256_file "$OUTPUT_DIR/platform-tools-darwin-amd64.zip")",
        "sizeBytes": $(file_size_bytes "$OUTPUT_DIR/platform-tools-darwin-amd64.zip"),
        "archiveKind": "zip",
        "binRelativePath": "platform-tools/adb"
      },
      "darwin-arm64": {
        "version": "$platform_tools_version_darwin",
        "artifact": "platform-tools-darwin-arm64.zip",
        "url": "$(runtime_release_url "platform-tools-darwin-arm64.zip")",
        "sha256": "$(sha256_file "$OUTPUT_DIR/platform-tools-darwin-arm64.zip")",
        "sizeBytes": $(file_size_bytes "$OUTPUT_DIR/platform-tools-darwin-arm64.zip"),
        "archiveKind": "zip",
        "binRelativePath": "platform-tools/adb"
      }
    },
    "cloudflared": {
      "windows-amd64": {
        "version": "$cloudflared_version",
        "artifact": "cloudflared-windows-amd64.zip",
        "url": "$(runtime_release_url "cloudflared-windows-amd64.zip")",
        "sha256": "$(sha256_file "$OUTPUT_DIR/cloudflared-windows-amd64.zip")",
        "sizeBytes": $(file_size_bytes "$OUTPUT_DIR/cloudflared-windows-amd64.zip"),
        "archiveKind": "zip",
        "binRelativePath": "cloudflared.exe"
      },
      "windows-arm64": {
        "version": "$cloudflared_version",
        "artifact": "cloudflared-windows-arm64.zip",
        "url": "$(runtime_release_url "cloudflared-windows-arm64.zip")",
        "sha256": "$(sha256_file "$OUTPUT_DIR/cloudflared-windows-arm64.zip")",
        "sizeBytes": $(file_size_bytes "$OUTPUT_DIR/cloudflared-windows-arm64.zip"),
        "archiveKind": "zip",
        "binRelativePath": "cloudflared.exe"
      },
      "darwin-amd64": {
        "version": "$cloudflared_version",
        "artifact": "cloudflared-darwin-amd64.tgz",
        "url": "$(runtime_release_url "cloudflared-darwin-amd64.tgz")",
        "sha256": "$(sha256_file "$OUTPUT_DIR/cloudflared-darwin-amd64.tgz")",
        "sizeBytes": $(file_size_bytes "$OUTPUT_DIR/cloudflared-darwin-amd64.tgz"),
        "archiveKind": "tgz",
        "binRelativePath": "cloudflared"
      },
      "darwin-arm64": {
        "version": "$cloudflared_version",
        "artifact": "cloudflared-darwin-arm64.tgz",
        "url": "$(runtime_release_url "cloudflared-darwin-arm64.tgz")",
        "sha256": "$(sha256_file "$OUTPUT_DIR/cloudflared-darwin-arm64.tgz")",
        "sizeBytes": $(file_size_bytes "$OUTPUT_DIR/cloudflared-darwin-arm64.tgz"),
        "archiveKind": "tgz",
        "binRelativePath": "cloudflared"
      }
    }
  },
  "notes": "versioned runtime release for ${RELEASE_SLUG}; Windows ARM64 继续使用上游 amd64 cloudflared 产物封装 zip。"
}
JSON

printf '%s  %s\n' "$(sha256_file "$manifest_path")" "runtime-manifest.json" > "$OUTPUT_DIR/runtime-manifest.sha256"

{
  cd "$OUTPUT_DIR"
  find . -maxdepth 1 -type f ! -name 'runtime-checksums.txt' -print0 \
    | sort -z \
    | while IFS= read -r -d '' file; do
        sha256_file "${file#./}" | awk -v name="${file#./}" '{printf "%s  %s\n", $1, name}'
      done > runtime-checksums.txt
}

jq -n \
  --arg generated_at "$generated_at" \
  --arg release_slug "$RELEASE_SLUG" \
  --arg runtime_tag "$RUNTIME_TAG" \
  --arg repository "$REPOSITORY_NAME" \
  --arg source_commit "$source_commit" \
  --arg manifest_url "$RUNTIME_MANIFEST_URL" \
  --arg manifest_sha256 "$(sha256_file "$manifest_path")" \
  '{
    generatedAt: $generated_at,
    releaseSlug: $release_slug,
    runtimeTag: $runtime_tag,
    repository: $repository,
    sourceCommit: $source_commit,
    manifestUrl: $manifest_url,
    manifestSha256: $manifest_sha256
  }' > "$OUTPUT_DIR/runtime-assets.json"

note "runtime 资源已准备完成：$OUTPUT_DIR"
