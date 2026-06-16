#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WATCH_APP_DIR="$ROOT_DIR/watch-app"
HISTORY_FILE="$WATCH_APP_DIR/RELEASE_BUILDS.md"
DIST_DIR="$ROOT_DIR/dist"
LATEST_APK_URL="${OPENWATCHER_LATEST_APK_URL:-}"
CHANGELOG_URL="${OPENWATCHER_CHANGELOG_URL:-}"
RELEASE_CHANNEL="${OPENWATCHER_RELEASE_CHANNEL:-dev}"
SLUG="${1:-release}"
if [[ $# -gt 0 ]]; then
  shift
fi
SAFE_SLUG="$(printf '%s' "$SLUG" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9._-]+/-/g; s/^-+//; s/-+$//')"
if [[ -z "$SAFE_SLUG" ]]; then
  SAFE_SLUG="release"
fi

normalize_summary() {
  printf '%s' "$1" | tr '\r\n|' '  /' | awk '{$1=$1; print}'
}

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/ }"
  value="${value//$'\r'/ }"
  printf '%s' "$value"
}

generate_latest_changelog_json() {
  local history_file="$1"
  local output_path="$2"
  if ! command -v go >/dev/null 2>&1; then
    echo "找不到 go，无法生成 latest-apk-changelog.json。" >&2
    exit 2
  fi
  (
    cd "$ROOT_DIR"
    go run ./cmd/watch-release-changelog --history "$history_file" --output "$output_path"
  )
}

write_history_markdown() {
  local output_path="$1"
  local row="$2"
  cat > "$output_path" <<EOF
# 手表 APK 构建记录

| 构建时间 UTC | versionName | versionCode | Git commit | 构建分支 | APK 文件 | SHA256 | 说明类型 | 变更摘要 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
$row
EOF
}

if [[ -n "$(git -C "$ROOT_DIR" status --short -- ':!dist')" ]]; then
  echo "release APK 必须基于已提交的工作区构建；请先提交代码改动，再重新运行。" >&2
  git -C "$ROOT_DIR" status --short -- ':!dist' >&2
  exit 2
fi

source "$ROOT_DIR/scripts/release-common.sh"
VERSION_NAME="$(require_watch_version_name)"
VERSION_CODE="$(require_watch_version_code)"

COMMIT="$(git -C "$ROOT_DIR" rev-parse --short HEAD)"
BRANCH="$(git -C "$ROOT_DIR" branch --show-current)"
if [[ -z "$BRANCH" ]]; then
  BRANCH="detached"
fi
SUMMARY="$(normalize_summary "${RELEASE_SUMMARY:-}")"
if [[ -z "$SUMMARY" ]]; then
  echo "必须提供面向用户的 RELEASE_SUMMARY；不再回退到 git 提交标题。" >&2
  exit 2
fi
SUMMARY_JSON="$(json_escape "$SUMMARY")"
BRANCH_JSON="$(json_escape "$BRANCH")"
LATEST_APK_URL_JSON="$(json_escape "$LATEST_APK_URL")"
CHANGELOG_URL_JSON="$(json_escape "$CHANGELOG_URL")"
RELEASE_CHANNEL_JSON="$(json_escape "$RELEASE_CHANNEL")"
BUILT_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
VERSION_LABEL="$(release_filename_version "$VERSION_NAME")"
if [[ "$SAFE_SLUG" == runtime-* ]]; then
  FINAL_APK="$DIST_DIR/watchapp-runtime_${VERSION_LABEL}.apk"
else
  FINAL_APK="$DIST_DIR/watchapp_${VERSION_LABEL}.apk"
fi

find_apksigner() {
  if [[ -n "${ANDROID_HOME:-}" && -x "$ANDROID_HOME/build-tools/$(ls "$ANDROID_HOME/build-tools" 2>/dev/null | sort | tail -1)/apksigner" ]]; then
    printf '%s\n' "$ANDROID_HOME/build-tools/$(ls "$ANDROID_HOME/build-tools" | sort | tail -1)/apksigner"
    return
  fi
  find "$HOME/Library/Android/sdk/build-tools" -type f -name apksigner 2>/dev/null | sort | tail -1
}

APK_SIGNER="$(find_apksigner)"
if [[ -z "$APK_SIGNER" || ! -x "$APK_SIGNER" ]]; then
  echo "找不到 apksigner，无法验证最终 APK。" >&2
  exit 2
