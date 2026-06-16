#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="${OPENWATCHER_PUBLIC_RELEASE_DIR:-$ROOT_DIR/dist/public-release}"
source "$ROOT_DIR/scripts/release-common.sh"

SCAN_DIR="${1:-$RELEASE_DIR}"
MANIFEST_PATH="${OPENWATCHER_RELEASE_MANIFEST_PATH:-$SCAN_DIR/release-manifest.json}"
REPOSITORY_NAME="${OPENWATCHER_GITHUB_REPOSITORY:-${GITHUB_REPOSITORY:-openwatcher-ai/openwatcher}}"
PRODUCT_NAME="${OPENWATCHER_PRODUCT_NAME:-OpenWatcher}"
RELEASE_TAG="$(normalize_beta_release_tag "${OPENWATCHER_RELEASE_TAG:-}")"
RELEASE_SCOPE="$(normalize_release_scope "${OPENWATCHER_RELEASE_SCOPE:-full}")"
RELEASE_SUMMARY="${OPENWATCHER_RELEASE_SUMMARY:-}"
RELEASE_PUBLISHED_AT="${OPENWATCHER_RELEASE_PUBLISHED_AT:-$BUILT_AT_UTC}"
RELEASE_COMMIT="${OPENWATCHER_RELEASE_COMMIT:-$(git -C "$ROOT_DIR" rev-parse HEAD)}"
RELEASE_BRANCH="${OPENWATCHER_RELEASE_BRANCH:-main}"
RUNTIME_RELEASE_TAG="$(resolve_runtime_release_tag)"
RUNTIME_MANIFEST_PATH="${OPENWATCHER_RUNTIME_MANIFEST_PATH:-}"
RUNTIME_MANIFEST_SHA256="${OPENWATCHER_RUNTIME_MANIFEST_SHA256:-}"
PREVIOUS_RELEASE_MANIFEST_PATH="${OPENWATCHER_PREVIOUS_RELEASE_MANIFEST_PATH:-${OPENWATCHER_PREVIOUS_CHANNEL_MANIFEST_PATH:-}}"
DESKTOP_VERSION="$(trim_value "${OPENWATCHER_DESKTOP_VERSION:-}")"
OFFICIAL_CHANGELOG_URL="${OPENWATCHER_OFFICIAL_CHANGELOG_URL:-https://openwatcher.ai/changelog.json}"

[[ -d "$SCAN_DIR" ]] || die "公开发布目录不存在：$SCAN_DIR"
[[ -n "$RELEASE_TAG" ]] || die "缺少 release tag，请设置 OPENWATCHER_RELEASE_TAG"

require_command jq

if [[ -n "$RUNTIME_MANIFEST_PATH" ]]; then
  [[ -f "$RUNTIME_MANIFEST_PATH" ]] || die "runtime manifest 不存在：$RUNTIME_MANIFEST_PATH"
  RUNTIME_MANIFEST_SHA256="$(sha256_file "$RUNTIME_MANIFEST_PATH")"
fi
[[ -n "$RUNTIME_MANIFEST_SHA256" ]] || die "缺少 runtime manifest SHA256；请设置 OPENWATCHER_RUNTIME_MANIFEST_PATH 或 OPENWATCHER_RUNTIME_MANIFEST_SHA256"

require_previous_manifest() {
  [[ -n "$PREVIOUS_RELEASE_MANIFEST_PATH" ]] || die "release_scope=$RELEASE_SCOPE 需要上一版 release manifest"
  [[ -f "$PREVIOUS_RELEASE_MANIFEST_PATH" ]] || die "上一版 release manifest 不存在：$PREVIOUS_RELEASE_MANIFEST_PATH"
}

previous_manifest_json() {
  require_previous_manifest
  jq -c '.' "$PREVIOUS_RELEASE_MANIFEST_PATH"
}

previous_manifest_value() {
  require_previous_manifest
  jq -er "$1" "$PREVIOUS_RELEASE_MANIFEST_PATH"
}

