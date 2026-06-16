---
name: openwatcher-desktop-install
description: 在 macOS 或 Windows 上为用户下载、校验、安装并启动最新 OpenWatcher Desktop。适用于安装 OpenWatcher Desktop、从 beta channel 获取最新桌面安装包、处理 macOS 未签名未公证技术预览包的 quarantine 限制、静默运行 Windows 安装器、创建 Windows 桌面快捷方式或打开已安装应用。
---

# OpenWatcher Desktop 安装

## 概览

使用本 Skill 在用户电脑上安装 OpenWatcher Desktop。优先通过脚本执行可重复的下载、校验、安装和启动动作，不手写临时命令替代脚本。

## 工作流程

1. 判断当前系统。
   - macOS 使用 `scripts/install-openwatcher-macos.sh`。
   - Windows 使用 `scripts/install-openwatcher-windows.ps1`。
   - 其他系统直接说明暂不支持。
2. 从 beta channel manifest 读取最新 Desktop 安装包：
   - 默认入口：`https://openwatcher.ai/channels/beta.json`
   - 只在用户明确指定测试入口时使用其他 channel URL。
3. 按本机平台选择 manifest 中的 `desktop.platforms`：
   - macOS Apple Silicon：`darwin-arm64`
   - macOS Intel：`darwin-amd64`
   - Windows x64：`windows-amd64`
   - Windows ARM64：优先 `windows-arm64`；manifest 缺失时脚本会退到 `windows-amd64` 并明确输出。
4. 下载安装包并校验 `sha256`。缺少 sha 或校验不一致时停止安装。
5. 执行平台安装动作。
6. 启动 OpenWatcher Desktop，并向用户报告版本、release tag、平台、安装路径和校验结果。

## macOS 行为

运行：

```bash
.codex/skills/openwatcher-desktop-install/scripts/install-openwatcher-macos.sh
```

脚本会：

- 支持 `.dmg` 和 `.zip` Desktop 包。
- 安装到 `/Applications/OpenWatcher.app`。
- 如已存在同名应用，默认覆盖该应用。
- 只对 `/Applications/OpenWatcher.app` 执行 `xattr -dr com.apple.quarantine`。
- 使用 `open /Applications/OpenWatcher.app` 启动应用。

不得关闭全局 Gatekeeper，不得执行 `spctl --master-disable`，不得对 OpenWatcher 之外的应用移除 quarantine。

## Windows 行为

运行：

```powershell
powershell -ExecutionPolicy Bypass -File .codex\skills\openwatcher-desktop-install\scripts\install-openwatcher-windows.ps1
```

脚本会：

- 优先安装 NSIS `Setup.exe`，也兼容 `.zip` 绿色包。
- 使用当前用户权限静默安装，不要求管理员权限。
- 默认安装到 `%LOCALAPPDATA%\OpenWatcher`。
- 检查 `%LOCALAPPDATA%\OpenWatcher\openwatcher.exe`。
- 创建当前用户桌面快捷方式 `OpenWatcher.lnk`。
- 启动 `openwatcher.exe`。

## 验证和排错

在改动脚本或排查问题时先运行非破坏性检查：

```bash
.codex/skills/openwatcher-desktop-install/scripts/install-openwatcher-macos.sh --help
bash -n .codex/skills/openwatcher-desktop-install/scripts/install-openwatcher-macos.sh
```

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .codex\skills\openwatcher-desktop-install\scripts\install-openwatcher-windows.ps1 -Help
```

如果安装失败，保留脚本输出中的失败阶段并说明原因。常见失败包括：

- channel manifest 无法访问。
- manifest 缺少当前平台资产。
- 下载文件 SHA-256 不匹配。
- macOS DMG 无法挂载或找不到 `OpenWatcher.app`。
- Windows 安装后找不到 `openwatcher.exe`。
- 桌面快捷方式创建失败。

## 安全边界

- 只安装 OpenWatcher 官方 channel manifest 指向的 Desktop 包。
- 不跳过 SHA-256 校验。
- 不修改系统级安全策略。
- 不打印 token、cookie、私有路径日志或其他敏感信息。
- 不处理 OpenWatcher 之外的应用安装或安全限制解除。
