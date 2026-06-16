# OpenWatcher Desktop 产品需求文档

## 一句话定位

OpenWatcher Desktop 是 OpenWatcher 的正式用户入口：它负责启动本机后端、完成自检、引导用户把手表 App 安装到手表上，并将手表连接到正确的 OpenWatcher 后端。

## 目标用户

### 普通用户

- 不熟悉 ADB；
- 不想下载 Android SDK；
- 不会配置环境变量；
- 只希望按步骤点击和输入手表屏幕上的 IP、端口、配对码；
- 大概率使用小米、OPPO、vivo、三星、Pixel Watch 或其他 Android/Wear OS 类手表。

### 技术用户

- 能自行配置内网穿透或公网域名；
- 希望 GitHub Release 下载 APK 和桌面工具；
- 可能需要自定义 Codex home、后端监听地址、端口、反代 URL。

### 托管隧道用户

- 可接受 OpenWatcher 托管隧道配置码；
- 可反馈手表兼容性；
- 可帮助验证小米、OPPO、vivo、三星、Pixel Watch 等设备。

## 核心场景

### 场景 1：首次安装，局域网使用

```text
用户下载 OpenWatcher Desktop
Desktop 检测 Codex 目录
Desktop 启动后端
Desktop 推荐电脑局域网 IP
用户按图文在手表开启无线调试
用户输入配对 IP、端口、配对码
Desktop 自动 adb pair / connect / install
Desktop 自动写入 baseUrl 和 deviceToken
手表确认配置
手表进入主界面
```

### 场景 2：首次安装，自定义公网 URL

```text
用户下载 OpenWatcher Desktop
Desktop 启动本机后端
用户自行配置 Cloudflare Tunnel / frp / ngrok / 反代
用户把公网 URL 填入 Desktop
Desktop 请求 /healthz 验证
Desktop 安装手表 App
Desktop 将公网 URL 写入手表
手表可离开局域网访问后端
```

### 场景 3：OpenWatcher 托管隧道

```text
用户获得 OpenWatcher 配置码
Desktop 兑换配置码
Desktop 获取 publicBaseUrl、tunnelToken 和 tunnelCredentials
Desktop 用本地 cloudflared 配置启动真实 tunnel
Desktop 写入手表 baseUrl
手表通过 *.openwatcher.ai 子域名访问本机后端
```

### 场景 4：已安装手表 App，只更新配置

```text
Desktop 检测手表已安装 OpenWatcher
用户切换 LAN / 公网 URL / 托管隧道
Desktop 通过 ADB bootstrap 更新 baseUrl
手表展示确认页
用户确认后保存
Desktop 验证连接
```

## 桌面应用页面结构

### 1. 首页 / 状态总览

展示：

- OpenWatcher Desktop 版本；
- 当前平台；
- Codex 检测状态；
- 后端服务状态；
- 当前访问模式；
- 当前 baseUrl；
- 手表安装状态；
- 最近错误；
- 下一步行动按钮。

主按钮：

- “开始安装手表 App”；
- “启动后端服务”；
- “配置访问方式”；
- “打开日志”；
- “复制诊断信息”。

### 2. Codex 检测页

展示：

- 默认 Codex home：`~/.codex`；
- `auth.json` 是否存在；
- `sessions` 是否存在；
- 是否可读；
- 是否需要用户手动选择目录。

安全要求：

- 不读取或展示 access token 明文；
- 不把 Codex auth 内容写入日志；
- 诊断信息只显示状态，不显示敏感内容。

### 3. 后端服务页

能力：

- 启动后端；
- 停止后端；
- 重启后端；
- 查看 `/healthz`；
- 查看当前监听地址；
- 查看 publicBaseUrl；
- 查看日志摘要。

错误处理：

- 端口占用；
- 后端二进制缺失；
- Codex 未登录；
- 配置文件权限异常；
- 防火墙阻止局域网访问。

### 4. 访问方式配置页

三种模式：

1. 局域网模式；
2. 自定义公网 URL；
3. OpenWatcher 托管隧道。

必须明确说明：

- 局域网模式适合电脑和手表在同一 Wi-Fi；
- 自定义公网 URL 需要用户自己完成反代/隧道；
- 托管隧道只提供网络通道，不上传 Codex auth、sessions 或 usage 数据；
- 托管隧道由 Worker 控制面兑换配置码，并由 Desktop 本地 `cloudflared` 进程转发到本机 sidecar；
- no-auth 不能用于公网访问。

### 5. 手表安装向导

步骤：

1. 确认电脑和手表同一 Wi-Fi；
2. 开启开发者模式；
3. 开启无线调试；
4. 输入配对 IP、配对端口、配对码；
5. 输入连接 IP、连接端口；
6. Desktop 自动配对；
7. Desktop 自动连接；
8. Desktop 自动安装 APK；
9. Desktop 启动手表 App；
10. Desktop 发送 bootstrap；
11. 用户在手表确认；
12. Desktop 验证成功。

### 6. 故障排查页

常见问题：

- 找不到 ADB；
- ADB 版本异常；
- 配对失败；
- 连接失败；
- 多设备冲突；
- 安装失败；
- 手表没有安装器；
- 手表无法访问电脑 IP；
- Windows 防火墙阻止；
- macOS 网络权限或防火墙阻止；
- 公网 URL `/healthz` 不可达；
- 手表已配置旧 baseUrl，需要重新确认覆盖。

## MVP 范围

### 必须实现

- Desktop 基础窗口；
- Codex 检测；
- 后端 sidecar 启动；
- `/healthz` 自检；
- ADB 二进制定位；
- 无线 ADB 配对、连接、安装；
- 手表 bootstrap；
- 局域网模式；
- 自定义公网 URL 模式；
- OpenWatcher 托管隧道配置码兑换和本地 cloudflared 启动；
- 日志脱敏；
- GitHub Release 打包。

### 可延后

- Desktop 自更新；
- 多语言；
- 设备兼容性云上报；
- 手机 companion；
- 厂商应用市场上架。

## 隐私与安全文案要求

Desktop 必须用清晰文案说明：

- OpenWatcher 后端运行在用户电脑本地；
- 后端会读取本机 Codex 登录状态和 sessions；
- Desktop 不应上传 Codex auth、sessions、usage 数据；
- 托管隧道只提供网络通道，不是数据托管；
- 手表 token 明文只保存在手表和 Desktop 初始化流程中，服务端只保存 hash；
- ADB 无线调试安装完成后建议关闭；
- 不建议在公共 Wi-Fi 开启无线调试；
- no-auth 只能用于本地开发。

## 成功标准

- 非技术用户无需打开终端即可完成首次安装；
- 用户不需要下载 Android SDK；
- 用户不需要配置环境变量；
- 手表无需 Google Play；
- 安装完成后手表能访问正确后端；
- 用户能在 Desktop 里切换后端访问模式；
- 出错时能看到明确的下一步操作。