scope_components_json() {
  case "$RELEASE_SCOPE" in
    full) printf '["desktop","watch"]' ;;
    desktop) printf '["desktop"]' ;;
    watch) printf '["watch"]' ;;
    runtime-pointer) printf '["runtime"]' ;;
    docs) printf '["docs"]' ;;
    compatibility) printf '["compatibility"]' ;;
    metadata) printf '["metadata"]' ;;
  esac
}

component_status() {
  local component="$1"
  case "$RELEASE_SCOPE:$component" in
    full:desktop|desktop:desktop) printf 'updated' ;;
    full:watch|watch:watch) printf 'updated' ;;
    runtime-pointer:runtime) printf 'updated' ;;
    compatibility:compatibility) printf 'updated' ;;
    docs:docs) printf 'updated' ;;
    full:runtime|desktop:runtime|watch:runtime|docs:runtime|compatibility:runtime|metadata:runtime) printf 'reused' ;;
    desktop:watch|watch:desktop|runtime-pointer:desktop|runtime-pointer:watch|docs:desktop|docs:watch|compatibility:desktop|compatibility:watch|metadata:desktop|metadata:watch) printf 'reused' ;;
    runtime-pointer:compatibility|runtime-pointer:docs|desktop:compatibility|desktop:docs|watch:compatibility|watch:docs|full:compatibility|full:docs|metadata:compatibility|metadata:docs) printf 'not_included' ;;
    docs:compatibility|compatibility:docs) printf 'not_included' ;;
    *) printf 'not_included' ;;
  esac
}

desktop_artifact_platform() {
  if [[ "$1" =~ ^desktop_v[0-9a-z][0-9a-z.-]*_(macos_x64|macos_arm64|windows_x64|windows_arm64)\.(zip|dmg|exe)$ ]]; then
    artifact_platform_id_from_label "${BASH_REMATCH[1]}"
    return 0
  fi
  if [[ "$1" =~ ^OpenWatcher-Desktop-([a-z0-9]+-[a-z0-9]+)(-Setup)?\.(zip|dmg|exe)$ ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  fi
  return 0
}

prefer_desktop_candidate() {
  local current="$1"
  local candidate="$2"
  local platform="$3"
  if [[ -z "$current" ]]; then
    return 0
  fi
  if [[ "$platform" == darwin-* ]]; then
    [[ "$candidate" == *.dmg && "$current" != *.dmg ]]
    return
  fi
  if [[ "$platform" == windows-* ]]; then
    [[ "$candidate" == *.exe && "$current" != *.exe ]]
    return
  fi
  return 1
}

artifact_component() {
  case "$1" in
    desktop_v*) printf 'desktop' ;;
    OpenWatcher-Desktop-*) printf 'desktop' ;;
    watchapp_*.apk|watchapp-runtime_*.apk) printf 'watch' ;;
    openwatcher-watchapp-*.apk) printf 'watch' ;;
    openwatcher_v*) printf 'backend' ;;
    openwatcher-*.exe|openwatcher-darwin-*|openwatcher-windows-*) printf 'backend' ;;
    latest-apk*.json|channel-beta.json|changelog-entry.json|release-manifest.json) printf 'metadata' ;;
    *.json) printf 'metadata' ;;
    *.md|*.txt) printf 'documentation' ;;
    *) printf 'asset' ;;
  esac
}

watch_metadata_path=""
watch_artifact=""
watch_version_name=""
watch_version_code=""
watch_commit=""
watch_summary=""
watch_published_at=""
watch_sha256=""
watch_size_bytes=""
watch_source_release_tag=""

