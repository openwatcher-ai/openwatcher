# OpenWatcher Desktop 技术架构设计

## 技术栈建议

首版建议使用 **Wails** 构建 OpenWatcher Desktop。

原因：

- 当前后端是 Go，Desktop 可复用 Go 的进程管理、路径处理、日志脱敏和配置逻辑；
- Wails 适合 Go + Web UI 的跨平台桌面应用；
- 相比 Electron，体积和运行时负担更低；
- 相比 Tauri，避免引入 Rust + Go sidecar 的双语言复杂度；
- 后续仍可将已有 Go 后端作为 sidecar 二进制打包。

Tauri 作为备选，适合后续想更强地利用 sidecar 打包和系统集成能力时评估。Electron 不建议作为首选，除非团队已有成熟 Electron 工程化能力。

## 目录结构建议

```text
desktop-app/
  README.md
  package.json
  wails.json
  frontend/
    src/
      pages/
      components/
      stores/
      assets/
  app/
    main.go
    app.go
  internal/
    adb/
      adb.go
      commands.go
      parser.go
      wizard.go
    backend/
      manager.go
      health.go
      config.go
    codex/
      detector.go
    tunnel/
      runner.go
      client.go
      manager.go
      storage.go
      cloudflared.go
    network/
      interfaces.go
      lan.go
      public_url.go
    installer/
      apk.go
      checksums.go
      bootstrap.go
    logging/
      redact.go
      writer.go
    settings/
      store.go
    diagnostics/
      bundle.go
  bundled/
    README.md
    openwatcher/
      darwin-arm64/
      darwin-amd64/
      windows-amd64/
      windows-arm64/
    runtime/
      manifest-url.txt
  runtime/
    manifests/
      current.json
    downloads/
    platform-tools/
    cloudflared/
    watch-apk/
```

`bundled/` 中不应直接提交大体积二进制到源码仓库。当前 Desktop 安装包只内置 sidecar 和固定 `runtime-stable` manifest 地址，ADB / platform-tools、`cloudflared` 与手表 APK 都改为首次使用时下载到用户配置目录下的 `runtime/` 缓存。

## 配置目录

### Desktop 自身配置

- macOS：`~/Library/Application Support/OpenWatcher`
- Windows：`%APPDATA%\OpenWatcher`
- Linux 预留：`~/.config/openwatcher`

Desktop 自身配置包括：

- UI 状态；
- 最近选择的访问模式；
- 最近选择的 LAN IP；
- ADB 设备记录；
- 兼容性本地记录；
- 托管隧道配置，敏感字段必须加密或权限收紧。

### 后端配置

OpenWatcher 后端配置继续使用用户级目录：

- macOS / Linux：`~/.openwatcher/config.json`
- Windows 可后续评估：`%APPDATA%\OpenWatcher\config.json` 或继续使用 home 下 `.openwatcher`

建议为了跨平台一致性，Desktop 传入显式 `--config`，不要依赖后端自动猜测路径。

## Sidecar 资源管理

Desktop 需要定位这些资源：

- `openwatcher` 后端二进制；
- `adb`；
- `openwatcher-watch-release.apk`；
- 可选 `cloudflared`。

资源查找顺序：

1. Release 打包后的 app resources 目录；
2. 开发模式下的仓库相对路径；
3. 用户手动指定路径；
4. 系统 PATH 作为开发 fallback，正式版不应依赖 PATH。

正式版不应要求用户安装 Android SDK 或配置环境变量。

## 后端进程管理

Desktop 应实现：

```go
type BackendManager interface {
    Start(ctx context.Context, cfg BackendStartConfig) error
    Stop(ctx context.Context) error
    Restart(ctx context.Context, cfg BackendStartConfig) error
    Status(ctx context.Context) BackendStatus
    Health(ctx context.Context) (HealthResponse, error)
    Logs(ctx context.Context, n int) []LogLine
}
```

启动参数：

```text
openwatcher serve \
  --config <path> \
  --listen <host:port> \
  --public-base-url <url>
```

局域网模式下：

```text
--listen <selected-lan-ip>:8787
--public-base-url http://<selected-lan-ip>:8787
```

自定义公网 URL 模式下：

```text
--listen 127.0.0.1:8787
--public-base-url https://user-domain.example.com
```

托管隧道模式下：

```text
--listen 127.0.0.1:8787
--public-base-url https://<subdomain>.openwatcher.ai
```

`127.0.0.1:8787` 是首选端口，不是硬编码唯一端口。若端口已被当前 sidecar 占用，Desktop 会复用现有进程；若被其他进程占用，Desktop 会解析后续可用 loopback 端口，并把实际 origin 写入本地 `cloudflared` 配置。

