#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/release-common.sh"

PLATFORM="${OPENWATCHER_DESKTOP_PLATFORM:-$(current_platform_id)}"
SOURCE_PATH="${OPENWATCHER_DESKTOP_SOURCE:-}"
SKIP_BUILD="${OPENWATCHER_DESKTOP_SKIP_BUILD:-0}"
WAILS_VERSION="${OPENWATCHER_WAILS_VERSION:-v2.12.0}"
CHANNEL_MANIFEST_URL="${OPENWATCHER_RUNTIME_CHANNEL_MANIFEST_URL:-}"
REPOSITORY_NAME="${OPENWATCHER_GITHUB_REPOSITORY:-${GITHUB_REPOSITORY:-openwatcher-ai/openwatcher}}"
DESKTOP_VERSION="$(require_desktop_version)"
DESKTOP_ARTIFACT_VERSION="$(release_filename_version "$DESKTOP_VERSION")"

declare -a FINAL_OUTPUTS=()
SUCCEEDED=0
WAILS_CONFIG_BACKUP=""

restore_wails_config() {
  if [[ -n "$WAILS_CONFIG_BACKUP" && -f "$WAILS_CONFIG_BACKUP" ]]; then
    mv "$WAILS_CONFIG_BACKUP" "$ROOT_DIR/desktop-app/wails.json"
    WAILS_CONFIG_BACKUP=""
  fi
}

cleanup_partial_output() {
  if [[ "$SUCCEEDED" == "1" ]]; then
    return
  fi
  for output in "${FINAL_OUTPUTS[@]}"; do
    rm -rf "$output"
  done
}

on_exit() {
  local status=$?
  restore_wails_config
  cleanup_partial_output
  exit "$status"
}
trap on_exit EXIT

while [[ $# -gt 0 ]]; do
  case "$1" in
    --platform)
      PLATFORM="$2"
      shift 2
      ;;
    --source)
      SOURCE_PATH="$2"
      shift 2
      ;;
    --skip-build)
      SKIP_BUILD=1
      shift
      ;;
    *)
      die "未知参数：$1"
      ;;
  esac
done

read -r GOOS_VALUE GOARCH_VALUE <<<"$(platform_go_values "$PLATFORM")"
ensure_release_dir
require_command go

if [[ -z "$CHANNEL_MANIFEST_URL" ]]; then
  CHANNEL_MANIFEST_URL="https://openwatcher.ai/channels/beta.json"
fi

validate_lightweight_bundle() {
  local bundle_root="$1"
  local platform="$2"
  local sidecar_name="openwatcher"
  local updater_name="openwatcher-updater"
  if [[ "$GOOS_VALUE" == "windows" ]]; then
    sidecar_name+=".exe"
    updater_name+=".exe"
  fi
  [[ -f "$bundle_root/openwatcher/$platform/$sidecar_name" ]] || die "bundled 内缺少 sidecar：$bundle_root/openwatcher/$platform/$sidecar_name"
  [[ -f "$bundle_root/updater/$platform/$updater_name" ]] || die "bundled 内缺少 updater：$bundle_root/updater/$platform/$updater_name"
  [[ -f "$bundle_root/runtime/channel-manifest-url.txt" ]] || die "bundled 内缺少 channel manifest 地址配置"
  for forbidden_dir in platform-tools cloudflared watch-apk; do
    if [[ -d "$bundle_root/$forbidden_dir" ]]; then
      die "Desktop 轻安装包不应再包含 $forbidden_dir：$bundle_root/$forbidden_dir"
    fi
  done
}

ensure_desktop_app_icon() {
  local source_icon="$ROOT_DIR/desktop-app/frontend/src/assets/openwatcher_launcher.png"
  local target_icon="$ROOT_DIR/desktop-app/build/appicon.png"
  [[ -f "$source_icon" ]] || die "缺少 Desktop app 图标源：$source_icon"
  mkdir -p "$(dirname "$target_icon")"
  cp "$source_icon" "$target_icon"
}

