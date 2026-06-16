#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/openwatcher-release-scripts.XXXXXX")"
trap 'rm -rf "$TMP_DIR"' EXIT

PUBLIC_DIR="$TMP_DIR/public-release"
RUNTIME_DIR="$TMP_DIR/runtime"
mkdir -p "$PUBLIC_DIR" "$RUNTIME_DIR"

fixture_desktop_version="${OPENWATCHER_TEST_DESKTOP_VERSION:-$(printf '%s.%s.%s' 0 0 1)}"
fixture_watch_version_name="${OPENWATCHER_TEST_WATCH_VERSION_NAME:-$(printf '%s.%s.%s' 0 0 1)}"
fixture_watch_version_code="${OPENWATCHER_TEST_WATCH_VERSION_CODE:-1}"
fixture_runtime_tag="runtime-v${fixture_desktop_version}"

watch_apk="$PUBLIC_DIR/openwatcher-watchapp-fixture-release.apk"
watch_metadata="$PUBLIC_DIR/latest-apk.json"
watch_changelog="$PUBLIC_DIR/latest-apk-changelog.json"
desktop_darwin="$PUBLIC_DIR/OpenWatcher-Desktop-darwin-arm64.zip"
desktop_windows="$PUBLIC_DIR/OpenWatcher-Desktop-windows-amd64-Setup.exe"
backend_bin="$PUBLIC_DIR/openwatcher-darwin-arm64"
notices="$PUBLIC_DIR/THIRD_PARTY_NOTICES.md"
runtime_manifest="$RUNTIME_DIR/runtime-manifest.json"

printf 'watch apk fixture\n' >"$watch_apk"
printf 'desktop windows fixture\n' >"$desktop_windows"
printf 'backend fixture\n' >"$backend_bin"
printf '# Third Party Notices\n' >"$notices"
printf '{"entries":[{"version":"%s","summary":"fixture"}]}\n' "$fixture_watch_version_name" >"$watch_changelog"
printf '{"schemaVersion":1,"channel":"beta","generatedAt":"2026-06-11T00:00:00Z","releaseSlug":"fixture","desktopMinVersion":"%s","resources":{"watchApk":{"version":"%s+%s","artifact":"watch.apk","url":"https://example.com/watch.apk","sha256":"fixture","sizeBytes":1},"platformTools":{"darwin-arm64":{"version":"fixture","artifact":"pt.zip","url":"https://example.com/pt.zip","sha256":"fixture","sizeBytes":1,"archiveKind":"zip","binRelativePath":"platform-tools/adb"}},"cloudflared":{"darwin-arm64":{"version":"fixture","artifact":"cf.tgz","url":"https://example.com/cf.tgz","sha256":"fixture","sizeBytes":1,"archiveKind":"tgz","binRelativePath":"cloudflared"}}},"notes":"fixture"}\n' "$fixture_desktop_version" "$fixture_watch_version_name" "$fixture_watch_version_code" >"$runtime_manifest"

watch_sha256="$(shasum -a 256 "$watch_apk" | awk '{print $1}')"
desktop_zip_stage="$TMP_DIR/desktop-zip-stage"
mkdir -p "$desktop_zip_stage"
printf 'desktop darwin fixture\n' >"$desktop_zip_stage/OpenWatcher.app"
(
  cd "$desktop_zip_stage"
  zip -qry "$desktop_darwin" OpenWatcher.app
)

cat >"$watch_metadata" <<JSON
{
  "channel": "beta",
  "publishedAt": "2026-06-11T00:00:00Z",
  "versionName": "$fixture_watch_version_name",
  "versionCode": $fixture_watch_version_code,
  "commit": "abcdef0",
  "artifact": "$(basename "$watch_apk")",
  "sha256": "$watch_sha256",
  "changelogUrl": "https://openwatcher.ai/changelog.json",
  "summary": "fixture watch release"
}
JSON

run_generate_manifest() {
  OPENWATCHER_PUBLIC_RELEASE_DIR="$1" \
  OPENWATCHER_RELEASE_TAG="$2" \
  OPENWATCHER_RELEASE_SCOPE="$3" \
  OPENWATCHER_RELEASE_SUMMARY="$4" \
  OPENWATCHER_RELEASE_PUBLISHED_AT="2026-06-11T00:00:00Z" \
  OPENWATCHER_RELEASE_COMMIT="abcdef0123456789abcdef0123456789abcdef01" \
  OPENWATCHER_RUNTIME_MANIFEST_PATH="$runtime_manifest" \
  OPENWATCHER_RUNTIME_MANIFEST_URL="https://github.com/openwatcher-ai/openwatcher/releases/download/${fixture_runtime_tag}/runtime-manifest.json" \
  OPENWATCHER_RUNTIME_RELEASE_TAG="$fixture_runtime_tag" \
  OPENWATCHER_DESKTOP_VERSION="$fixture_desktop_version" \
  OPENWATCHER_PREVIOUS_CHANNEL_MANIFEST_PATH="${5:-}" \
  "$ROOT_DIR/scripts/generate-release-manifest.sh"
}

run_generate_changelog() {
  OPENWATCHER_PUBLIC_RELEASE_DIR="$1" \
  "$ROOT_DIR/scripts/generate-changelog-entry.sh"
}

run_generate_notes() {
  OPENWATCHER_PUBLIC_RELEASE_DIR="$1" \
  "$ROOT_DIR/scripts/generate-release-notes.sh"
}

run_generate_checksums() {
  OPENWATCHER_PUBLIC_RELEASE_DIR="$1" "$ROOT_DIR/scripts/generate-checksums.sh"
}