## 后端健康检查

`/healthz` 建议返回：

```json
{
  "ok": true,
  "build": {
    "version": "dev",
    "commit": "abcdef0",
    "builtAt": "2026-06-07T00:00:00Z"
  },
  "config": {
    "listen": "127.0.0.1:8787",
    "publicBaseUrl": "https://openwatcher.example.com",
    "paired": true,
    "noAuth": false
  },
  "codex": {
    "homeDetected": true,
    "authDetected": true,
    "sessionsDetected": true
  }
}
```

注意：

- 不返回 token hash；
- 不返回 access token；
- 不返回完整敏感路径，必要时只返回状态；
- no-auth 状态要返回，以便 Desktop 阻止公网/托管隧道模式。

## ADB 模块

核心接口：

```go
type ADB interface {
    Version(ctx context.Context) (string, error)
    Pair(ctx context.Context, host string, port int, code string) error
    Connect(ctx context.Context, host string, port int) (ConnectResult, error)
    Devices(ctx context.Context) ([]Device, error)
    Install(ctx context.Context, serial string, apkPath string) error
    StartDeepLink(ctx context.Context, serial string, uri string) error
    StartApp(ctx context.Context, serial string, packageName string) error
    Shell(ctx context.Context, serial string, args ...string) (CommandResult, error)
}
```

所有命令必须支持：

- timeout；
- stdout/stderr 捕获；
- 脱敏；
- `-s <serial>`；
- 多设备选择。

## Bootstrap 写入流程

Desktop 生成：

- `endpoints` 入口列表；
- `deviceToken`；
- `deviceName`；
- `installId` 可选；
- token fingerprint。

后端写入：

- Desktop 将 `sha256(deviceToken)` 写入后端配置；
- 重启或热加载后端；
- 确认 `/api/status` 用该 token 可通过。

手表写入：

```bash
adb -s <serial> shell am start \
  -a android.intent.action.VIEW \
  -d "openwatcher://bootstrap?endpoints=<base64url-json>&deviceToken=<...>&deviceName=<...>"
```

手表必须展示确认页，不允许静默覆盖已有配置。

## 日志与脱敏

必须脱敏：

- `deviceToken`；
- `X-OpenWatcher-Token`；
- Cloudflare `tunnelToken`；
- Cloudflare API Token；
- Codex `access_token`；
- ADB pairing code；
- 任何 `Authorization: Bearer ...`。

日志记录建议：

```text
[time] [module] [level] message
```

命令日志示例：

```text
adb pair 192.168.1.25:37123 ******
exit=0 stdout="Successfully paired..."
```

## 网络模块

### LAN IP 选择

要求：

- 枚举 IPv4；
- 排除 loopback；
- 尽量排除虚拟网卡；
- 推荐默认路由网卡；
- 允许用户手动选择。

### URL 验证

要求：

- scheme 只允许 http/https；
- 去掉末尾斜杠；
- host 不能为空；
- 公网模式默认要求 https；
- 局域网模式允许 http；
- no-auth + 公网 URL 必须阻止。

## 托管隧道模块

接口：

```go
type TunnelRunner interface {
    Start(ctx context.Context, cfg TunnelConfig) error
    Stop(ctx context.Context) error
    Status(ctx context.Context) TunnelStatus
    Logs(ctx context.Context, n int) []LogLine
}
```

当前实现使用 Worker 控制面兑换配置码，并通过本地 `cloudflared` runner 建立隧道。

真实模式优先使用 Worker 返回的 `tunnelCredentials` 写入本地 `cloudflared` 配置，并按 sidecar 实际监听端口写入 ingress；只有兼容旧绑定时才使用 token-file。不要使用会让 token 出现在进程列表里的 `--token` 参数。

## 测试策略

必须测试：

- 路径查找；
- 端口占用；
- 后端 healthz 解析；
- ADB 输出解析；
- 多设备选择；
- URL 规范化；
- 日志脱敏；
- APK SHA-256 校验；
- bootstrap URI 生成；
- no-auth 阻断公网模式。

## 主要风险

| 风险 | 缓解 |
|---|---|
| 用户不会开启无线调试 | 通用文字步骤、兼容性说明、故障排查 |
| ADB 多设备误装 | 必须选择 serial，后续命令都带 `-s` |
| 配对端口和连接端口不同 | UI 分成两个输入区 |
| Windows 防火墙阻止 LAN | 明确提示用户允许访问 |
| token 写入日志 | 全局脱敏器和测试 |
| 真实托管隧道泄露 token | token 不进日志，安全存储，支持撤销 |
| 打包体积过大 | Release artifact 分平台打包，二进制按平台内置 |
