#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/scripts/release-common.sh"

RELEASE_DIR="${OPENWATCHER_PUBLIC_RELEASE_DIR:-$ROOT_DIR/dist/public-release}"
RELEASE_MANIFEST_PATH="${OPENWATCHER_RELEASE_MANIFEST_PATH:-$RELEASE_DIR/release-manifest.json}"
CHANGELOG_ENTRY_PATH="${OPENWATCHER_CHANGELOG_ENTRY_PATH:-$RELEASE_DIR/changelog-entry.json}"

require_command jq
[[ -f "$RELEASE_MANIFEST_PATH" ]] || die "缺少 release-manifest.json：$RELEASE_MANIFEST_PATH"

jq -n \
  --slurpfile manifest "$RELEASE_MANIFEST_PATH" \
  --arg releaseManifestUrl "https://github.com/$(jq -r '.product.repository' "$RELEASE_MANIFEST_PATH")/releases/download/$(jq -r '.release.tag' "$RELEASE_MANIFEST_PATH")/release-manifest.json" \
  'def componentLabel(key):
      if key == "desktop" then "桌面应用"
      elif key == "watch" then "手表应用"
      elif key == "runtime" then "运行时依赖"
      elif key == "compatibility" then "兼容性"
      elif key == "docs" then "文档"
      else key
      end;
   def compatibilityText(key; value):
      if key == "desktop" and value.status == "reused" then "本次未更新桌面应用，继续复用上一版桌面安装包。"
      elif key == "watch" and value.status == "reused" then "本次未更新手表应用，继续复用上一版 Watch APK，不会触发手表端版本更新。"
      elif key == "runtime" and value.status == "reused" then "本次继续复用当前 Runtime Release。"
      elif key == "runtime" and value.status == "updated" then "本次更新 beta 通道指向的 Runtime Release。"
      elif key == "docs" and value.status == "updated" then "本次包含公开文档更新。"
      elif key == "compatibility" and value.status == "updated" then "本次包含兼容性或升级说明更新。"
      else empty
      end;
   def releaseSummaryCategory(summary):
      if summary == "" then empty
      elif (summary | test("^\\s*(修复|解决|更正)")) then "fixes"
      elif (summary | test("^\\s*(新增|增加|支持|加入)")) then "features"
      else "improvements"
      end;
   def updatedReleaseSummaryNotes(m; category):
      (m.release.summary // "") as $summary
      | (releaseSummaryCategory($summary)) as $summaryCategory
      | if $summary == "" or $summaryCategory != category then []
        else [
          m.components
          | to_entries[]
          | select(.value.status == "updated")
          | select(.key != "metadata")
          | {
              component: componentLabel(.key),
              text: $summary
            }
        ]
        end;
   ($manifest[0]) as $m
  | {
      schemaVersion: 1,
      channel: "beta",
      id: $m.release.tag,
      publishedAt: $m.release.publishedAt,
      scope: $m.release.scope,
      components: $m.components,
      notes: {
        features: updatedReleaseSummaryNotes($m; "features"),
        improvements: updatedReleaseSummaryNotes($m; "improvements"),
        fixes: updatedReleaseSummaryNotes($m; "fixes"),
        compatibility: [
          ($m.components | to_entries[] | compatibilityText(.key; .value)) as $text
          | {
              component: "兼容性",
              text: $text
            }
        ]
      },
      links: {
        releaseUrl: "https://github.com/\($m.product.repository)/releases/tag/\($m.release.tag)",
        releaseManifestUrl: $releaseManifestUrl
      }
    }' >"$CHANGELOG_ENTRY_PATH"

note "已生成：$CHANGELOG_ENTRY_PATH"