run_check() {
  OPENWATCHER_PUBLIC_RELEASE_DIR="$1" \
  OPENWATCHER_RUNTIME_MANIFEST_PATH="$runtime_manifest" \
  "$ROOT_DIR/scripts/check-release-artifacts.sh" "$1"
}

expect_fail() {
  local label="$1"
  shift
  if "$@"; then
    echo "期望失败但成功：$label" >&2
    exit 1
  fi
}

run_generate_manifest "$PUBLIC_DIR" "beta-2026.06.11.1" "full" "新增 fixture product release"
run_generate_changelog "$PUBLIC_DIR"
run_generate_notes "$PUBLIC_DIR"
run_generate_checksums "$PUBLIC_DIR"
run_check "$PUBLIC_DIR"
jq -e '.notes.features | length == 2' "$PUBLIC_DIR/changelog-entry.json" >/dev/null
jq -e '.notes.features | map(.component) | sort == ["手表应用","桌面应用"]' "$PUBLIC_DIR/changelog-entry.json" >/dev/null
rg -n '^## 新增功能$' "$PUBLIC_DIR/release-notes.md" >/dev/null
rg -n '^- 【桌面应用】新增 fixture product release$' "$PUBLIC_DIR/release-notes.md" >/dev/null
rg -n '^- 【手表应用】新增 fixture product release$' "$PUBLIC_DIR/release-notes.md" >/dev/null

desktop_only_dir="$TMP_DIR/desktop-only"
mkdir -p "$desktop_only_dir"
cp "$PUBLIC_DIR/OpenWatcher-Desktop-darwin-arm64.zip" "$desktop_only_dir/"
cp "$PUBLIC_DIR/OpenWatcher-Desktop-windows-amd64-Setup.exe" "$desktop_only_dir/"
cp "$PUBLIC_DIR/THIRD_PARTY_NOTICES.md" "$desktop_only_dir/"
run_generate_manifest "$desktop_only_dir" "beta-2026.06.11.2" "desktop" "修复 desktop only release" "$PUBLIC_DIR/release-manifest.json"
run_generate_changelog "$desktop_only_dir"
run_generate_notes "$desktop_only_dir"
run_generate_checksums "$desktop_only_dir"
run_check "$desktop_only_dir"
jq -e '(.notes.fixes | length == 1) and (.notes.fixes[0].component == "桌面应用")' "$desktop_only_dir/changelog-entry.json" >/dev/null
rg -n '^## 问题修复$' "$desktop_only_dir/release-notes.md" >/dev/null
rg -n '^- 【桌面应用】修复 desktop only release$' "$desktop_only_dir/release-notes.md" >/dev/null

legacy_previous_manifest="$TMP_DIR/legacy-previous-release-manifest.json"
jq '
  .desktop.platforms["darwin-arm64"].downloadUrl = "https://github.com/openwatcher-ai/openwatcher/releases/download/beta/desktop.zip"
  | .runtime.manifestUrl = "https://github.com/openwatcher-ai/openwatcher/releases/download/runtime/runtime-manifest.json"
  | .runtime.assetBaseUrl = "https://github.com/openwatcher-ai/openwatcher/releases/download/runtime"
  | .watch.downloadUrl = "https://github.com/openwatcher-ai/openwatcher/releases/download/beta/watch.apk"
  | .watch.fallbackDownloadUrl = "https://github.com/openwatcher-ai/openwatcher/releases/download/beta/watch.apk"
' "$PUBLIC_DIR/release-manifest.json" >"$legacy_previous_manifest"

watch_only_dir="$TMP_DIR/watch-only"
mkdir -p "$watch_only_dir"
cp "$watch_apk" "$watch_only_dir/"
cp "$watch_metadata" "$watch_only_dir/"
cp "$watch_changelog" "$watch_only_dir/"
cp "$PUBLIC_DIR/THIRD_PARTY_NOTICES.md" "$watch_only_dir/"
run_generate_manifest "$watch_only_dir" "beta-2026.06.11.3" "watch" "优化 watch only release" "$legacy_previous_manifest"
run_generate_changelog "$watch_only_dir"
run_generate_notes "$watch_only_dir"
run_generate_checksums "$watch_only_dir"
run_check "$watch_only_dir"

missing_field_dir="$TMP_DIR/missing-field"
wrong_sha_dir="$TMP_DIR/wrong-sha"
bad_url_dir="$TMP_DIR/bad-url"
cp -R "$desktop_only_dir" "$missing_field_dir"
cp -R "$desktop_only_dir" "$wrong_sha_dir"
cp -R "$desktop_only_dir" "$bad_url_dir"

tmp_json="$TMP_DIR/tmp.json"
jq 'del(.release.scope)' "$missing_field_dir/release-manifest.json" >"$tmp_json"
mv "$tmp_json" "$missing_field_dir/release-manifest.json"
run_generate_checksums "$missing_field_dir"
expect_fail "manifest 缺字段" run_check "$missing_field_dir"

jq '.desktop.platforms["darwin-arm64"].sha256 = "deadbeef"' "$wrong_sha_dir/release-manifest.json" >"$tmp_json"
mv "$tmp_json" "$wrong_sha_dir/release-manifest.json"
run_generate_checksums "$wrong_sha_dir"
expect_fail "manifest SHA256 错误" run_check "$wrong_sha_dir"

jq '.watch.downloadUrl = "https://github.com/openwatcher-ai/openwatcher/releases/download/beta/watch.apk"' "$bad_url_dir/release-manifest.json" >"$tmp_json"
mv "$tmp_json" "$bad_url_dir/release-manifest.json"
run_generate_checksums "$bad_url_dir"
expect_fail "manifest 不允许客户端下载 URL" run_check "$bad_url_dir"

echo "release 脚本验证通过"
