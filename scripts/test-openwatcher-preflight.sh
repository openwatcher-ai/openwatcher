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

search_pattern() {
  local pattern="$1"
  local file="$2"
  if command -v rg >/dev/null 2>&1; then
    rg -n "$pattern" "$file" >/dev/null
  else
    grep -RInE "$pattern" "$file" >/dev/null
  fi
}

run() {
  note "$*"
  "$@"
}

root_go_packages() {
  (
    cd "$ROOT_DIR" &&
      go list ./... | grep -vE '^openwatcher/desktop-app(/|$)'
  )
}

scan_for_regressions() {
  note "运行公开仓残留扫描"
  "$ROOT_DIR/scripts/scan-openwatcher-public-tree.sh"
}

run_go_tests() {
  note "运行根 Go 测试"
  local packages=()
  while IFS= read -r package; do
    [[ -n "$package" ]] && packages+=("$package")
  done <<EOF
$(root_go_packages)
EOF
  [[ ${#packages[@]} -gt 0 ]] || die "未找到根 Go 包"
  (cd "$ROOT_DIR" && go test "${packages[@]}")

  note "运行 Desktop Go 测试"
  (cd "$ROOT_DIR/desktop-app" && go test ./...)
}

run_optional_headless_e2e() {
  local e2e_dir="$ROOT_DIR/desktop-app/internal/e2e"
  if [[ ! -d "$e2e_dir" ]]; then
    note "Desktop headless E2E 尚未实现，跳过 go test ./desktop-app/internal/e2e -run TestDesktopHeadless"
    return 0
  fi

  search_pattern 'func TestDesktopHeadless' "$e2e_dir" \
    || die "desktop-app/internal/e2e 已存在，但缺少 TestDesktopHeadless"
  note "运行 Desktop headless E2E"
  (cd "$ROOT_DIR" && go test ./desktop-app/internal/e2e -run TestDesktopHeadless)
}

run_watch_unit_tests() {
  note "运行 Watch JVM 单元测试"
  local gradle_args=(--no-daemon testDebugUnitTest)
  if [[ "${CI:-}" == "true" ]]; then
    gradle_args+=(--info)
  fi
  (cd "$ROOT_DIR/watch-app" && ./gradlew "${gradle_args[@]}")
}

main() {
  require_command go
  require_command java

  scan_for_regressions
  run_go_tests
  run_optional_headless_e2e
  run_watch_unit_tests

  note "OpenWatcher preflight 通过"
}

main "$@"
