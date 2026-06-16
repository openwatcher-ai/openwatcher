# ADB 无线安装向导设计

## 目标

让非技术用户无需安装 Android SDK、无需配置环境变量、无需打开终端，也能将 OpenWatcher 手表 App 安装到手表上，并完成初始化配置。

## 前置假设

- 用户电脑和手表在同一局域网；
- 手表支持 Android APK 安装；
- 手表支持无线 ADB 调试；
- 用户可在手表系统设置中开启开发者模式和无线调试；
- Desktop 内置 ADB / platform-tools；
- Desktop 内置手表 release APK。

## 重要约束

无线 ADB 通常会出现两个端口：

1. 配对端口：用于 `adb pair ip:port`；
2. 连接端口：用于 `adb connect ip:port`。

部分设备这两个端口不同，向导不能假设相同。

## 向导状态机

```text
Welcome
  ↓
SameWiFiCheck
  ↓
EnableDeveloperModeGuide
  ↓
EnableWirelessDebuggingGuide
  ↓
InputPairingInfo
  ↓
Pairing
  ↓
InputConnectInfo
  ↓
Connecting
  ↓
SelectDeviceIfNeeded
  ↓
InstallingAPK
  ↓
LaunchingWatchApp
  ↓
BootstrapConfig
  ↓
VerifyWatchStatus
  ↓
Done
```

错误统一进入：

```text
Troubleshooting
```

## 每一步设计

### 1. Welcome

说明：

- 将通过无线调试安装 OpenWatcher；
- 不需要下载 ADB；
- 安装完成后建议关闭无线调试；
- 需要电脑和手表在同一 Wi-Fi。

### 2. SameWiFiCheck

用户确认：

- 电脑连接 Wi-Fi；
- 手表连接同一 Wi-Fi；
- 不在公共网络；
- VPN/访客网络可能导致无法发现或访问。

Desktop 可显示当前电脑局域网 IP。

### 3. EnableDeveloperModeGuide

图文占位：

```text
打开手表 设置
进入 关于手表 / 系统信息
连续点击 版本号 / 构建号 7 次
看到“已处于开发者模式”提示
返回设置页
```

不同品牌入口不同，文档和 UI 保留品牌说明占位：

- Xiaomi / HyperOS Watch；
- OPPO Watch；
- vivo Watch；
- Samsung / Wear OS；
- Pixel Watch；
- Huawei 设备需单独验证，不默认承诺支持。

### 4. EnableWirelessDebuggingGuide

图文占位：

```text
打开 开发者选项
开启 无线调试
点击 使用配对码配对设备
记下 IP 地址、配对端口、六位配对码
```

提示：

- 配对码短时间有效；
- 页面不要关闭太久；
- 配对码过期需要重新生成。

### 5. InputPairingInfo

输入：

- 手表 IP；
- 配对端口；
- 六位配对码。

校验：

- IP 格式；
- 端口范围 1-65535；
- 配对码非空；
- 日志中配对码脱敏。

### 6. Pairing

命令：

```bash
adb pair <ip>:<pairing-port>
```

向 stdin 输入配对码。

成功判断：

- 退出码为 0；
- stdout 包含成功配对信息；
- 或后续 connect 成功。

失败提示：

- 配对码错误；
- 配对码过期；
- IP 或端口错误；
- 不在同一网络；
- 防火墙或路由器隔离。

### 7. InputConnectInfo

输入：

- 连接 IP；
- 连接端口。

很多手表无线调试页会显示“IP 地址和端口”，它与“配对码弹窗里的端口”可能不同。UI 必须明确说明。

### 8. Connecting

命令：

```bash
adb connect <ip>:<connect-port>
adb devices -l
```

成功后得到 serial：

```text
<ip>:<connect-port>
```

### 9. SelectDeviceIfNeeded

如果 `adb devices -l` 返回多个设备：

- 显示设备列表；
- 要求用户选择手表；
- 后续所有命令都必须使用 `-s <serial>`；
- 避免误装到手机或模拟器。

