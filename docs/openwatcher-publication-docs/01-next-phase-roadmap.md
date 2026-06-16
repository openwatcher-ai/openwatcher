# OpenWatcher 后续阶段路线图

## 背景

当前已经完成第一波发布化清理，后续重点不再是简单改名，而是补齐真正影响用户可用性的链路：

- 初次安装手表 App；
- 启动本机后端服务；
- 将手表连接到正确的后端 API；
- 局域网或公网访问配置；
- GitHub Release 分发；
- 非技术用户无需接触命令行。

国内大量小米、OPPO、vivo、部分 Android/Wear OS 类手表不能稳定通过 Google Play 安装应用，各厂商应用商店上架成本高且审核不可控。因此 OpenWatcher 初始技术预览应以 **GitHub Release + OpenWatcher Desktop** 作为主分发和初次安装路线。

## 新的产品主线

```text
OpenWatcher Desktop
  ├─ 内置 openwatcher 后端服务二进制
  ├─ 内置 adb / platform-tools
  ├─ 内置 openwatcher-watch release APK
  ├─ 可选内置 cloudflared
  ├─ 自动检测 Codex 目录
  ├─ 启动并自检本机后端
  ├─ 引导手表无线 ADB 配对
  ├─ 自动安装手表 APK
  ├─ 自动写入手表 baseUrl / token
  ├─ 支持局域网模式
  ├─ 支持用户自定义公网 URL
  └─ 预留 OpenWatcher 托管隧道配置码
```

## 阶段优先级

| 阶段 | 名称 | 目标 | 初始技术预览是否阻断 |
|---|---|---|---|
| P0 | 发布化清理收口 | 确认旧品牌、私人域名、旧路径、旧 header 已清理 | 是 |
| P1 | Desktop MVP | 创建跨平台桌面应用主入口 | 是 |
| P2 | 后端 sidecar | Desktop 可启动内置后端并自检 | 是 |
| P3 | 手表 bootstrap | 手表 App 支持运行时 baseUrl 与 desktop bootstrap | 是 |
| P4 | ADB 安装向导 | Desktop 完成无线 ADB 配对、连接、安装、初始化 | 是 |
| P5 | 局域网模式 | 用户同一 Wi-Fi 下可直接使用 | 是 |
| P6 | 自定义公网 URL | 技术用户可填自己的公网域名并写入手表 | 强烈建议 |
| P7 | 托管隧道 Desktop 接入 | 在已落地的 Worker 控制面之上补 Desktop 真实兑换 UI、本地 cloudflared 运行器与真机验收 | 不阻断 |
| P8 | Release 打包 | GitHub Release 产物、checksums、notices | 是 |
| P9 | 设备文档 | 兼容性表、安装图文步骤、故障排查 | 是 |
| P10 | 最终验收 | 技术、文档、分发、安全验收 | 是 |

## 初始技术预览最小可发布范围

technical preview 的最小可发布范围建议为：

- OpenWatcher Desktop for macOS / Windows；
- Desktop 内置或可定位 openwatcher 后端；
- Desktop 内置或可定位 adb；
- Desktop 内置或可定位手表 release APK；
- Desktop 可检测 Codex 目录并启动后端；
- Desktop 可通过 `/healthz` 自检；
- Desktop 有无线 ADB 安装向导；
- Desktop 可将局域网 baseUrl 写入手表；
- 手表 App 支持运行时 baseUrl；
- 手表 App 支持 desktop bootstrap deep link；
- 手表 App 内部更新机制继续可用；
- GitHub Release 提供 Desktop、APK、后端 CLI、checksums；
- 文档覆盖初次安装、局域网使用、自定义公网 URL、兼容性和安全边界。

## 不建议阻塞初始技术预览的事项

以下内容很重要，但不建议阻塞第一版发布：

- 真实 openwatcher.ai 托管隧道服务；
- 自动创建 Cloudflare Tunnel 的云端后台；
- Desktop 自更新；
- 完整手机 companion app；
- 各厂商应用商店上架；
- Play Store 上架；
- 设备兼容性云上报；
- 完整多语言文档。

## 关键验收命令

```bash
go test ./...
cd watch-app && ./gradlew test
rg -n "watcher\.uuss\.top|top\.uuss|Codex Watcher|codex-watcher|CODEX_WATCHER|X-Codex-Watcher|\.codex-watcher|/Users" .
```

Desktop 工程建立后应补充：

```bash
cd desktop-app
# 按实际技术栈替换
npm test
npm run lint
wails build
```

## 发布前必须人工验证

至少需要人工验证这些路径：

1. Windows 用户从 GitHub Release 下载 Desktop，打开后完成后端启动和自检；
2. macOS 用户从 GitHub Release 下载 Desktop，打开后完成后端启动和自检；
3. 一台真实 Android/Wear OS 手表完成无线 ADB 配对、安装、bootstrap；
4. 局域网模式下手表可访问后端；
5. 自定义公网 URL 模式下手表可访问后端；
6. 手表内部更新检查不依赖旧域名；
7. no-auth 不会被用于局域网/公网/托管隧道正式流程；
8. 日志、诊断包、release artifact 中不包含 token、Cloudflare token、Codex access token、个人路径。