if [[ "$(component_status watch)" == "updated" ]]; then
  if [[ -f "$SCAN_DIR/latest-apk.json" ]]; then
    watch_metadata_path="$SCAN_DIR/latest-apk.json"
  fi
  [[ -n "$watch_metadata_path" && -f "$watch_metadata_path" ]] || die "release_scope=$RELEASE_SCOPE 需要 latest-apk.json"
  watch_artifact="$(jq -r '.artifact // empty' "$watch_metadata_path")"
  watch_version_name="$(jq -r '.versionName // empty' "$watch_metadata_path")"
  watch_version_code="$(jq -r '.versionCode // empty' "$watch_metadata_path")"
  watch_commit="$(jq -r '.commit // empty' "$watch_metadata_path")"
  watch_summary="$(jq -r '.summary // empty' "$watch_metadata_path")"
  watch_published_at="$(jq -r '.publishedAt // .builtAt // empty' "$watch_metadata_path")"
  [[ -n "$watch_artifact" ]] || die "latest-apk.json 缺少 artifact"
  [[ -n "$watch_version_name" ]] || die "latest-apk.json 缺少 versionName"
  [[ -n "$watch_version_code" && "$watch_version_code" != "null" ]] || die "latest-apk.json 缺少 versionCode"
  watch_artifact_path="$SCAN_DIR/$watch_artifact"
  [[ -f "$watch_artifact_path" ]] || die "Watch APK 不存在：$watch_artifact_path"
  watch_sha256="$(sha256_file "$watch_artifact_path")"
  watch_size_bytes="$(file_size_bytes "$watch_artifact_path")"
  [[ -n "$watch_published_at" ]] || watch_published_at="$RELEASE_PUBLISHED_AT"
else
  watch_version_name="$(previous_manifest_value '.watch.versionName // empty')"
  watch_version_code="$(previous_manifest_value '.watch.versionCode')"
  watch_artifact="$(previous_manifest_value '.watch.artifact // empty')"
  watch_sha256="$(previous_manifest_value '.watch.sha256 // empty')"
  watch_size_bytes="$(previous_manifest_value '.watch.sizeBytes')"
  watch_summary="$(jq -r '.watch.summary // .release.summary // empty' "$PREVIOUS_RELEASE_MANIFEST_PATH")"
  watch_published_at="$(jq -r '.watch.publishedAt // .release.publishedAt // empty' "$PREVIOUS_RELEASE_MANIFEST_PATH")"
  watch_commit="$(jq -r '.watch.commit // .release.commit // empty' "$PREVIOUS_RELEASE_MANIFEST_PATH")"
  watch_source_release_tag="$(jq -r '.release.tag // empty' "$PREVIOUS_RELEASE_MANIFEST_PATH")"
  [[ -n "$watch_version_name" && -n "$watch_artifact" && -n "$watch_sha256" && -n "$watch_size_bytes" ]] || die "上一版 release manifest 缺少完整的 watch 字段"
fi

desktop_version="$DESKTOP_VERSION"
desktop_source_release_tag=""
desktop_platforms_tmp="$(mktemp "${TMPDIR:-/tmp}/openwatcher-desktop-platforms.XXXXXX.jsonl")"
artifacts_tmp="$(mktemp "${TMPDIR:-/tmp}/openwatcher-release-artifacts.XXXXXX.jsonl")"
trap 'rm -f "$desktop_platforms_tmp" "$artifacts_tmp"' EXIT

if [[ "$(component_status desktop)" == "updated" ]]; then
  [[ -n "$DESKTOP_VERSION" ]] || die "release_scope=$RELEASE_SCOPE 需要 OPENWATCHER_DESKTOP_VERSION"
  validate_semver_version "Desktop 版本" "$DESKTOP_VERSION"
  while IFS= read -r path; do
    name="$(basename "$path")"
    platform="$(desktop_artifact_platform "$name")"
    [[ -n "$platform" ]] || continue
    rank=1
    if [[ "$name" == *.zip ]]; then
      rank=3
    elif [[ "$platform" == darwin-* && "$name" == *.dmg ]]; then
      rank=2
    elif [[ "$platform" == windows-* && "$name" == *-Setup.exe ]]; then
      rank=2
    fi
    jq -cn \
      --arg platform "$platform" \
      --arg artifact "$name" \
      --arg sha256 "$(sha256_file "$path")" \
      --argjson sizeBytes "$(file_size_bytes "$path")" \
      --argjson rank "$rank" \
      '{
        platform: $platform,
        artifact: $artifact,
        sha256: $sha256,
        sizeBytes: $sizeBytes,
        rank: $rank
      }' >>"$desktop_platforms_tmp"
    printf '\n' >>"$desktop_platforms_tmp"
  done < <(find "$SCAN_DIR" -maxdepth 1 -type f \( -name 'desktop_v*' -o -name 'OpenWatcher-Desktop-*' \) | sort)

  [[ -s "$desktop_platforms_tmp" ]] || die "release_scope=$RELEASE_SCOPE 需要 Desktop 产物"