ensure_desktop_macos_info_plist() {
  local plist_path="$ROOT_DIR/desktop-app/build/darwin/Info.plist"
  mkdir -p "$(dirname "$plist_path")"
  cat > "$plist_path" <<'PLIST'
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
    <dict>
        <key>CFBundlePackageType</key>
        <string>APPL</string>
        <key>CFBundleName</key>
        <string>{{.Info.ProductName}}</string>
        <key>CFBundleDisplayName</key>
        <string>{{.Info.ProductName}}</string>
        <key>CFBundleExecutable</key>
        <string>{{.OutputFilename}}</string>
        <key>CFBundleIdentifier</key>
        <string>ai.openwatcher.desktop</string>
        <key>CFBundleVersion</key>
        <string>{{.Info.ProductVersion}}</string>
        <key>CFBundleGetInfoString</key>
        <string>{{.Info.Comments}}</string>
        <key>CFBundleShortVersionString</key>
        <string>{{.Info.ProductVersion}}</string>
        <key>CFBundleIconFile</key>
        <string>iconfile</string>
        <key>LSMinimumSystemVersion</key>
        <string>10.13.0</string>
        <key>NSHighResolutionCapable</key>
        <string>true</string>
        <key>NSHumanReadableCopyright</key>
        <string>{{.Info.Copyright}}</string>
        {{if .Info.FileAssociations}}
        <key>CFBundleDocumentTypes</key>
        <array>
          {{range .Info.FileAssociations}}
          <dict>
            <key>CFBundleTypeExtensions</key>
            <array>
              <string>{{.Ext}}</string>
            </array>
            <key>CFBundleTypeName</key>
            <string>{{.Name}}</string>
            <key>CFBundleTypeRole</key>
            <string>{{.Role}}</string>
            <key>CFBundleTypeIconFile</key>
            <string>{{.IconName}}</string>
          </dict>
          {{end}}
        </array>
        {{end}}
        {{if .Info.Protocols}}
        <key>CFBundleURLTypes</key>
        <array>
          {{range .Info.Protocols}}
            <dict>
                <key>CFBundleURLName</key>
                <string>ai.openwatcher.{{.Scheme}}</string>
                <key>CFBundleURLSchemes</key>
                <array>
                    <string>{{.Scheme}}</string>
                </array>
                <key>CFBundleTypeRole</key>
                <string>{{.Role}}</string>
            </dict>
          {{end}}
        </array>
        {{end}}
    </dict>
</plist>
PLIST
}

prepare_wails_project_version() {
  local config_path="$ROOT_DIR/desktop-app/wails.json"
  local backup_path
  require_command node
  backup_path="$(mktemp "${TMPDIR:-/tmp}/openwatcher-wails-json.XXXXXX")"
  cp "$config_path" "$backup_path"
  WAILS_CONFIG_BACKUP="$backup_path"
  node - "$config_path" "$DESKTOP_VERSION" <<'NODE'
const fs = require("fs")
const configPath = process.argv[2]
const productVersion = process.argv[3]
const config = JSON.parse(fs.readFileSync(configPath, "utf8"))
config.info = {
  ...(config.info || {}),
  productName: config.info?.productName || config.name || "OpenWatcher",
  productVersion
}
fs.writeFileSync(configPath, `${JSON.stringify(config, null, 2)}\n`)
NODE
}

build_windows_setup() {
  local setup_path="$RELEASE_DIR/desktop_${DESKTOP_ARTIFACT_VERSION}_$(artifact_platform_label "$PLATFORM").exe"
  local setup_path_win
  local installer_dir="$ROOT_DIR/.tmp/nsis-${PLATFORM}"
  local script_path="$installer_dir/openwatcher-installer.nsi"
  local binary_path_win
  local bundled_dir_win
  rm -rf "$installer_dir"
  mkdir -p "$installer_dir"
  require_command makensis
  setup_path_win="$(cygpath -w "$setup_path")"
  binary_path_win="$(cygpath -w "$ROOT_DIR/desktop-app/build/bin/openwatcher.exe")"
  bundled_dir_win="$(cygpath -w "$ROOT_DIR/desktop-app/build/bin/bundled")"
  cat > "$script_path" <<NSI
Unicode True
ManifestDPIAware true
Name "OpenWatcher"
OutFile "$setup_path_win"
InstallDir "\$LOCALAPPDATA\\OpenWatcher"
RequestExecutionLevel user
Page directory
Page instfiles
UninstPage uninstConfirm
UninstPage instfiles

Section "OpenWatcher"
  SetOutPath "\$INSTDIR"
  File "$binary_path_win"
  File /r "$bundled_dir_win"
  WriteUninstaller "\$INSTDIR\\Uninstall.exe"
SectionEnd

Section "Uninstall"
  Delete "\$INSTDIR\\openwatcher.exe"
  Delete "\$INSTDIR\\Uninstall.exe"
  RMDir /r "\$INSTDIR\\bundled"
  RMDir "\$INSTDIR"
SectionEnd
NSI
  makensis "$script_path" >/dev/null
  FINAL_OUTPUTS+=("$setup_path")
}

