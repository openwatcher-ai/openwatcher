#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="${OPENWATCHER_PUBLIC_RELEASE_DIR:-$ROOT_DIR/dist/public-release}"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/release-common.sh"

SCAN_DIR="${1:-$RELEASE_DIR}"
[[ -d "$SCAN_DIR" ]] || die "发布目录不存在：$SCAN_DIR"

require_command jq
require_command rg

CHECKSUMS_PATH="$SCAN_DIR/checksums.txt"
MANIFEST_PATH="$SCAN_DIR/release-manifest.json"
CHANGELOG_ENTRY_PATH="$SCAN_DIR/changelog-entry.json"
RELEASE_NOTES_PATH="$SCAN_DIR/release-notes.md"
NOTICES_PATH="$SCAN_DIR/THIRD_PARTY_NOTICES.md"
RUNTIME_MANIFEST_PATH="${OPENWATCHER_RUNTIME_MANIFEST_PATH:-}"

[[ -f "$CHECKSUMS_PATH" ]] || die "缺少 checksums.txt：$CHECKSUMS_PATH"
[[ -f "$MANIFEST_PATH" ]] || die "缺少 release-manifest.json：$MANIFEST_PATH"
[[ -f "$CHANGELOG_ENTRY_PATH" ]] || die "缺少 changelog-entry.json：$CHANGELOG_ENTRY_PATH"
[[ -f "$RELEASE_NOTES_PATH" ]] || die "缺少 release-notes.md：$RELEASE_NOTES_PATH"
[[ -f "$NOTICES_PATH" ]] || die "缺少 THIRD_PARTY_NOTICES.md：$NOTICES_PATH"

checksum_lookup() {
  awk -v target="$1" '$2 == target {print $1}' "$CHECKSUMS_PATH" | head -1
}

require_manifest_string() {
  local expr="$1"
  local label="$2"
  local value
  value="$(jq -er "$expr | strings | select(length > 0)" "$MANIFEST_PATH" 2>/dev/null || true)"
  [[ -n "$value" ]] || die "release-manifest.json 缺少字段：$label"
}

require_manifest_number() {
  local expr="$1"
  local label="$2"
  jq -e "$expr | numbers" "$MANIFEST_PATH" >/dev/null 2>&1 || die "release-manifest.json 缺少字段：$label"
}

require_manifest_string '.release.tag' 'release.tag'
require_manifest_string '.release.commit' 'release.commit'
require_manifest_string '.product.name' 'product.name'
require_manifest_string '.product.repository' 'product.repository'
require_manifest_number '.schemaVersion' 'schemaVersion'
jq -e '.schemaVersion == 1' "$MANIFEST_PATH" >/dev/null 2>&1 || die "release-manifest.json schemaVersion 必须是 1"
jq -e '.product.version? == null' "$MANIFEST_PATH" >/dev/null 2>&1 || die "release-manifest.json 不应包含 product.version"
jq -e '.release.scope | arrays | length > 0' "$MANIFEST_PATH" >/dev/null 2>&1 || die "release-manifest.json 缺少 release.scope"
jq -e '.components.desktop.status | IN("updated","reused","not_included")' "$MANIFEST_PATH" >/dev/null 2>&1 || die "desktop.status 非法"
jq -e '.components.watch.status | IN("updated","reused","not_included")' "$MANIFEST_PATH" >/dev/null 2>&1 || die "watch.status 非法"
jq -e '.components.runtime.status | IN("updated","reused","not_included")' "$MANIFEST_PATH" >/dev/null 2>&1 || die "runtime.status 非法"
jq -e '.components.compatibility.status | IN("updated","reused","not_included")' "$MANIFEST_PATH" >/dev/null 2>&1 || die "compatibility.status 非法"
jq -e '.components.docs.status | IN("updated","reused","not_included")' "$MANIFEST_PATH" >/dev/null 2>&1 || die "docs.status 非法"
require_manifest_string '.runtime.releaseTag' 'runtime.releaseTag'
require_manifest_string '.runtime.manifestSha256' 'runtime.manifestSha256'
jq -e '.artifacts | arrays' "$MANIFEST_PATH" >/dev/null 2>&1 || die "release-manifest.json 缺少 artifacts"
jq -e '.checksums.artifact == "checksums.txt"' "$MANIFEST_PATH" >/dev/null 2>&1 || die "release-manifest.json checksums.artifact 必须是 checksums.txt"
jq -e '
  [
    paths
    | select(length > 0)
    | select(.[-1] == "downloadUrl" or .[-1] == "fallbackDownloadUrl" or .[-1] == "assetBaseUrl" or .[-1] == "releaseAssetBaseUrl" or .[-1] == "manifestUrl")
  ]
  | length == 0