设备信息可包含：

- serial；
- model；
- product；
- transport；
- device/offline/unauthorized 状态。

### 10. InstallingAPK

安装前：

- 定位 `openwatcher-watch-release.apk`；
- 读取 versionName/versionCode；
- 校验 SHA-256；
- 确认不是 debug APK；
- 显示 APK 信息。

命令：

```bash
adb -s <serial> install -r <apk-path>
```

失败处理：

| 错误 | 说明 |
|---|---|
| `INSTALL_FAILED_VERSION_DOWNGRADE` | 已安装更高版本，需要用户确认降级或取消 |
| `INSTALL_FAILED_NO_MATCHING_ABIS` | APK ABI 不兼容 |
| `INSTALL_PARSE_FAILED_NO_CERTIFICATES` | APK 签名异常或文件损坏 |
| `INSTALL_FAILED_UPDATE_INCOMPATIBLE` | 签名不一致，可能安装过非官方包 |
| `device offline` | 设备离线，重新 connect |
| `unauthorized` | 手表未授权调试 |

### 11. LaunchingWatchApp

命令可选：

```bash
adb -s <serial> shell monkey -p ai.openwatcher.watch 1
```

或：

```bash
adb -s <serial> shell am start -n ai.openwatcher.watch/.MainActivity
```

### 12. BootstrapConfig

Desktop 生成：

- endpoints 入口列表；
- deviceToken；
- deviceName；
- token fingerprint。

先写后端：

- 保存 token hash；
- 保存 deviceName；
- 保存 pairedAt；
- 重启或热加载后端。

再写手表：

```bash
adb -s <serial> shell am start \
  -a android.intent.action.VIEW \
  -d "openwatcher://bootstrap?endpoints=<base64url-json>&deviceToken=<...>&deviceName=<...>"
```

手表端必须显示确认页。

### 13. VerifyWatchStatus

验证方式：

- Desktop 观察后端是否收到手表请求；
- Desktop 调用后端 `/api/status` 使用相同 token 验证后端鉴权；
- UI 让用户确认手表是否显示在线；
- 后续可通过手表端诊断上传做更强验证。

### 14. Done

完成页展示：

- 安装成功；
- 当前 baseUrl；
- 当前访问模式；
- 建议关闭手表无线调试；
- 如何之后更新 App；
- 如何重新配置服务器地址。

## ADB 二进制打包

正式版查找顺序：

1. Desktop 首次启动或依赖缺失时下载到本机 `runtime/` 缓存的 platform-tools；
2. `bundled/` 中的开发或兼容 fallback；
3. 用户手动指定 ADB 路径；
4. 开发模式下 PATH fallback。

Windows 需要注意：

- `adb.exe`；
- platform-tools 相关 DLL；
- 路径空格处理；
- Defender 或安全软件误报。

macOS 需要注意：

- 可执行权限；
- Gatekeeper；
- 当前 technical preview 包未公证，首次启动 Desktop 时需要提示用户在 Finder 中右键应用并选择“打开”；
- app bundle resources 路径。

## 日志脱敏

必须脱敏：

```text
adb pair 192.168.1.25:37123 ******
openwatcher://bootstrap?...deviceToken=<redacted>
X-OpenWatcher-Token: <redacted>
tunnelToken=<redacted>
Authorization: Bearer <redacted>
```

## 兼容性记录

安装完成后本地记录：

```json
{
  "brand": "",
  "model": "",
  "androidVersion": "",
  "apiLevel": 0,
  "supportsWirelessAdb": true,
  "supportsAdbPair": true,
  "installSucceeded": true,
  "knownIssues": [],
  "verifiedAt": ""
}
```

默认不上传。后续可让用户选择匿名上报兼容性。

## 文档配套

需要配套：

- `docs/install-watch-with-desktop.md`；
- `docs/watch-compatibility.md`；
- `docs/troubleshooting-adb.md`。

兼容性表不得虚构型号。未验证设备应标注“待验证”。