build_windows_portable_zip() {
  local zip_path="$RELEASE_DIR/desktop_${DESKTOP_ARTIFACT_VERSION}_$(artifact_platform_label "$PLATFORM").zip"
  rm -f "$zip_path"
  if command -v zip >/dev/null 2>&1; then
    (
      cd "$ROOT_DIR/desktop-app/build/bin"
      zip -qry "$zip_path" openwatcher.exe bundled
    )
  elif command -v 7z >/dev/null 2>&1; then
    (
      cd "$ROOT_DIR/desktop-app/build/bin"
      7z a -tzip "$zip_path" openwatcher.exe bundled >/dev/null
    )
  elif command -v powershell.exe >/dev/null 2>&1; then
    local source_dir_win
    local zip_path_win
    source_dir_win="$(cygpath -w "$ROOT_DIR/desktop-app/build/bin")"
    zip_path_win="$(cygpath -w "$zip_path")"
    powershell.exe -NoProfile -Command \
      "Set-Location '$source_dir_win'; Compress-Archive -Path 'openwatcher.exe','bundled' -DestinationPath '$zip_path_win' -Force" >/dev/null
  else
    die "缺少 zip、7z 和 powershell.exe，无法生成 Windows 绿色版 zip"
  fi
  FINAL_OUTPUTS+=("$zip_path")
}

build_macos_outputs() {
  local app_source="$1"
  local app_copy="$ROOT_DIR/.tmp/OpenWatcher.app"
  local platform_label
  platform_label="$(artifact_platform_label "$PLATFORM")"
  local zip_path="$RELEASE_DIR/desktop_${DESKTOP_ARTIFACT_VERSION}_${platform_label}.zip"
  local dmg_path="$RELEASE_DIR/desktop_${DESKTOP_ARTIFACT_VERSION}_${platform_label}.dmg"

  mkdir -p "$ROOT_DIR/.tmp"
  rm -rf "$app_copy" "$zip_path" "$dmg_path"
  cp -R "$app_source" "$app_copy"
  if command -v xattr >/dev/null 2>&1; then
    xattr -cr "$app_copy" || true
  fi
  COPYFILE_DISABLE=1 ditto --norsrc --noextattr --noqtn --noacl -c -k --keepParent "$app_copy" "$zip_path"
  hdiutil create -volname "OpenWatcher" -srcfolder "$app_copy" -ov -format UDZO "$dmg_path" >/dev/null
  rm -rf "$app_copy"
  FINAL_OUTPUTS+=("$zip_path" "$dmg_path")
}

if [[ -z "$SOURCE_PATH" && "$SKIP_BUILD" != "1" ]]; then
  note "调用 Wails 构建 Desktop 包：$PLATFORM"
  ensure_desktop_app_icon
  prepare_wails_project_version
  if [[ "$GOOS_VALUE" == "darwin" ]]; then
    ensure_desktop_macos_info_plist
  fi
  (
    cd "$ROOT_DIR/desktop-app"
    GOFLAGS="-trimpath" \
      OPENWATCHER_RUNTIME_CHANNEL_MANIFEST_URL="$CHANNEL_MANIFEST_URL" \
      OPENWATCHER_GITHUB_REPOSITORY="$REPOSITORY_NAME" \
      OPENWATCHER_BUNDLE_PLATFORM="$PLATFORM" \
      go run github.com/wailsapp/wails/v2/cmd/wails@"$WAILS_VERSION" build \
        -platform "$GOOS_VALUE/$GOARCH_VALUE" \
        -ldflags "-X main.desktopProductVersion=$DESKTOP_VERSION"
  )
  restore_wails_config
fi

if [[ -z "$SOURCE_PATH" ]]; then
  case "$GOOS_VALUE" in
    darwin)
      SOURCE_PATH="$ROOT_DIR/desktop-app/build/bin/OpenWatcher.app"
      ;;
    windows)
      SOURCE_PATH="$ROOT_DIR/desktop-app/build/bin/openwatcher.exe"
      ;;
    *)
      die "当前未配置该平台的默认 Desktop 产物路径：$PLATFORM"
      ;;
  esac
fi

[[ -e "$SOURCE_PATH" ]] || die "Desktop 产物不存在：$SOURCE_PATH"

if [[ "$GOOS_VALUE" == "darwin" ]]; then
  validate_lightweight_bundle "$SOURCE_PATH/Contents/Resources/bundled" "$PLATFORM"
  build_macos_outputs "$SOURCE_PATH"
else
  validate_lightweight_bundle "$ROOT_DIR/desktop-app/build/bin/bundled" "$PLATFORM"
  build_windows_setup
  build_windows_portable_zip
fi

for output in "${FINAL_OUTPUTS[@]}"; do
  note "已生成 Desktop 发布产物：$output"
done
SUCCEEDED=1
printf '%s\n' "${FINAL_OUTPUTS[@]}"