' "$MANIFEST_PATH" >/dev/null 2>&1 || die "release-manifest.json 不应包含客户端 URL 或 GitHub asset URL 字段"

find "$SCAN_DIR" -type f | rg -n 'debug[^/]*\.apk$|app-debug\.apk$' && die "发布目录混入了 debug 手表 APK" || true

for sensitive_name in '*.keystore' '*.jks' 'release.properties' '*.p12' '*.mobileprovision'; do
  if find "$SCAN_DIR" -type f -name "$sensitive_name" | grep -q .; then
    die "发布目录包含敏感文件：$sensitive_name"
  fi
done

text_patterns='Codex Watcher|loccen/codex-watcher|openwatcher-pub-pre|CODEX_WATCHER|watcher\.uuss\.top|top\.uuss|/Users/|C:\\Users\\|release\.properties|tunnelToken|cloudflare.*token|menu-app'

find "$SCAN_DIR" -type f \
  \( -name '*.md' -o -name '*.txt' -o -name '*.json' -o -name '*.plist' -o -name '*.yaml' -o -name '*.yml' -o -name '*.toml' \) \
  -print0 | while IFS= read -r -d '' file; do
    if rg -n -a "$text_patterns" "$file" >/dev/null; then
      die "文本产物中发现旧品牌、旧域名、个人路径或敏感信息：$file"
    fi
  done

find "$SCAN_DIR" -type f \( -name 'openwatcher-*' -o -name '*.exe' -o -name '*.dmg' \) -print0 | while IFS= read -r -d '' file; do
  if strings "$file" | rg -n '/Users/|watcher\.uuss\.top|top\.uuss|loccen/codex-watcher|openwatcher-pub-pre|menu-app' >/dev/null; then
    die "二进制产物中发现个人路径、旧域名或旧品牌：$file"
  fi
done

find "$SCAN_DIR" -type f -name '*.zip' -print0 | while IFS= read -r -d '' file; do
  if zipinfo -1 "$file" | rg -n 'app-debug\.apk$|debug[^/]*\.apk$|\.keystore$|\.jks$|release\.properties$|bundled/(platform-tools|cloudflared|watch-apk)/' >/dev/null; then
    die "压缩包内发现 debug APK 或敏感文件：$file"
  fi
done

while IFS= read -r path; do
  rel_path="${path#$SCAN_DIR/}"
  actual_sha="$(sha256_file "$path")"
  listed_sha="$(checksum_lookup "$rel_path")"
  [[ -n "$listed_sha" ]] || die "checksums.txt 缺少条目：$rel_path"
  [[ "$listed_sha" == "$actual_sha" ]] || die "checksums.txt SHA256 不匹配：$rel_path"
done < <(find "$SCAN_DIR" -maxdepth 1 -type f ! -name 'checksums.txt' | sort)

while IFS=$'\t' read -r name sha256 size_bytes component; do
  [[ -n "$name" ]] || continue
  path="$SCAN_DIR/$name"
  [[ -f "$path" ]] || die "release-manifest.json 引用了不存在的文件：$name"
  actual_sha="$(sha256_file "$path")"
  [[ "$actual_sha" == "$sha256" ]] || die "release-manifest.json SHA256 不匹配：$name"
  actual_size="$(file_size_bytes "$path")"
  [[ "$actual_size" == "$size_bytes" ]] || die "release-manifest.json sizeBytes 不匹配：$name"
  listed_sha="$(checksum_lookup "$name")"
  [[ "$listed_sha" == "$sha256" ]] || die "release-manifest.json 与 checksums.txt 不一致：$name"
done < <(jq -r '.artifacts[]? | [.name, .sha256, (.sizeBytes | tostring), .component] | @tsv' "$MANIFEST_PATH")

