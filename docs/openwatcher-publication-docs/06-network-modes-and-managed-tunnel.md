# 访问模式与托管隧道设计

> 当前事实:
> 托管隧道已经切换到 Cloudflare Worker 控制面、MCP 管理面与 Desktop 本地 `cloudflared` 凭据运行链路。
> 公开仓只保留客户端行为与公开契约，不包含私有控制面部署文档。

## 目标

OpenWatcher 后端运行在用户电脑本地，手表需要访问这个后端。根据用户能力和使用场景，Desktop 提供三种访问模式：

1. 局域网模式；
2. 自定义公网 URL 模式；
3. OpenWatcher 托管隧道。

## 模式一：局域网模式

### 适用用户

- 只在家里或办公室同一 Wi-Fi 下使用；
- 不需要离开局域网访问；
- 不想配置域名、隧道或公网反代。

### baseUrl 示例

```text
http://192.168.1.12:8787
```

### Desktop 行为

1. 枚举本机 IPv4；
2. 推荐默认路由网卡 IP；
3. 用户确认 IP 和端口；
4. 后端监听 `<selected-ip>:8787` 或 `0.0.0.0:8787`；
5. `publicBaseUrl` 设置为 `http://<selected-ip>:8787`；
6. Desktop 自检 `http://<selected-ip>:8787/healthz`；
7. Desktop 通过 ADB bootstrap 写入手表。

### 网卡过滤

应排除或降权：

- loopback；
- Docker；
- VMware；
- VirtualBox；
- Tailscale；
- ZeroTier；
- link-local `169.254.x.x`；
- IPv6 首版可暂缓。

### 防火墙提示

Windows/macOS 首次监听局域网可能弹出防火墙提示。Desktop 应提醒：

```text
请允许 OpenWatcher 接受局域网连接，否则手表无法访问电脑上的后端服务。
```

不要尝试绕过系统防火墙。

### 安全边界

- 局域网模式只建议在可信 Wi-Fi 使用；
- 不建议在公共 Wi-Fi 开启无线调试；
- no-auth 不允许用于局域网模式；
- 敏感接口仍需 `X-OpenWatcher-Token`。

## 模式二：自定义公网 URL

### 适用用户

- 有自己的域名或内网穿透能力；
- 能配置 Cloudflare Tunnel、frp、ngrok、Tailscale Funnel、反向代理等；
- 希望手表离开局域网也能访问。

### baseUrl 示例

```text
https://openwatcher.example.com
```

### Desktop 行为

1. 用户输入公网 URL；
2. Desktop 校验 URL 格式；
3. Desktop 请求 `<url>/healthz`；
4. 校验返回 OpenWatcher 后端；
5. 写入后端 `publicBaseUrl`；
6. 阻止 no-auth；
7. 通过 ADB bootstrap 写入手表。

### 校验规则

- scheme 只允许 http/https；
- 默认要求 https；
- host 不能为空；
- 去掉末尾斜杠；
- `/healthz` 必须返回 ok；
- build/name 字段应能识别为 OpenWatcher；
- no-auth 状态为 true 时阻止继续。

### 错误提示

| 错误 | 排查建议 |
|---|---|
| DNS 解析失败 | 检查域名解析 |
| TLS 失败 | 检查证书 |
| 连接超时 | 检查隧道或防火墙 |
| 502/503 | 检查反代 origin |
| healthz 非 OpenWatcher | 检查 URL 是否填错 |
| no-auth 开启 | 关闭 no-auth 后重试 |

## 模式三：OpenWatcher 托管隧道

### 适用用户

- 不会配置公网域名；
- 希望手表离开局域网也能访问；
- 拿到 OpenWatcher 管理面签发的一次性配置码；
- 接受流量经过 Cloudflare 和 openwatcher.ai 子域名。

### 重要原则

- 配置码不是 tunnel token；
- 配置码一次性兑换；
- `tunnelToken` 和 `tunnelCredentials` 是敏感凭证；
- Cloudflare API Token 只存在云端服务，不能进入 Desktop；
- OpenWatcher Cloud 不接收 Codex auth、sessions、usage 数据；
- 托管隧道只是网络通道，不是身份认证；
- 后端敏感接口仍必须 token 鉴权；
- no-auth 禁止用于托管隧道模式。

