#!/bin/zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
APP_DIR="$ROOT_DIR/watch-app"
ADB="${ADB:-}"
SERIAL="${1:-emulator-5554}"
PACKAGE="${PACKAGE:-ai.openwatcher.watchapp.debug}"
ACTIVITY="${ACTIVITY:-$PACKAGE/ai.openwatcher.watchapp.MainActivity}"
APK_PATH="$APP_DIR/app/build/outputs/apk/debug/app-debug.apk"
OUT_DIR="$ROOT_DIR/artifacts/screenshots"
MODE="${MODE:-settings}"
SKIP_GRADLE_BUILD="${SKIP_GRADLE_BUILD:-0}"
BOOTSTRAP_BASE_URL="${BOOTSTRAP_BASE_URL:-http://10.0.2.2:18787}"
BOOTSTRAP_DEVICE_TOKEN="${BOOTSTRAP_DEVICE_TOKEN:-test-token-0123456789abcdef0123456789}"
BOOTSTRAP_DEVICE_NAME="${BOOTSTRAP_DEVICE_NAME:-watch}"
BOOTSTRAP_OUT_NAME="${BOOTSTRAP_OUT_NAME:-bootstrap-confirm.png}"
BOOTSTRAP_WAIT_SECONDS="${BOOTSTRAP_WAIT_SECONDS:-1}"

mkdir -p "$OUT_DIR"

if [[ -z "$ADB" ]]; then
  if command -v adb >/dev/null 2>&1; then
    ADB="$(command -v adb)"
  elif [[ -x "$HOME/Library/Android/sdk/platform-tools/adb" ]]; then
    ADB="$HOME/Library/Android/sdk/platform-tools/adb"
  elif [[ -n "${ANDROID_SDK_ROOT:-}" && -x "${ANDROID_SDK_ROOT}/platform-tools/adb" ]]; then
    ADB="${ANDROID_SDK_ROOT}/platform-tools/adb"
  elif [[ -n "${ANDROID_HOME:-}" && -x "${ANDROID_HOME}/platform-tools/adb" ]]; then
    ADB="${ANDROID_HOME}/platform-tools/adb"
  else
    echo "找不到 adb。请先把 adb 加入 PATH，或设置 ADB / ANDROID_SDK_ROOT / ANDROID_HOME。" >&2
    exit 1
  fi
fi

if [[ ! -f "$APK_PATH" ]]; then
  SKIP_GRADLE_BUILD=0
fi

if [[ "$SKIP_GRADLE_BUILD" != "1" ]]; then
  (
    cd "$APP_DIR"
    ANDROID_HOME="${ANDROID_HOME:-$HOME/Library/Android/sdk}" \
    ANDROID_SDK_ROOT="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$HOME/Library/Android/sdk}}" \
    ./gradlew assembleDebug >/dev/null
  )
fi

launch() {
  local scenario="$1"
  shift || true
  "$ADB" -s "$SERIAL" shell am force-stop "$PACKAGE" >/dev/null
  if [[ -n "$scenario" ]]; then
    "$ADB" -s "$SERIAL" shell am start \
      -a android.intent.action.MAIN \
      -c android.intent.category.LAUNCHER \
      -n "$ACTIVITY" \
      --es openwatcher_debug_scenario "$scenario" \
      "$@" >/dev/null
  else
    "$ADB" -s "$SERIAL" shell am start \
      -a android.intent.action.MAIN \
      -c android.intent.category.LAUNCHER \
      -n "$ACTIVITY" \
      "$@" >/dev/null
  fi
}

screenshot() {
  local file="$1"
  "$ADB" -s "$SERIAL" exec-out screencap -p > "$file"
}