watch_status="$(jq -r '.components.watch.status' "$MANIFEST_PATH")"
watch_artifact="$(jq -r '.watch.artifact' "$MANIFEST_PATH")"
watch_sha256="$(jq -r '.watch.sha256' "$MANIFEST_PATH")"
if [[ "$watch_status" == "updated" ]]; then
  [[ -f "$SCAN_DIR/$watch_artifact" ]] || die "Watch APK 不存在：$watch_artifact"
  [[ "$(sha256_file "$SCAN_DIR/$watch_artifact")" == "$watch_sha256" ]] || die "Watch APK SHA256 与 release-manifest.json 不一致：$watch_artifact"
else
  jq -e '.watch.sourceReleaseTag | strings | select(length > 0)' "$MANIFEST_PATH" >/dev/null 2>&1 || die "复用的 watch 缺少 sourceReleaseTag"
fi

desktop_status="$(jq -r '.components.desktop.status' "$MANIFEST_PATH")"
if [[ "$desktop_status" == "updated" ]]; then
  jq -e '.desktop.platforms | objects | length > 0' "$MANIFEST_PATH" >/dev/null 2>&1 || die "更新的 desktop 缺少 desktop.platforms"
  while IFS=$'\t' read -r platform artifact sha256 size_bytes; do
    [[ -n "$platform" && -n "$artifact" ]] || continue
    [[ -f "$SCAN_DIR/$artifact" ]] || die "Desktop 产物不存在：$artifact"
    [[ "$(sha256_file "$SCAN_DIR/$artifact")" == "$sha256" ]] || die "Desktop 平台 SHA256 与 release-manifest.json 不一致：$platform"
    [[ "$(file_size_bytes "$SCAN_DIR/$artifact")" == "$size_bytes" ]] || die "Desktop 平台 sizeBytes 与 release-manifest.json 不一致：$platform"
    if [[ "$artifact" == *.zip ]]; then
      case "$platform" in
        darwin-*)
          zipinfo -1 "$SCAN_DIR/$artifact" | rg -Fx 'OpenWatcher.app/Contents/Library/Helpers/OpenWatcher Widget.app/Contents/MacOS/openwatcher-widget' >/dev/null \
            || die "macOS Desktop 压缩包缺少悬浮球辅助程序：$artifact"
          ;;
        windows-*)
          zipinfo -1 "$SCAN_DIR/$artifact" | rg -Fx "bundled/widget/$platform/openwatcher-widget.exe" >/dev/null \
            || die "Windows Desktop 压缩包缺少悬浮球辅助程序：$artifact"
          ;;
      esac
    fi
  done < <(jq -r '.desktop.platforms | to_entries[] | [.key, .value.artifact, .value.sha256, (.value.sizeBytes | tostring)] | @tsv' "$MANIFEST_PATH")
else
  jq -e '.desktop.sourceReleaseTag | strings | select(length > 0)' "$MANIFEST_PATH" >/dev/null 2>&1 || die "复用的 desktop 缺少 sourceReleaseTag"
fi

if [[ -n "$RUNTIME_MANIFEST_PATH" ]]; then
  [[ -f "$RUNTIME_MANIFEST_PATH" ]] || die "runtime manifest 不存在：$RUNTIME_MANIFEST_PATH"
  runtime_manifest_sha256="$(jq -r '.runtime.manifestSha256' "$MANIFEST_PATH")"
  [[ "$(sha256_file "$RUNTIME_MANIFEST_PATH")" == "$runtime_manifest_sha256" ]] || die "runtime manifest SHA256 与 release-manifest.json 不一致"
fi

jq -e --slurpfile manifest "$MANIFEST_PATH" '
  ($manifest[0]) as $m
  | .schemaVersion == 1
    and .channel == "beta"
    and .id == $m.release.tag
    and .scope == $m.release.scope
    and .components == $m.components
    and (.links.releaseManifestUrl | endswith("/release-manifest.json"))
' "$CHANGELOG_ENTRY_PATH" >/dev/null 2>&1 || die "changelog-entry.json 与 release-manifest.json 不一致"

note "发布产物检查通过：$SCAN_DIR"
