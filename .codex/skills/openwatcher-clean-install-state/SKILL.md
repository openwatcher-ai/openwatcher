---
name: openwatcher-clean-install-state
description: 备份、清理和恢复 OpenWatcher 本机安装测试状态。适用于模拟干净安装环境、卸载 OpenWatcher Desktop 本机痕迹、清理运行缓存、在清理前备份 ~/.openwatcher、恢复最新或指定时间的 ~/.openwatcher 配置备份；不用于删除源码仓库、~/.codex 或 CODEX_HOME。
---

# OpenWatcher 清理测试环境

## 概览

使用本 Skill 为安装流程测试准备接近干净的 OpenWatcher Desktop 环境。清理前只备份 `~/.openwatcher`，安装包、缓存和应用支持目录直接清理。

## 工作流程

1. 判断当前系统。
   - macOS 使用 `scripts/clean-openwatcher-macos.sh`。
   - Windows 使用 `scripts/clean-openwatcher-windows.ps1`。
   - 其他系统直接说明暂不支持。
2. 默认先预览，不直接删除：
   - macOS：`scripts/clean-openwatcher-macos.sh`
   - Windows：`scripts\clean-openwatcher-windows.ps1`
3. 用户确认要清理时再加 `--yes` 或 `-Yes`。
4. 清理前只备份用户配置目录：
   - macOS / Linux 风格路径：`~/.openwatcher`
   - Windows 路径：`$HOME\.openwatcher`
5. 备份目录固定放在：
   - `~/OpenWatcherBackups/openwatcher-config/<timestamp>/.openwatcher`
6. 支持恢复：
   - 恢复最新备份：`--restore latest`
   - 恢复指定备份：`--restore 20260612-153000`

## macOS 命令

预览清理：

```bash
.codex/skills/openwatcher-clean-install-state/scripts/clean-openwatcher-macos.sh
```

执行清理：

```bash
.codex/skills/openwatcher-clean-install-state/scripts/clean-openwatcher-macos.sh --yes
```

列出备份：

```bash
.codex/skills/openwatcher-clean-install-state/scripts/clean-openwatcher-macos.sh --list-backups
```

恢复最新备份：

```bash
.codex/skills/openwatcher-clean-install-state/scripts/clean-openwatcher-macos.sh --restore latest --yes
```

## Windows 命令

预览清理：

```powershell
powershell -ExecutionPolicy Bypass -File .codex\skills\openwatcher-clean-install-state\scripts\clean-openwatcher-windows.ps1
```

执行清理：

```powershell
powershell -ExecutionPolicy Bypass -File .codex\skills\openwatcher-clean-install-state\scripts\clean-openwatcher-windows.ps1 -Yes
```

恢复最新备份：

```powershell
powershell -ExecutionPolicy Bypass -File .codex\skills\openwatcher-clean-install-state\scripts\clean-openwatcher-windows.ps1 -Restore latest -Yes
```

## 清理范围

macOS 清理：

- `/Applications/OpenWatcher.app`
- `~/Applications/OpenWatcher.app`
- `~/Library/Application Support/OpenWatcher`
- `~/Library/WebKit/ai.openwatcher.desktop`
- `~/Library/Caches/ai.openwatcher.desktop`
- `~/Library/Caches/OpenWatcher`
- `~/Library/Logs/OpenWatcher`
- `~/Library/Preferences/ai.openwatcher.desktop.plist`
- `~/.openwatcher`

Windows 清理：

- `%LOCALAPPDATA%\OpenWatcher`
- `%APPDATA%\OpenWatcher`
- `%USERPROFILE%\.openwatcher`
- `%USERPROFILE%\Desktop\OpenWatcher.lnk`

清理时不要删除源码仓库、`~/.codex`、`CODEX_HOME`、下载目录或其他非 OpenWatcher 路径。

## 恢复规则

恢复前如果当前 `~/.openwatcher` 已存在，脚本会先把它移动到：

```text
~/OpenWatcherBackups/openwatcher-config/pre-restore-<timestamp>/.openwatcher
```

然后再恢复指定备份。恢复模式同样默认预览，需要 `--yes` 或 `-Yes` 才会实际写入。
