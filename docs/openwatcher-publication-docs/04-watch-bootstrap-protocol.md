# 手表端 Desktop Bootstrap 协议设计

## 目标

Desktop 安装完手表 App 后，需要把后端 API 地址和手表访问 token 写入手表 App。用户不应该手动输入长 URL 或 token，也不应该必须扫码配对。

因此手表 App 需要支持 Desktop Bootstrap 协议。

## 协议入口

Deep link：

```text
openwatcher://bootstrap?endpoints=<base64url-json>&deviceToken=<token>&deviceName=<encoded-name>
```

ADB 调用示例：

```bash
adb -s <serial> shell am start \
  -a android.intent.action.VIEW \
  -d "openwatcher://bootstrap?endpoints=<base64url-json>&deviceToken=<token>&deviceName=<name>"
```

## 参数

| 参数 | 必填 | 说明 |
|---|---|---|
| `endpoints` | 是 | base64url 编码后的入口列表 JSON，至少包含一个可访问的 OpenWatcher 地址 |
| `deviceToken` | 是 | 手表请求后端 API 使用的高熵 token |
| `deviceName` | 否 | 显示在后端配置中的设备名称 |
| `source` | 否 | 可选，默认 `desktop-bootstrap` |
| `nonce` | 后续 | 防重放或短期授权窗口使用 |

当前代码还区分两种 host：

- `openwatcher://bootstrap?...`：公开 `beta` / 正式入口写入；
- `openwatcher://dev-bootstrap?...`：开发环境写入。

## URL 校验规则

- scheme 只允许 `http` 或 `https`；
- 去掉末尾斜杠；
- host 不能为空；
- release 中不允许默认使用私人域名；
- 局域网模式允许 `http://192.168.x.x:8787`；
- 公网模式建议 `https`；
- 非法 URL 不保存，并展示错误。

## Token 校验规则

- token 由 Desktop 生成；
- 推荐 32 字节随机数，base64url 无 padding；
- 手表保存明文 token；
- 后端只保存 SHA-256 hash；
- UI 不显示完整 token，只显示 fingerprint；
- fingerprint 可取 SHA-256 前 6 个 hex 字符或 token hash 前 6 个 hex 字符。

## 手表端状态模型

新增运行时配置：

```kotlin
data class ServerEndpoint(
    val id: String,
    val label: String,
    val url: String,
    val priority: Int,
)

data class ServerConfig(
    val endpoints: List<ServerEndpoint>,
    val activeEndpointId: String?,
    val configuredAt: Instant,
    val source: ServerConfigSource,
)

enum class ServerConfigSource {
    Manual,
    DesktopBootstrap,
    Adb,
    Qr,
    BuildDefault,
}
```

新增或扩展存储：

```text
ServerConfigRepository
DeviceTokenRepository
PairingPreferenceStore
```

## 安全策略

### 首次配置

当手表 App 没有保存 baseUrl，也没有完成配对时：

1. 接收 bootstrap deep link；
2. 解析入口列表，按优先级确定主入口；
3. 展示确认页；
4. 用户确认后保存 endpoints 和 deviceToken；
5. 请求当前主入口的 `/healthz`；
6. 成功后进入主界面。

### 已配置状态

当手表已保存 baseUrl 或已经配对：

- 不允许静默覆盖；
- 展示“已有配置”提示；
- 展示当前主入口 host 和新主入口 host；
- 用户明确确认后才覆盖；
- 后续可增加“允许桌面重新配置 5 分钟”的设置开关。

### 拒绝静默覆盖

即使 deep link 由 ADB 触发，也必须在手表端展示确认页。原因：

- Android deep link activity 可能是 exported；
- 其他应用理论上可能构造 deep link；
- baseUrl 和 token 是安全敏感配置。

## 确认页内容

确认页应展示：

```text
OpenWatcher Desktop 想要配置这块手表

主入口：192.168.1.12:8787
其他入口：自定义公网 URL、托管隧道
设备名：Xiaomi Watch ...
Token 指纹：a1b2c3

确认后，手表将使用该服务器获取 OpenWatcher 状态。
```

按钮：

- “确认配置”；
- “取消”；
- “查看风险说明”。

如果当前已有配置：

```text
当前服务器：old.example.com
新服务器：new.example.com

覆盖后可能需要重新配对。
```

## Desktop 辅助配对流程

推荐主流程：

```text
Desktop 生成 deviceToken
Desktop 计算 tokenHash = sha256(deviceToken)
Desktop 写入后端配置 tokenHash/deviceName/pairedAt
Desktop 重启或热加载后端
Desktop ADB 安装手表 APK
Desktop ADB 触发 openwatcher://bootstrap
手表展示确认页
用户确认后手表保存 endpoints 和 deviceToken
Desktop 验证后端收到手表请求
完成
```

二维码配对仍可保留为 fallback：

```text
用户手动在手表配置 baseUrl
手表生成二维码
手机扫码打开 <baseUrl>/pair?deviceToken=...
后端保存 token hash
```

## Health Check

手表确认保存后应请求：

```text
GET <active-endpoint>/healthz
```

成功条件：

- HTTP 2xx；
- 返回 `ok=true`；
- 可选校验 build 字段；
- 可选校验后端品牌字段 `name=openwatcher`。

失败时不应丢弃配置，而应显示：

- 服务器不可达；
- 可能不在同一 Wi-Fi；
- 可能防火墙阻止；
- 可能 baseUrl 输入错误；
- 可返回设置页重新配置。

## API Client 改造要求

所有手表端网络模块必须使用运行时 `ServerConfigRepository` 中当前选中的 active endpoint：

- 状态请求；
- SSE；
- 截图上传；
- 诊断上传；
- 更新检查；
- APK 下载；
- changelog 获取；
- 配对二维码生成。

`BuildConfig` 中可保留默认值，但只能是 fallback，不能成为唯一配置来源。

## 单元测试要求

必须覆盖：

- bootstrap URI 解析；
- URL 规范化；
- 非法 scheme；
- 缺少 host；
- token 太短；
- 首次配置保存；
- 已配置状态拒绝静默覆盖；
- 用户确认后覆盖；
- healthz 成功/失败；
- API client 使用最新 baseUrl；
- token fingerprint 不泄露完整 token。

## ADB 手工验证命令

```bash
adb devices -l
adb -s <serial> install -r openwatcher-watch-release.apk
adb -s <serial> shell am start \
  -a android.intent.action.VIEW \
  -d "openwatcher://bootstrap?endpoints=<base64url-json>&deviceToken=test-token-please-use-real-random-token&deviceName=watch"
```

手工验证要点：

- 手表出现确认页；
- 不显示完整 token；
- 保存后进入主界面；
- baseUrl 可在设置页看到；
- 重新触发 bootstrap 时不会静默覆盖。
