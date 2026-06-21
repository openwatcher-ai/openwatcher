# OpenWatcher Desktop Technical Preview 用户指南

本文档面向外部用户，描述 OpenWatcher 当前公开技术预览的安装与使用路径。

## 先看这四条

- OpenWatcher Desktop 是当前技术预览的主入口，负责启动本机服务、安装手表 App，并把手表连接到你的 OpenWatcher 服务地址。
- 当前推荐路径是 **局域网模式** 或 **自定义公网 URL 模式**。
- 仓库里已经实现真实托管隧道 Worker 控制面、管理员 MCP，以及 Desktop 侧真实兑换 UI 和本地 `cloudflared` 运行器；托管隧道仍依赖管理员发码，不提供公开自助申请。
- Windows x64 release 安装器主路径已经完成真实验收；手表真机安装、启动和运行时配置写入已经完成真实验收。公网模式和更多手表机型兼容性仍需继续补充样本。

## 下载与启动

### 下载什么

请以当前 GitHub Release 的附件为准，优先下载与你电脑平台匹配的 OpenWatcher Desktop 包。

目标交付平台：

- macOS Apple Silicon
- macOS Intel
- Windows x64
- Windows arm64

注意：

- Windows x64 已完成 release 安装器主路径验收，包括安装、启动、本机服务自检、ADB 安装手表 APK 和运行时配置写入。
- Windows arm64 已生成公开 release 产物，但仍需要更多真实设备验收样本。
- GitHub Release 的 Desktop 包当前只附带主程序和 sidecar；首次启动或依赖缺失时，Desktop 会自动下载并缓存 ADB / platform-tools、`cloudflared` 和手表 APK 到本机 `runtime/` 目录。

### 启动前准备

建议先确认：

1. 这台电脑上已经登录过 Codex，并且本机存在 `~/.codex` 或 `CODEX_HOME`。
2. 电脑与手表准备接入同一局域网，至少用于首次安装和排障。
3. 你要使用的网络模式已经明确：
   - 只在同一 Wi-Fi 下使用：选局域网模式。
   - 需要离开局域网访问：先自行准备一个可用的公网反向代理或隧道，再选自定义公网 URL 模式。

### 首次启动

Desktop 启动后，建议先完成这些检查：

1. 确认 Codex 目录状态正常。
2. 确认 Desktop 已找到 OpenWatcher 本机服务组件。
3. 启动本机服务并查看 `/healthz` 是否通过。
4. 确认 Desktop 已找到或已开始下载 ADB。
5. 确认 Desktop 已找到或已开始下载手表 APK。

如果系统弹出防火墙提示，需要允许 OpenWatcher 接受局域网连接，否则手表无法访问电脑上的后端服务。

如果你在 macOS 上首次双击 `OpenWatcher.app` 被系统拦截，这是当前技术预览未公证包的预期表现。请在 Finder 中右键应用，选择“打开”，再确认一次后继续。

## 首次安装向导

当前主路径可以概括为下面几步：

1. 在 Desktop 中启动本机服务。
2. 选择网络访问方式。
3. 打开手表无线调试。
4. 在 Desktop 中输入配对 IP、配对端口、配对码，并完成 `adb pair`。
5. 输入连接端口，完成 `adb connect`。
6. 在 Desktop 中选择目标设备，安装手表 APK。
7. 启动手表 App。
8. 由 Desktop 生成 bootstrap 配置，并把入口列表 `endpoints`、`deviceToken` 和 `deviceName` 写入手表。
9. 回到手表确认是否能正常请求 OpenWatcher 后端。

如果当前已经有一块已连接设备，Desktop 会优先复用，不一定要求重新配对。

## 局域网模式

适合场景：

- 电脑和手表长期在同一 Wi-Fi。
- 你不打算做公网暴露。
- 你想先用最短路径验证功能。

局域网模式下，Desktop 会：

1. 枚举本机 IPv4 地址。
2. 推荐一个适合作为手表访问入口的网卡 IP。
3. 让后端监听所选 IP 和端口。
4. 把 `http://<局域网IP>:8787` 这样的地址写入手表。
5. 对对应 `/healthz` 做自检。

使用建议：

- 只在可信 Wi-Fi 下启用。
- 不要在公共 Wi-Fi 上打开无线调试。
- 不要在局域网模式下使用 `no-auth`。

## 自定义公网 URL 模式

适合场景：

- 你已经有自己的域名、反向代理或隧道。
- 你希望手表离开局域网后也能访问。

你需要自己准备一个已经可达的地址，例如：

```text
https://openwatcher.example.com
```

Desktop 在这个模式下会做两件事：

1. 请求你填写地址的 `/healthz`，确认它确实连到了 OpenWatcher 后端。
2. 把这个公网 URL 写入手表运行时配置。

使用建议：

