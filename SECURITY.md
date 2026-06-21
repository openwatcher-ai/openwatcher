# Security Policy

OpenWatcher 是本机优先的技术预览项目，会接触本机 Codex 状态目录、Desktop 本地配置、手表配对信息和可选的托管隧道配置。请不要在公开 issue、PR、截图或日志里提交 token、cookie、配置码、tunnel 凭据、设备 token、完整本机路径或完整诊断日志。

## 支持范围

当前只支持最新公开 beta 与其引用的 Runtime Release：

- Product Release: `beta-2026.06.16.1`
- Runtime Release: `runtime-v0.1.0`
- Desktop: `0.1.0`
- Watch: `0.1.0`

旧 beta 和旧 runtime 只作为历史记录保留，不承诺安全补丁。

## 报告漏洞

优先通过 GitHub 仓库的 Security Advisory / private vulnerability report 入口提交漏洞细节。报告中请包含：

- 受影响组件：Desktop、Watch、backend、runtime、release workflow 或文档。
- 影响范围：本机访问、局域网访问、自定义公网 URL、托管隧道或发布产物。
- 复现步骤和期望行为。
- 已脱敏日志或截图。

如果 GitHub 页面没有显示私密漏洞报告入口，请先提交一个不含漏洞细节和敏感信息的普通 issue，请维护者开启私密协调渠道。不要把可利用细节、token、配置码、tunnel 凭据或完整日志直接贴到公开 issue。

## 敏感信息

下面内容必须脱敏：

- `Authorization` header、Bearer token、cookie。
- `deviceToken`、`tokenHash`、`tunnelToken`、`tunnelCredentials`、配置码、配对码。
- Codex auth 文件、session 原文、用户本机完整路径。
- Cloudflare、GitHub、Apple、Microsoft 或 Android 签名凭据。
- 未公开的下载入口、私有部署地址和完整运维日志。

OpenWatcher Desktop 的日志 redaction 会处理常见 token 字段、配对码、配置码和用户 home 路径，但自动脱敏不能替代人工检查。提交前请先查看日志内容。

## 安全边界

- Desktop 默认作为本机控制台运行，不应上传 Codex auth、sessions 或 usage 原始数据到 OpenWatcher 云端。
- 局域网模式只适合可信网络。
- `no-auth` 只允许本机或受控开发环境使用，不得用于公网模式。
- 自定义公网 URL 必须先确认 `/healthz` 指向 OpenWatcher 后端，并启用设备 token。
- 托管隧道当前依赖管理员配置码，不提供公开自助申请；兑换后的 tunnel 凭据只应保存在用户本机 Desktop 配置目录。
- Release 产物不得包含 keystore、`.jks`、`release.properties`、token、私有路径或 debug APK。

## 本地清理

需要清理本机测试状态时，优先删除或备份这些位置：

- `~/.openwatcher/config.json`
- `~/.openwatcher/cache/`
- macOS: `~/Library/Application Support/OpenWatcher`
- Windows: `%APPDATA%/OpenWatcher`

清理前请确认没有正在运行的 OpenWatcher Desktop、sidecar 或 `cloudflared` 实例。