else
  desktop_version="$(previous_manifest_value '.desktop.version // empty')"
  desktop_source_release_tag="$(jq -r '.release.tag // empty' "$PREVIOUS_RELEASE_MANIFEST_PATH")"
  jq -ce '.desktop.platforms' "$PREVIOUS_RELEASE_MANIFEST_PATH" >/dev/null 2>&1 || die "上一版 release manifest 缺少 desktop.platforms"
fi

while IFS= read -r path; do
  name="$(basename "$path")"
  jq -cn \
    --arg name "$name" \
    --arg component "$(artifact_component "$name")" \
    --arg sha256 "$(sha256_file "$path")" \
    --argjson sizeBytes "$(file_size_bytes "$path")" \
    '{
      name: $name,
      component: $component,
      sha256: $sha256,
      sizeBytes: $sizeBytes
    }' >>"$artifacts_tmp"
  printf '\n' >>"$artifacts_tmp"
done < <(find "$SCAN_DIR" -maxdepth 1 -type f ! -name 'checksums.txt' ! -name 'release-manifest.json' | sort)

desktop_platforms_json='{}'
if [[ "$(component_status desktop)" == "updated" ]]; then
  desktop_platforms_json="$(jq -sc 'sort_by(.platform, .rank) | group_by(.platform) | map(.[-1]) | map({(.platform): {artifact: .artifact, sha256: .sha256, sizeBytes: .sizeBytes}}) | add' "$desktop_platforms_tmp")"
else
  desktop_platforms_json="$(
    jq -c '
      .desktop.platforms
      | with_entries(
          .value |= (
            {
              artifact: .artifact,
              sha256: .sha256
            }
            + (if has("sizeBytes") then {sizeBytes: .sizeBytes} else {} end)
          )
        )
    ' "$PREVIOUS_RELEASE_MANIFEST_PATH"
  )"
fi

artifacts_json="$(jq -sc '.' "$artifacts_tmp")"
scope_json="$(scope_components_json)"

runtime_release_tag="$RUNTIME_RELEASE_TAG"
runtime_manifest_sha256="$RUNTIME_MANIFEST_SHA256"
runtime_source_release_tag=""
if [[ "$(component_status runtime)" != "updated" && -n "$PREVIOUS_RELEASE_MANIFEST_PATH" && -f "$PREVIOUS_RELEASE_MANIFEST_PATH" ]]; then
  runtime_release_tag="$(jq -r '.runtime.releaseTag // empty' "$PREVIOUS_RELEASE_MANIFEST_PATH")"
  runtime_manifest_sha256="$(jq -r '.runtime.manifestSha256 // empty' "$PREVIOUS_RELEASE_MANIFEST_PATH")"
  runtime_source_release_tag="$(jq -r '.release.tag // empty' "$PREVIOUS_RELEASE_MANIFEST_PATH")"
fi
[[ -n "$runtime_release_tag" ]] || die "缺少 runtime releaseTag"
[[ -n "$runtime_manifest_sha256" ]] || die "缺少 runtime manifest SHA256"

components_json="$(jq -cn \
  --arg desktopStatus "$(component_status desktop)" \
  --arg desktopVersion "$desktop_version" \
  --arg desktopSourceReleaseTag "$desktop_source_release_tag" \
  --arg watchStatus "$(component_status watch)" \
  --arg watchVersionName "$watch_version_name" \
  --argjson watchVersionCode "$watch_version_code" \
  --arg watchSourceReleaseTag "$watch_source_release_tag" \
  --arg runtimeStatus "$(component_status runtime)" \
  --arg runtimeReleaseTag "$runtime_release_tag" \
  --arg runtimeSourceReleaseTag "$runtime_source_release_tag" \
  --arg compatibilityStatus "$(component_status compatibility)" \
  --arg docsStatus "$(component_status docs)" \
  '{
    desktop: {
      status: $desktopStatus,
      version: $desktopVersion
    },
    watch: {
      status: $watchStatus,
      versionName: $watchVersionName,
      versionCode: $watchVersionCode
    },
    runtime: {
      status: $runtimeStatus,
      releaseTag: $runtimeReleaseTag
    },
    compatibility: {
      status: $compatibilityStatus
    },
    docs: {
      status: $docsStatus
    }
  }
  | if $desktopSourceReleaseTag == "" then . else .desktop.sourceReleaseTag = $desktopSourceReleaseTag end
  | if $watchSourceReleaseTag == "" then . else .watch.sourceReleaseTag = $watchSourceReleaseTag end
  | if $runtimeSourceReleaseTag == "" then . else .runtime.sourceReleaseTag = $runtimeSourceReleaseTag end')"