fi

mkdir -p "$DIST_DIR"
(cd "$WATCH_APP_DIR" && ./gradlew "-PopenWatcherVersionName=$VERSION_NAME" "-PopenWatcherVersionCode=$VERSION_CODE" assembleRelease "$@")

SOURCE_APK="$WATCH_APP_DIR/app/build/outputs/apk/release/app-release.apk"
if [[ ! -f "$SOURCE_APK" ]]; then
  echo "找不到 release 构建产物：$SOURCE_APK" >&2
  exit 2
fi

cp "$SOURCE_APK" "$FINAL_APK"
"$APK_SIGNER" verify -v --print-certs "$FINAL_APK" > "$FINAL_APK.apksigner.txt"
SHA256="$(shasum -a 256 "$FINAL_APK" | awk '{print $1}')"
printf '%s  %s\n' "$SHA256" "$(basename "$FINAL_APK")" > "$FINAL_APK.sha256"
if command -v zipinfo >/dev/null 2>&1; then
  zipinfo -1 "$FINAL_APK" | grep '^lib/' > "$FINAL_APK.libs.txt" || true
fi
ABI_LIST="none"
if [[ -s "$FINAL_APK.libs.txt" ]]; then
  ABI_LIST="$(sed -nE 's#^lib/([^/]+)/.*#\1#p' "$FINAL_APK.libs.txt" | sort -u | paste -sd, -)"
fi
APK_VERIFY_STATUS="failed"
if grep -q '^Verifies$' "$FINAL_APK.apksigner.txt"; then
  APK_VERIFY_STATUS="Verifies"
fi

cat > "$FINAL_APK.json" <<JSON
{
  "channel": "$RELEASE_CHANNEL_JSON",
  "builtAt": "$BUILT_AT",
  "publishedAt": "$BUILT_AT",
  "versionName": "$VERSION_NAME",
  "versionCode": $VERSION_CODE,
  "commit": "$COMMIT",
  "branch": "$BRANCH_JSON",
  "artifact": "$(basename "$FINAL_APK")",
  "sha256": "$SHA256",
  "summary": "$SUMMARY_JSON"
$(if [[ -n "$LATEST_APK_URL_JSON" ]]; then printf ',\n  "downloadUrl": "%s"' "$LATEST_APK_URL_JSON"; fi)
$(if [[ -n "$CHANGELOG_URL_JSON" ]]; then printf ',\n  "changelogUrl": "%s"' "$CHANGELOG_URL_JSON"; fi)
}
JSON
cp "$FINAL_APK.json" "$DIST_DIR/latest-apk.json"

RELATIVE_APK="${FINAL_APK#$ROOT_DIR/}"
CURRENT_HISTORY_ROW="$(printf '| %s | %s | %s | %s | %s | %s | %s | %s | %s |' \
  "$BUILT_AT" "$VERSION_NAME" "$VERSION_CODE" "$COMMIT" "$BRANCH" "$RELATIVE_APK" "$SHA256" "user" "$SUMMARY")"
printf '%s\n' "$CURRENT_HISTORY_ROW" >> "$HISTORY_FILE"
CURRENT_HISTORY_FILE="$(mktemp "${TMPDIR:-/tmp}/openwatcher-current-watch-release.XXXXXX.md")"
trap 'rm -f "$CURRENT_HISTORY_FILE"' EXIT
write_history_markdown "$CURRENT_HISTORY_FILE" "$CURRENT_HISTORY_ROW"
generate_latest_changelog_json "$CURRENT_HISTORY_FILE" "$DIST_DIR/latest-apk-changelog.json"

cat <<SUMMARY
Release APK summary
  artifact: $(basename "$FINAL_APK")
  versionName: $VERSION_NAME
  versionCode: $VERSION_CODE
  commit: $COMMIT
  branch: $BRANCH
  sha256: $SHA256
  channel: $RELEASE_CHANNEL
  url: $LATEST_APK_URL
  abi: $ABI_LIST
  apksigner: $APK_VERIFY_STATUS
  metadata: dist/$(basename "$FINAL_APK").json
  latestMetadata: dist/latest-apk.json
  latestChangelog: dist/latest-apk-changelog.json
  history: watch-app/RELEASE_BUILDS.md
SUMMARY

echo "$FINAL_APK"
