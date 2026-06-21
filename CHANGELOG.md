# Changelog

## Unreleased

- 补充公开项目治理文件、隐私与安全边界说明、GitHub issue templates，并把 release 文档中的当前公开 beta 事实更新到 `beta-2026.06.16.1` / `runtime-v0.1.0`。
- 强化 Product Release 产物检查，要求 `release-notes.md` 与 manifest、checksums、changelog entry 和第三方 notices 一起存在。

## 0.1.0 Public Beta

### Added

- OpenWatcher Desktop technical preview，作为当前公开预览的主入口。
- Wear OS / Android 手表应用，用于查看 Codex 会话状态、token 消耗和运行摘要。
- 本机 Go 后端、Desktop 配对向导、局域网模式和自定义公网 URL 模式。
- 托管隧道兑换链路的公开客户端侧接入；当前仍依赖管理员发码。
- Runtime 资源下载模型、beta channel manifest、Product Release fact package 和公开下载入口。

### Security / Privacy

- 默认本机优先，不把 Codex auth、session 原文、完整 prompt、完整本机路径、设备 token、tunnel token、配置码或配对码上传到 OpenWatcher 云端。
- 手表访问后端依赖设备 token；`no-auth` 只允许本地调试，不支持公网暴露。
- 公开 issue、PR、日志和截图不得包含 token、cookie、配置码、tunnel 凭据、完整隐私日志或未脱敏本机路径。

### Verified

- Windows x64 release 安装器主路径：安装、启动、本机服务自检、ADB 安装手表 APK 和运行时配置写入。
- 手表真机主路径：安装、启动、bootstrap 配置写入和基本后端访问。
- Product Release `beta-2026.06.16.1` 与 Runtime Release `runtime-v0.1.0` 的 manifest、checksums、notes、changelog entry 和第三方 notices 产物检查。

### Known limitations

- macOS technical preview 包当前未签名、未公证，首次启动需要在 Finder 中右键打开。
- 托管隧道不提供公开自助申请或自助发码。
- Windows arm64、macOS Intel 和更多手表型号仍需要真实设备样本。
- 公网模式和更多网络环境仍需要继续补充兼容性验证。
- 当前是 Public Beta，不是 stable 或 GA，不承诺覆盖所有 Wear OS / Android 手表。