- 优先使用 `https://`。
- 先在电脑浏览器中确认 `<你的地址>/healthz` 可访问，再回到 Desktop。
- 如果后端处于 `no-auth` 状态，不要继续做公网暴露。

## 托管隧道

当前 Desktop 已经接入真实托管隧道链路：

1. 输入管理员提供的一次性配置码。
2. Desktop 调用已上线的 Worker redeem API。
3. Desktop 本地安全保存 `publicBaseUrl`、`tunnelToken`、`tunnelCredentials`、`tunnelId`、`tokenVersion` 和 `redeemedAt`。
4. Desktop 用独立运行配置启动本地 `cloudflared`。
5. Desktop 校验 `publicBaseUrl/healthz`，再把真实基址写入手表 bootstrap。

当前边界：

- 托管隧道仍依赖管理员发码，不提供公开自助申请或公开后台页面。
- 如果管理员重置或撤销绑定，Desktop 会提示联系管理员重新绑定。

## 故障排查

### Desktop 提示未找到本机服务组件

说明当前 Desktop 包里没有找到 OpenWatcher 本机服务二进制，或者你是在源码目录里直接启动了桌面端。

先检查：

- 你是否使用了 release 包，而不是只下载源码。
- 如果你是开发者，本地是否已经生成 `bin/openwatcher`。

### `/healthz` 未通过

常见原因：

- 本机服务没有真正启动。
- 监听地址和你选择的网络模式不一致。
- 本机防火墙拦截了连接。
- 你填写的自定义公网 URL 没有正确反代到 OpenWatcher。

排查顺序：

1. 先在本机访问 `http://127.0.0.1:8787/healthz`。
2. 再访问局域网或公网地址对应的 `/healthz`。
3. 检查 Desktop 诊断与日志页面。

### ADB 不可用或找不到设备

先检查：

- 手表是否已经开启无线调试。
- 电脑和手表是否在同一网络。
- 配对端口和连接端口是否填反。
- 手表是否已经被其他 ADB 会话占用。

### APK 安装失败

先检查：

- 当前 Desktop 是否找到了手表 APK。
- 运行时资源下载是否已经完成，网络是否阻止了首次下载。
- 手表剩余空间是否足够。
- 设备系统是否允许安装对应包。
- 这块手表是否在当前兼容范围内。

### bootstrap 后手表仍连不上

先检查：

- 你选的网络模式是否真的可从手表访问。
- `baseUrl` 是否填成了只对电脑本机可见的地址。
- 电脑防火墙是否阻止了手表访问。
- 公网代理是否正确转发到了 OpenWatcher 后端。

## 隐私与安全

OpenWatcher Desktop 的目标是做本机控制台，不是把你的 Codex 数据上传到 OpenWatcher 云端。

当前公开文档要求保持这些边界：

- Desktop 只检查本机 Codex 目录是否存在和可读。
- 不应把任何云端 token、兑换码、私有密钥打进公开仓库或公开包。
- 手表访问后端仍依赖设备 token，不能把 `no-auth` 当作公网方案。
- 对外反馈问题时，优先提供 Desktop 的脱敏日志和诊断摘要，不要直接公开你的本机配置或凭据。

## 已知限制

- 托管隧道不提供公开自助注册或自助发码。
- Windows x64 已完成主路径验收，但仍建议在更多 Windows 版本和网络环境中补充样本。
- Windows arm64 有 release 产物，仍缺少足够真实设备样本。
- macOS technical preview 包当前按未公证 release 交付，首次启动需要右键打开。
- 手表真机已完成安装、启动和配置写入主路径验收；公网模式和更多机型兼容性仍需继续补充样本。
- 当前兼容性仍在扩大，不能保证所有 Wear OS 或 Android 手表都能稳定使用。
- 当前文档描述的是技术预览主路径，不代表正式商业级支持承诺。

## 兼容性说明

### 电脑平台

- macOS Apple Silicon：当前主要开发路径。
- macOS Intel：已生成公开 release 产物，仍需要更多真实设备验收样本。
- Windows x64：release 安装器主路径已完成真实验收。
- Windows arm64：已生成公开 release 产物，仍需要更多真实设备验收样本。
- Linux：当前不在本轮公开技术预览范围内。

### 手表平台

- 目标设备是支持无线调试的 Wear OS / Android 手表。
- 小米、OPPO、vivo 等非官方 Wear OS 风格设备属于重点关注对象，但真机覆盖仍在补。
- 在没有真机验收结论前，不应把任何具体型号写成“已经完全兼容”。

## 反馈问题时建议带上这些信息

- Desktop 版本
- 电脑系统版本与架构
- 选择的网络模式
- `/healthz` 是否通过
- ADB 配对和安装卡在哪一步
- 脱敏后的 Desktop 日志和诊断摘要
