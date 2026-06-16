#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/release-common.sh"

ensure_release_dir
require_command go

WAILS_VERSION="$(cd "$ROOT_DIR" && go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2)"
require_command python3
VITE_VERSION="$(
  python3 - <<'PY' "$ROOT_DIR/desktop-app/frontend/package.json"
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    data = json.load(handle)

value = ""
for key in ("devDependencies", "dependencies"):
    section = data.get(key)
    if isinstance(section, dict) and isinstance(section.get("vite"), str):
        value = section["vite"]
        break

print(value.lstrip("^"))
PY
)"
ADB_VERSION="$(
  {
    if [[ -n "${ANDROID_HOME:-}" && -x "${ANDROID_HOME}/platform-tools/adb" ]]; then
      "${ANDROID_HOME}/platform-tools/adb" version 2>/dev/null | head -1
    elif [[ -n "${ANDROID_SDK_ROOT:-}" && -x "${ANDROID_SDK_ROOT}/platform-tools/adb" ]]; then
      "${ANDROID_SDK_ROOT}/platform-tools/adb" version 2>/dev/null | head -1
    elif [[ -x "$HOME/Library/Android/sdk/platform-tools/adb" ]]; then
      "$HOME/Library/Android/sdk/platform-tools/adb" version 2>/dev/null | head -1
    fi
  } | sed 's/^Android Debug Bridge version //'
)"
if [[ -z "$ADB_VERSION" ]]; then
  ADB_VERSION="按打包时实际复制的 platform-tools 为准"
fi
CLOUDFLARED_VERSION="$(
  if command -v cloudflared >/dev/null 2>&1; then
    cloudflared --version 2>/dev/null | head -1 | sed 's/^cloudflared version //'
  fi
)"

NOTICES_PATH="$RELEASE_DIR/THIRD_PARTY_NOTICES.md"

cat >"$NOTICES_PATH" <<EOF
# THIRD_PARTY_NOTICES

本文件为 OpenWatcher 公开发布产物的第三方组件说明模板。正式发布时，请结合当前打包结果再次核对版本和 license 文件是否齐全。

## 桌面端

### Wails

- 版本：${WAILS_VERSION}
- 用途：OpenWatcher Desktop 桌面应用壳与打包框架
- 上游：<https://github.com/wailsapp/wails>
- License：MIT

### Vite

- 版本：${VITE_VERSION:-unknown}
- 用途：Desktop 前端构建
- 上游：<https://vite.dev/>
- License：MIT

## 平台工具

### Android platform-tools / ADB

- 版本：${ADB_VERSION}
- 用途：无线调试、设备发现、APK 安装和启动
- 上游：<https://developer.android.com/tools/releases/platform-tools>
- License：Android SDK / platform-tools 对应上游条款

### cloudflared

- 版本：${CLOUDFLARED_VERSION:-按打包时实际复制的 cloudflared 为准}
- 用途：OpenWatcher 托管隧道 Desktop 本地连接器
- 上游：<https://github.com/cloudflare/cloudflared>
- License：Apache-2.0

## Go 依赖

当前后端和 Desktop sidecar 依赖以仓库根 go.mod / go.sum 为准。正式发布时，建议同步归档：

- go list -m all
- 关键三方组件的上游主页
- 各组件 license 类型

## Android / Gradle 依赖

当前手表端依赖以 watch-app/ 下 Gradle 配置与锁定结果为准。正式发布手表 APK 时，请确保：

- Gradle 依赖列表可复现
- Android 第三方 license 文件未遗漏

## 维护说明

- 本文件由 scripts/generate-third-party-notices.sh 生成。
- 如果发布包新增了二进制或 SaaS 组件，请在正式发布前补充实际版本、来源和 license。
EOF

note "已生成：$NOTICES_PATH"
