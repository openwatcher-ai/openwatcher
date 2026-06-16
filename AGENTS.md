# 开源协作说明（OpenWatcher）

本文件适用于该开源仓库的公开版开发约定。

## 适用范围

- 本仓库面向开源发布，默认不包含私有生产流程、私有域名与内部运维地址。
- 接口与文档以自托管后端为前提；`openwatcher.ai` 仅用于官网与文档入口。

## 开发规范

- 仅在本仓库中提交面向公开发布的变更。
- 不在公开文档中暴露密钥、token、cookie、凭据或完整隐私日志。
- 修改前优先阅读现有说明文档与最近提交，避免重复冲突。
- 提交前应完成基础验证（至少跑过目标范围内的可行性验证或脚本检查）。
- 需要临时文件、签名材料、用户私有配置时，默认写入 `.gitignore` 并避免入库。

## 开源发布打磨基线

- 当前分支作为 OpenWatcher 开源发布前的细节打磨基线使用；后续会另建 `openwatcher/` 目录，只复制最终要开源的代码、文档、脚本和配置，并在该目录内做开源版本初始化提交。
- 在用户明确要求创建 `openwatcher/` 目录前，不要提前复制全仓或重排目录结构。
- 当前阶段可以保留并同步有助于后续打磨的 repo 内 agent 约束、技能、检查脚本和 release gate；同步时必须改写成 OpenWatcher 公共发布语义。
- 不要从原项目直接搬入私有生产域名、用户本机路径、LaunchAgent 名称、通知 topic、私有下载入口、签名材料、真实 token、cookie、Cloudflare 凭据或完整隐私日志。
- 未来生成 `openwatcher/` 目录前，应先列出包含/排除清单；默认排除 `dist/`、构建缓存、签名目录、用户配置、运行日志、私有部署脚本和任何未脱敏诊断数据。
- `openwatcher/` 初始化前必须跑公开残留扫描，覆盖旧品牌、旧包名、私有域名、用户路径、敏感 header 和密钥字段；只有确认剩余命中均为明确的历史说明或负向检查时，才进入初始化提交。

## 常用命令

- 本地服务：`scripts/start-local.sh`
- 测试：`go test ./...`
- 打包脚本：`scripts/package-watch-release.sh`（按需求使用）
- 发版 Skill：`.codex/skills/openwatcher-release/SKILL.md`
- 手表 release gate：`.codex/skills/openwatcher-release/references/watch-apk-release-gate.md`

## 当前代码事实

- 手表端公开包名为 `ai.openwatcher.watchapp`，源码路径为 `watch-app/app/src/main/java/ai/openwatcher/watchapp`；不要恢复旧 `top.uuss.codexwatcher` 包名或旧路径。
- 手表端 BuildConfig 后端入口字段为 `OPENWATCHER_BASE_URL`；不要恢复旧 `WATCHER_BASE_URL`。
- 手表端默认通过 `ServerConfigRepository`、`EndpointSelector` 和运行时 bootstrap 管理后端入口；release 默认地址不得指向私人生产域名。
- 设置首页已经承载服务状态检查和应用诊断入口；不要恢复旧的独立 `SettingsDestination.ServiceStatus` 服务状态页。
- Desktop 默认按局域网模式启动 sidecar，监听推荐的本机局域网 IPv4 与 `8787` 端口，便于手表访问；托管隧道模式下才使用 loopback origin，并会在端口占用时复用当前进程或自动选择后续可用 loopback 端口，把真实监听端口写入本地 tunnel origin。
- OpenWatcher 托管隧道当前是 Cloudflare Worker 控制面 + Desktop 本地 `cloudflared` 凭据运行链路，不是 mock/Beta 流程。配置码兑换返回 `publicBaseUrl`、`tunnelToken` 和 `tunnelCredentials`；Desktop 优先用 `tunnelCredentials` 写入本地 `cloudflared` config，旧绑定才兼容 `--token-file`。
- 配置推送文案以“发送到手表确认”为准；手表端 bootstrap 不应静默覆盖已有配置。

## 风险控制

- 涉及服务接入、签名策略、打包流程时，保持默认安全口令与凭据不进入仓库。
- 若涉及数据来源变更，应同步更新 README 中的配置示例与接口说明。
- 构建或交付手表 release APK 前，必须先使用 `.codex/skills/openwatcher-release/SKILL.md`，并读取 `.codex/skills/openwatcher-release/references/watch-apk-release-gate.md`；遵守版本递增、最终 APK 验签、ABI 列表、SHA256、`watch-app/RELEASE_BUILDS.md` 记录和公开残留扫描要求。
- `debug` APK 不得作为公开发布产物；release APK 默认至少包含 `armeabi-v7a` 与 `arm64-v8a`，除非已有设备 ABI 证据支持更窄范围。
- 聊天文件、临时路径或 Gradle 原始输出不能替代最终 release artifact；准备交付哪个文件，就验证哪个文件。
