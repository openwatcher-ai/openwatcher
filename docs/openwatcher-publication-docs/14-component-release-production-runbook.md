# OpenWatcher Component Release 生产执行手册

本文档用于把组件级发布模型真正切到线上。它只记录已经验证过的事实、当前线上现状、需要执行的顺序和每一步完成后应看到的证据。

## 当前结论

截至 2026-06-12，本地代码、脚本、Worker、Pages、Desktop、Watch 的目标范围改造已经完成，并完成了一次真实 Runtime / Product Release 发布与官网切换。

当前线上证据：

- `https://openwatcher.ai/channels/beta.json`
  - 已返回新 schema
  - 当前 `revision = 1`
  - 当前 `release.tag = "beta-2026.06.12.1"`
  - 不再包含 `product.version`
- `https://openwatcher.ai/changelog.json`
  - 已返回 JSON changelog
  - 当前 `entries[0].id = "beta-2026.06.12.1"`

当前仓库与远端差异：

- 公开仓 `<public-openwatcher-repo>`
  - 已推送到 `origin/main`
- 私有仓 `<private-platform-repo>`
  - 已推送到 `origin/openwatcher-pub-pre`

额外事实：

- 公开仓 `main` 当前未启用 GitHub branch protection
- `gh auth status` 可用
- `wrangler whoami` 可用
- Cloudflare Pages 项目名：`openwatcher-ai`
- control-plane Worker 已部署
- Pages 项目 `openwatcher-ai` 已部署

## 已验证的本地脚本入口

公开仓：

- `scripts/trigger-publish-runtime-workflow.sh`
- `scripts/trigger-publish-beta-workflow.sh`
- `scripts/test-release-product-scripts.sh`

私有仓：

- `scripts/deploy-openwatcher-control-plane.sh`
- `scripts/deploy-openwatcher-pages.sh`
- `scripts/verify-openwatcher-release-surface.sh`

## 生产执行顺序

### 1. 推送公开仓 `main`

在 `<public-openwatcher-repo>`：

```bash
git push origin main
```

完成证据：

- `gh run` 读取到的 workflow 源文件已经对应当前本地提交
- `gh workflow view 'OpenWatcher Publish Beta' --yaml` 和 `publish-runtime` 对应的是新版本

### 2. 触发 Runtime Release

建议先发新的 Runtime Release：

```bash
cd <public-openwatcher-repo>
scripts/trigger-publish-runtime-workflow.sh v0.1.1 36.0.0 2026.6.0
```

完成后核对：

```bash
gh run list --workflow 'OpenWatcher Publish Runtime' --limit 5
gh release view runtime-v0.1.1
```

必须看到：

- 新的 `runtime-v*` Release
- `runtime-manifest.json`
- `runtime-checksums.txt`

### 3. 触发 Product Release

再发组件级 beta：

```bash
cd <public-openwatcher-repo>
scripts/trigger-publish-beta-workflow.sh full '组件级发布模型首个线上切换版'
```

如果只是验证组件复用，再分别补跑：

```bash
scripts/trigger-publish-beta-workflow.sh desktop '仅更新 Desktop，验证 Watch 事实复用'
scripts/trigger-publish-beta-workflow.sh watch '仅更新 Watch，验证 Desktop 事实复用'
scripts/trigger-publish-beta-workflow.sh runtime-pointer '仅更新 Runtime 指针'
```

完成后核对：

```bash
gh run list --workflow 'OpenWatcher Publish Beta' --limit 5
gh release list --repo openwatcher-ai/openwatcher --limit 10
```

必须看到：

- 新 tag 为 `beta-YYYY.MM.DD.N`
- Release asset 包含：
  - `release-manifest.json`
  - `changelog-entry.json`
  - `release-notes.md`
  - `checksums.txt`

### 4. 部署 control-plane Worker

在 `<private-platform-repo>`：

```bash
scripts/deploy-openwatcher-control-plane.sh deploy
```

完成证据：

- `wrangler deployments list --config workers/openwatcher-control-plane/wrangler.jsonc`
 里出现新 deployment

### 5. 部署 Pages

在 `<private-platform-repo>`：

```bash
scripts/deploy-openwatcher-pages.sh
```

完成证据：

- `wrangler pages project list` 中 `openwatcher-ai` 最近修改时间更新
- 官网内容与本地 `site/openwatcher-pages` 一致

## 线上验收命令

### 自动化入口

```bash
cd <private-platform-repo>
scripts/verify-openwatcher-release-surface.sh
```

这一步必须通过。若失败，优先以脚本输出作为阻塞证据。

### 手工核对

```bash
curl -fsSL https://openwatcher.ai/channels/beta.json | jq .
curl -fsSL https://openwatcher.ai/changelog.json | jq .
curl -I -L https://openwatcher.ai/download/desktop/macos-arm64
```

必须满足：

- `channels/beta.json` 有 `revision`
- `channels/beta.json.release.tag` 以 `beta-` 开头
- `channels/beta.json` 不再包含 `product.version`
- `channels/beta.json` 的客户端 `downloadUrl` / `manifestUrl` 都以 `https://openwatcher.ai/` 开头
- `changelog.json` 返回 JSON，不再是 HTML
- Desktop 下载路由从 official channel 派生

## 客户端验收

### Watch

- 真实检查更新时，主入口读取 `/channels/beta.json`
- 不再请求 GitHub Releases API 或 GitHub Release asset 作为更新检查来源
- changelog 只显示手表相关条目

### Desktop

- official channel 的 `runtime.manifestUrl` 与 `runtime.manifestSha256` 仍能驱动首次下载
- runtime 包解析 changelog 时，只取：
  - 【桌面应用】
  - 【运行时依赖】
  - 【兼容性】
  - 必要时【文档】

## 常见阻塞

### `channels/beta.json` 仍是旧 schema

说明：

- `publish-beta` 还没跑到线上
- 或 Worker / Pages 还没部署

### `changelog.json` 返回 HTML

说明：

- Pages `_worker.js` 还没部署
- 或 control-plane Worker 没有新接口

### 公开仓 workflow 没有新输入

说明：

- 本地提交还没 push 到 `origin/main`

### Worker dry-run 通过，但线上未变

说明：

- 只做了 dry-run，没有 `deploy`

## 本轮实际结果

- [x] push 公开仓 `main`
- [x] 真实触发 `publish-runtime`
- [x] 真实触发 `publish-beta`
- [x] 部署 control-plane Worker
- [x] 部署 Pages
- [x] `scripts/verify-openwatcher-release-surface.sh` 通过

当前线上发布结果：

- Runtime Release：`runtime-v0.1.1`
- Product Release：`beta-2026.06.12.1`
- Worker 公开入口：
  - `https://api.worker.openwatcher.ai/channels/beta.json`
  - `https://api.worker.openwatcher.ai/changelog.json`
- 官网公开入口：
  - `https://openwatcher.ai/channels/beta.json`
  - `https://openwatcher.ai/changelog.json`