## 配置码模型

术语：

| 名称 | 说明 |
|---|---|
| `inviteCode` | 用户拿到的一次性配置码 |
| `installId` | Desktop 本地生成的随机安装 ID |
| `redemption` | 配置码兑换记录 |
| `tunnelId` | 云端创建或分配的 tunnel ID |
| `tunnelToken` | Desktop 兼容旧 token-file 运行方式所需 token |
| `tunnelCredentials` | Desktop 本地 `cloudflared` 配置运行方式所需凭据 |
| `publicBaseUrl` | 分配给用户的 `https://xxx.openwatcher.ai` |
| `revocation` | 撤销记录 |

## 云端 API

### 当前用户入口：兑换配置码

```http
POST https://api.worker.openwatcher.ai/v1/tunnel/redeem
Content-Type: application/json

{
  "code": "OW-ABCD-1234",
  "installId": "local-random-install-id",
  "machineFingerprintHash": "sha256-of-machine-fingerprint",
  "desktopVersion": "0.1.0",
  "platform": "windows-amd64"
}
```

响应：

```json
{
  "ok": true,
  "publicBaseUrl": "https://ow-a1b2c3.openwatcher.ai",
  "tunnelToken": "<sensitive>",
  "tunnelCredentials": {
    "AccountTag": "<account>",
    "TunnelSecret": "<sensitive>",
    "TunnelID": "<id>",
    "Endpoint": ""
  },
  "tunnelId": "<id>",
  "tokenVersion": 1,
  "issuedAt": "2026-07-01T00:00:00Z"
}
```

### 管理操作

当前实现里，`rotate`、`revoke`、`status` 已收拢到私有 Cloudflare Worker 管理面，不再作为公开 HTTP 契约对 Desktop 暴露。

## Desktop 托管隧道流程

```text
用户输入 inviteCode
Desktop 调用 redeem API
云端返回 publicBaseUrl + tunnelToken + tunnelCredentials
Desktop 安全保存绑定信息和本地运行凭据
Desktop 启动后端；若 127.0.0.1:8787 被占用，会自动选择后续可用端口
Desktop 按实际 sidecar origin 写入 cloudflared 本地运行配置
Desktop 请求 publicBaseUrl/healthz 验证
Desktop 将配置发送到手表确认
```

## cloudflared 运行方式

优先：

```bash
cloudflared tunnel --config <path> run <tunnel-id>
```

兼容旧绑定时使用：

```bash
cloudflared tunnel --url <actual-origin> run --token-file <path>
```

不应使用会把 token 明文放进进程列表的 `--token <token>`。

## 本地敏感存储

### macOS

优先 Keychain。

### Windows

优先 Credential Manager 或 DPAPI。

### 开发模式

可使用本地测试文件，但必须：

- 不提交；
- `.gitignore`；
- 日志脱敏；
- 文件权限收紧。

## 技术预览要求

技术预览以真实托管隧道链路为当前实现事实：

- Worker 控制面负责兑换配置码，并返回 `publicBaseUrl`、`tunnelToken` 和 `tunnelCredentials`；
- Desktop 本地保存绑定信息，优先用 `tunnelCredentials` 写入 `cloudflared` config；
- Desktop 默认按局域网模式启动 sidecar，监听推荐的本机局域网 IPv4 与 `8787` 端口；
- 托管隧道模式使用 loopback origin，端口占用时使用实际可用 loopback 端口；
- 同一 tunnelId + originURL 可复用旧 `cloudflared` 进程；
- UI 不再标注 Mock；命名中的历史 `Beta` 只可作为内部兼容名，不代表 mock 实现；
- 不在仓库放任何真实 token、Cloudflare 凭据或用户配置。

## 威胁模型

| 风险 | 缓解 |
|---|---|
| 配置码泄露 | 一次性兑换、过期、速率限制 |
| tunnelToken 泄露 | 不进日志、安全存储、支持撤销 |
| 子域名滥用 | 审计、限额、封禁、invite 制 |
| no-auth 暴露公网 | Desktop 阻止，后端警告 |
| 用户误以为数据托管在 openwatcher.ai | 文档明确只提供网络通道 |
| Cloudflare API Token 泄露 | 只在云端环境变量，不返回客户端 |
| 成本失控 | 首批 invite 限量，流量监控 |
