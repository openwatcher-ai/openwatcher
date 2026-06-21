# OpenWatcher Component Release 验收 Checklist

本文档用于组件级发布模型落地后的真实发布验收。它只记录当前模型需要验证的事实，不把未执行的线上步骤写成已完成。

## 当前状态标记

- `已完成`
  - 本轮已经完成代码、文档或本地脚本验证
- `待真实发布验收`
  - 必须依赖 GitHub Actions、私有 Worker、Pages 或真实设备环境
- `当前阻塞`
  - 本轮未具备相应外部环境或发布窗口，尚未执行

## 本轮已完成

### 契约与文档

- [x] 权威 spec、Skill、`release-contract`、`release-notes` 已对齐组件级发布模型
- [x] 公开仓发布文档已改成“发布记录号 + 组件独立版本 + release manifest 事实包 + official channel/changelog”

### 公开仓脚本与 workflow

- [x] `publish-beta.yml` 已改成 `release_scope` 输入
- [x] Product Release tag 已改成 `beta-YYYY.MM.DD.N`
- [x] `generate-release-manifest.sh` 支持组件复用
- [x] `generate-changelog-entry.sh`、`generate-release-notes.sh` 已新增；`generate-channel-manifest.sh` 已退出公开仓职责
- [x] `check-release-artifacts.sh` 已按新 schema 校验，并强制要求 `release-manifest.json`、`checksums.txt`、`release-notes.md`、`changelog-entry.json`、`THIRD_PARTY_NOTICES.md`
- [x] `scripts/test-release-product-scripts.sh` 已覆盖 full 和 desktop 复用场景

### 公开治理与安全文档

- [x] `SECURITY.md` 已补充支持版本、漏洞报告路径、敏感信息脱敏要求和公网安全边界
- [x] `PRIVACY.md` 已补充本机保存内容、默认不上传内容和诊断反馈边界
- [x] `CONTRIBUTING.md` 已补充公开仓贡献范围与验证命令
- [x] `.github/ISSUE_TEMPLATE/bug_report.yml` 已补充脱敏确认
- [x] `.github/ISSUE_TEMPLATE/compatibility_report.yml` 已补充兼容性样本字段
- [x] `.github/ISSUE_TEMPLATE/config.yml` 已提供私密安全报告入口

### 私有 Worker / Pages / 客户端

- [x] Worker 已支持 `/channels/beta.json`
- [x] Worker 已支持 `/changelog.json`
- [x] Worker 已支持 `/changelog/<releaseTag>.json`
- [x] Worker 已支持 `POST /admin/changelog/beta`
- [x] Pages 已代理 changelog 并渲染首页最近发布记录
- [x] Watch 已支持从 `/channels/beta.json` 解析新 schema
- [x] Watch 已支持解析结构化 changelog
- [x] Desktop runtime 包已支持解析聚合 changelog 并过滤与 Desktop 相关的条目

### 本地验证

- [x] `scripts/test-release-product-scripts.sh`
- [x] `go test ./desktop-app/internal/runtime/...`
- [x] `cd workers/openwatcher-control-plane && npm run check`
- [x] `cd watch-app && ./gradlew --no-daemon testDebugUnitTest --tests 'ai.openwatcher.watchapp.data.WatcherApkUpdateManagerTest'`
- [x] `publish-beta.yml` YAML 解析通过

## 本轮已完成的真实发布验收

### GitHub Actions

- [x] 真实执行 `publish-runtime.yml`
- [x] 真实执行新的 `publish-beta.yml`
- [x] 完成一次 `release_scope=full` 的公开 beta 发布

### GitHub Release 证据

- [x] Release assets 包含 `release-manifest.json`
- [x] Release assets 包含 `checksums.txt`
- [x] Release assets 包含 `changelog-entry.json`
- [x] Release assets 包含 `release-notes.md`
- [x] Release notes 与 `changelog-entry.json` 语义一致

当前公开产物：

- Runtime Release：`runtime-v0.1.0`
- Product Release：`beta-2026.06.16.1`
- Release commit：`130013e3eb29bff4eb20b6b293278ebd6f04ce47`

### 私有 Worker / Pages

- [x] 部署新的 control-plane Worker
- [x] 部署新的 Pages 站点
- [x] `GET https://openwatcher.ai/channels/beta.json` 返回新 schema
- [x] `GET https://openwatcher.ai/changelog.json` 返回聚合历史
- [x] `GET https://openwatcher.ai/changelog/<releaseTag>.json` 返回单条记录
- [x] `GET https://openwatcher.ai/changelog.json` 正常返回聚合 changelog

### 发布面与兼容入口

- [x] `scripts/verify-openwatcher-release-surface.sh` 通过
- [x] official channel 中客户端 URL 指向 `https://openwatcher.ai`
- [x] 桌面下载路由从 official channel 派生
- [x] 官网首页已包含 changelog 区块，并从 `/changelog.json` 拉取数据

## 当前外部阻塞

以下项目不属于公开仓代码直接修复范围，需要在官网 / official 发布面处理后复验：

- [ ] `https://openwatcher.ai/file/beta/apk` 在 2026-06-21 复验时仍返回 `410 Gone`，需要恢复为可下载 APK 或明确移除该公开入口。
- [ ] official 聚合 changelog 中历史 entry 仍可能包含旧字段 `links.channelManifestUrl`；公开仓当前生成脚本已经输出 `links.releaseManifestUrl`，需要 official 同步侧重放或迁移历史记录。

## 后续可追加的真机体验验证

以下项目不再阻塞本次组件级发布模型改造完成，但可作为后续补充样本：

- [ ] 真实 Watch 设备重新执行一次更新检查，确认 UI 中只显示手表相关 changelog
- [ ] 真实 Desktop release 包重新走一遍首启 runtime 下载链路
- [ ] 额外执行 `release_scope=desktop`、`watch`、`runtime-pointer` 的线上复用验收

## 本轮执行顺序

1. 先跑 `publish-runtime.yml`
2. 再跑 `publish-beta.yml`
3. 检查 GitHub Release assets 与 notes
4. 部署 Worker / Pages
5. 检查 `channels/beta.json`、`changelog.json` 和 official 下载 URL
6. 验证 Watch 更新检查
7. 验证 Desktop runtime 下载

## 当前结论

- 本轮已经完成模型落地、脚本改造、Worker/API、Pages 渲染、Desktop/Watch 解析、目标范围内本地测试，以及一次真实 Runtime / Product Release 发布与官网切换
- 当前公开入口已切到新模型：
  - `channels/beta.json`
  - `changelog.json`
  - `changelog/<releaseTag>.json`
