#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

note() {
  printf '\n==> %s\n' "$*" >&2
}

die() {
  printf '%s\n' "$*" >&2
  exit 2
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "缺少命令：$1"
}

list_files() {
  if git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1 && [[ -n "$(cd "$ROOT_DIR" && git ls-files)" ]]; then
    (cd "$ROOT_DIR" && git ls-files)
    return
  fi
  if command -v rg >/dev/null 2>&1; then
    (cd "$ROOT_DIR" && rg --files -g '*')
    return
  fi
  (
    cd "$ROOT_DIR"
    find . -type f | sed 's#^\./##' | sort
  )
}

match_paths() {
  local pattern="$1"
  if command -v rg >/dev/null 2>&1; then
    rg -n "$pattern"
  else
    grep -nE "$pattern"
  fi
}

search_tree() {
  local pattern="$1"
  shift
  if command -v rg >/dev/null 2>&1; then
    rg -n "$pattern" "$@"
  else
    local paths=()
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --glob)
          shift 2
          ;;
        *)
          paths+=("$1")
          shift
          ;;
      esac
    done
    grep -RInE "$pattern" "${paths[@]}"
  fi
}

scan_tracked_paths() {
  note "检查公开仓已跟踪路径是否混入私有目录"

  local forbidden_paths='^(site/|workers/|\.agent-handoff/|docs/superpowers/|docs/references/|docs/desktop-prototype/|menu-app/|artifacts/|dist/|build/|bin/|\.tmp/|watch-app/signing/|desktop-app/bundled/|scripts/install-menu-app\.sh$|docs/watch-app-round-ui-polish-guidelines\.md$|docs/openwatcher-publication-docs/08-agent-task-prompts\.md$|docs/openwatcher-publication-docs/11-real-managed-tunnel-worker-control-plane\.md$|docs/openwatcher-publication-docs/12-open-source-migration-and-platform-sync-plan\.md$)'
  if list_files | match_paths "$forbidden_paths"; then
    die "公开仓仍包含不应跟踪的私有目录或内部文档"
  fi
}

scan_public_content() {
  note "检查公开仓文本内容残留"

  local scan_paths=(
    README.md
    CHANGELOG.md
    .github/workflows
    cmd
    desktop-app
    internal
    scripts
    testsupport
    watch-app
  )
  local forbidden_pattern='watcher\.uuss\.top|top\.uuss|Codex Watcher|CODEX_WATCHER|X-Codex-Watcher|/Users/|loccen/codex-watcher|openwatcher-pub-pre|menu-app'
  if (
    cd "$ROOT_DIR"
    search_tree "$forbidden_pattern" \
      --glob '!watch-app/RELEASE_BUILDS.md' \
      --glob '!scripts/check-release-artifacts.sh' \
      --glob '!scripts/scan-openwatcher-public-tree.sh' \
      "${scan_paths[@]}"
  ); then
    die "公开仓文本仍包含旧仓库名、旧品牌、个人路径或已排除模块残留"
  fi
}

scan_release_surface() {
  note "检查公开仓 release 默认地址与版本来源"

  rg -n 'OPENWATCHER_BETA_UPDATE_PRIMARY_URL", "https://openwatcher\.ai/channels/beta\.json"' "$ROOT_DIR/watch-app/app/build.gradle.kts" >/dev/null \
    || die "watch-app beta 更新主入口未指向 openwatcher.ai channel"
  if rg -n 'api\.github\.com/repos|openWatcherGithubRepository|OPENWATCHER_BETA_UPDATE_BACKUP_URL", "https?://' "$ROOT_DIR/watch-app/app/build.gradle.kts" >/dev/null; then
    die "watch-app beta 更新检查不应继续默认使用 GitHub Releases 备用入口"
  fi
  if rg -n 'const[[:space:]]+desktopProductVersion[[:space:]]*=|var[[:space:]]+desktopProductVersion[[:space:]]*=[[:space:]]*"[0-9]+\.[0-9]+\.[0-9]+' "$ROOT_DIR/desktop-app/app.go" >/dev/null; then
    die "Desktop 产品版本不得在 Go 源码中写死，请通过 OPENWATCHER_DESKTOP_VERSION 构建注入"
  fi
  if rg -n 'versionName[[:space:]]*=[[:space:]]*"[0-9]+\.[0-9]+\.[0-9]+|versionCode[[:space:]]*=[[:space:]]*[0-9]+' "$ROOT_DIR/watch-app/app/build.gradle.kts" >/dev/null; then
    die "Watch versionName/versionCode 不得在 Gradle 文件中写死，请通过 OPENWATCHER_WATCH_VERSION_NAME/OPENWATCHER_WATCH_VERSION_CODE 构建注入"
  fi
  if rg -n '"productVersion"[[:space:]]*:[[:space:]]*"[0-9]+\.[0-9]+\.[0-9]+"' "$ROOT_DIR/desktop-app/wails.json" >/dev/null; then
    die "Wails productVersion 不得写入 wails.json，请由 Desktop 打包脚本临时注入"
  fi
  if rg -n 'Version[[:space:]]*=[[:space:]]*"[0-9]+\.[0-9]+\.[0-9]+' "$ROOT_DIR/internal/buildinfo/buildinfo.go" >/dev/null; then
    die "本机服务 buildinfo.Version 不得写死，请构建时注入或使用 dev 标识"
  fi
}

main() {
  require_command git
  scan_tracked_paths
  scan_public_content
  scan_release_surface
  note "公开仓残留扫描通过"
}

main "$@"