jq -n \
  --arg releaseTag "$RELEASE_TAG" \
  --arg releaseSummary "$RELEASE_SUMMARY" \
  --arg releaseBuiltAt "$BUILT_AT_UTC" \
  --arg releasePublishedAt "$RELEASE_PUBLISHED_AT" \
  --arg releaseCommit "$RELEASE_COMMIT" \
  --arg releaseBranch "$RELEASE_BRANCH" \
  --arg productName "$PRODUCT_NAME" \
  --arg repository "$REPOSITORY_NAME" \
  --arg desktopVersion "$desktop_version" \
  --arg desktopSourceReleaseTag "$desktop_source_release_tag" \
  --arg watchVersionName "$watch_version_name" \
  --argjson watchVersionCode "$watch_version_code" \
  --arg watchArtifact "$watch_artifact" \
  --arg watchCommit "$watch_commit" \
  --arg watchSha256 "$watch_sha256" \
  --argjson watchSizeBytes "${watch_size_bytes:-0}" \
  --arg watchPublishedAt "$watch_published_at" \
  --arg watchSummary "$watch_summary" \
  --arg watchChangelogUrl "$OFFICIAL_CHANGELOG_URL" \
  --arg watchSourceReleaseTag "$watch_source_release_tag" \
  --arg runtimeReleaseTag "$runtime_release_tag" \
  --arg runtimeManifestSha256 "$runtime_manifest_sha256" \
  --arg runtimeSourceReleaseTag "$runtime_source_release_tag" \
  --argjson scope "$scope_json" \
  --argjson components "$components_json" \
  --argjson desktopPlatforms "$desktop_platforms_json" \
  --argjson artifacts "$artifacts_json" \
  '{
    schemaVersion: 1,
    channel: "beta",
    release: {
      tag: $releaseTag,
      scope: $scope,
      summary: $releaseSummary,
      builtAt: $releaseBuiltAt,
      publishedAt: $releasePublishedAt,
      commit: $releaseCommit,
      branch: $releaseBranch
    },
    product: {
      name: $productName,
      repository: $repository
    },
    components: $components,
    desktop: {
      version: $desktopVersion,
      platforms: $desktopPlatforms
    },
    watch: {
      versionName: $watchVersionName,
      versionCode: $watchVersionCode,
      artifact: $watchArtifact,
      sha256: $watchSha256,
      sizeBytes: $watchSizeBytes,
      publishedAt: $watchPublishedAt,
      summary: $watchSummary,
      changelogUrl: $watchChangelogUrl
    },
    runtime: {
      releaseTag: $runtimeReleaseTag,
      manifestArtifact: "runtime-manifest.json",
      manifestSha256: $runtimeManifestSha256
    },
    checksums: {
      algorithm: "sha256",
      artifact: "checksums.txt"
    },
    artifacts: $artifacts
  }
  | if $watchCommit == "" then . else .watch.commit = $watchCommit end
  | if $watchSourceReleaseTag == "" then . else .watch.sourceReleaseTag = $watchSourceReleaseTag end
  | if $desktopSourceReleaseTag == "" then . else .desktop.sourceReleaseTag = $desktopSourceReleaseTag end
  | if $runtimeSourceReleaseTag == "" then . else .runtime.sourceReleaseTag = $runtimeSourceReleaseTag end' \
  >"$MANIFEST_PATH"

note "已生成：$MANIFEST_PATH"
