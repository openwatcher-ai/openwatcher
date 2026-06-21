# Privacy

OpenWatcher 的公开技术预览以本机优先为默认设计。Desktop 负责读取本机 Codex 状态、启动本机后端、安装手表 App，并把必要的访问地址写入手表运行时配置。

## 默认不上传的内容

OpenWatcher 不应上传下面内容到 OpenWatcher 云端：

- Codex auth 文件、access token、cookie 或 API key。
- Codex session 原文、完整 prompt、完整日志。
- 用户本机完整路径、完整环境变量、签名材料或私有配置。
- 手表设备 token、托管隧道 token、配置码或配对码。

## 本机会保存什么

根据使用路径，OpenWatcher 可能在本机保存：

- `~/.openwatcher/config.json`：后端配置。
- `~/.openwatcher/cache/`：缓存状态、定价或会话摘要。
- `~/.openwatcher/diagnostics/`：用户主动导出的诊断信息。
- macOS `~/Library/Application Support/OpenWatcher` 或 Windows `%APPDATA%/OpenWatcher`：Desktop 设置、runtime 缓存和托管隧道本地运行配置。

托管隧道模式下，Desktop 会保存 `publicBaseUrl`、`tunnelToken`、`tunnelCredentials`、`tunnelId`、`tokenVersion` 和兑换时间，用于启动本机 `cloudflared` 实例。这些字段属于敏感本机配置，不应进入公开日志、issue 或 release 产物。

## 诊断与反馈

提交 issue 前，请先使用 Desktop 提供的诊断摘要或手动脱敏日志。公开反馈时建议只提供：

- Desktop 版本、电脑系统和架构。
- Watch 版本、手表系统类型和网络模式。
- `/healthz` 是否通过。
- ADB 配对、安装或 bootstrap 卡住的阶段。
- 已脱敏的错误信息。

不要公开提交 token、配置码、完整本机路径、完整 Codex session、完整 shell 历史或完整系统日志。

## 网络模式

- 局域网模式：只在可信 Wi-Fi 下使用。
- 自定义公网 URL：必须使用你控制的地址，优先使用 HTTPS，并确认后端启用设备 token。
- 托管隧道：当前为管理员发码流程，不提供公开自助申请。

如果发现 OpenWatcher 上传了不应上传的数据，请按 [SECURITY.md](SECURITY.md) 报告。