wait_for_app_ui() {
  local timeout_seconds="${1:-20}"
  local started_at
  started_at="$(date +%s)"
  while true; do
    local top
    top="$("$ADB" -s "$SERIAL" shell dumpsys activity top 2>/dev/null || true)"
    local window
    window="$("$ADB" -s "$SERIAL" shell uiautomator dump /sdcard/window_dump.xml >/dev/null 2>&1; "$ADB" -s "$SERIAL" shell cat /sdcard/window_dump.xml 2>/dev/null || true)"
    if [[ "$top" == *"ai.openwatcher.watchapp.MainActivity"* && "$window" == *"package=\"$PACKAGE\""* ]]; then
      return 0
    fi
    if (( $(date +%s) - started_at >= timeout_seconds )); then
      echo "等待应用前台 UI 超时：$PACKAGE" >&2
      return 1
    fi
    sleep 1
  done
}

wait_for_preview() {
  sleep 3
}

swipe_percent() {
  local start_x_percent="$1"
  local start_y_percent="$2"
  local end_x_percent="$3"
  local end_y_percent="$4"
  local duration_ms="${5:-250}"
  local size
  size="$("$ADB" -s "$SERIAL" shell wm size | awk -F': ' 'NR==1 {print $2}')"
  local width="${size%x*}"
  local height="${size#*x}"
  local start_x=$(( width * start_x_percent / 100 ))
  local start_y=$(( height * start_y_percent / 100 ))
  local end_x=$(( width * end_x_percent / 100 ))
  local end_y=$(( height * end_y_percent / 100 ))
  "$ADB" -s "$SERIAL" shell input swipe "$start_x" "$start_y" "$end_x" "$end_y" "$duration_ms" >/dev/null
}

"$ADB" -s "$SERIAL" install -r "$APK_PATH" >/dev/null
"$ADB" -s "$SERIAL" shell pm clear "$PACKAGE" >/dev/null

if [[ "$MODE" == "bootstrap" ]]; then
  BOOTSTRAP_URI="$(python3 - "$BOOTSTRAP_BASE_URL" "$BOOTSTRAP_DEVICE_TOKEN" "$BOOTSTRAP_DEVICE_NAME" <<'PY'
import sys, urllib.parse
base = sys.argv[1]
token = sys.argv[2]
name = sys.argv[3]
params = {
    "baseUrl": base,
    "deviceToken": token,
    "deviceName": name,
}
print("openwatcher://bootstrap?" + urllib.parse.urlencode(params))
PY
)"
  "$ADB" -s "$SERIAL" shell am force-stop "$PACKAGE" >/dev/null
  "$ADB" -s "$SERIAL" shell "am start -W -a android.intent.action.VIEW -d '$BOOTSTRAP_URI'" >/dev/null
  sleep "$BOOTSTRAP_WAIT_SECONDS"
  screenshot "$OUT_DIR/$BOOTSTRAP_OUT_NAME"
  echo "bootstrap screenshot updated in $OUT_DIR/$BOOTSTRAP_OUT_NAME"
  exit 0
fi

launch "DASHBOARD" \
  --ez openwatcher_debug_open_settings true \
  --es openwatcher_debug_settings_destination root
wait_for_app_ui
wait_for_preview
screenshot "$OUT_DIR/settings-root.png"

launch "DASHBOARD" \
  --ez openwatcher_debug_open_settings true \
  --es openwatcher_debug_settings_destination service_status
wait_for_app_ui
wait_for_preview
screenshot "$OUT_DIR/settings-service-status.png"

launch "DASHBOARD" \
  --ez openwatcher_debug_open_settings true \
  --es openwatcher_debug_settings_destination update
wait_for_app_ui
wait_for_preview
screenshot "$OUT_DIR/settings-update-check.png"

launch "DASHBOARD" \
  --ez openwatcher_debug_open_settings true \
  --es openwatcher_debug_settings_destination update_notes \
  --es openwatcher_debug_update_preview available \
  --ez openwatcher_debug_install_permission_enabled true
wait_for_app_ui
wait_for_preview
swipe_percent 50 78 50 40 220
sleep 1
screenshot "$OUT_DIR/settings-update-notes-available.png"

launch "DASHBOARD" \
  --ez openwatcher_debug_open_settings true \
  --es openwatcher_debug_settings_destination current_version_notes \
  --ez openwatcher_debug_install_permission_enabled true
wait_for_app_ui
wait_for_preview
screenshot "$OUT_DIR/settings-current-version-notes.png"

echo "screenshots updated in $OUT_DIR"
