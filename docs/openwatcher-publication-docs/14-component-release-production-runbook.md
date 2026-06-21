# OpenWatcher 正式发布执行手册

本文档记录正式发布初始化后的执行顺序。清理前的旧 beta、旧 runtime、旧官网 changelog 和旧 workflow run 不作为当前发布事实保留。

## 固定边界

- 公开仓 `openwatcher-ai/openwatcher` 只生成 GitHub Release fact package。
- `ow-official` 只消费公开 release fact package，并发布 `openwatcher.ai` 的 channel、changelog 和 current 下载对象。
- 正式产物必须来自 GitHub Actions，不使用本地临时构建作为公开产物。
- 首个正式 Product Release 使用 `release_scope=full`。
- 所有组件初始版本号为 `0.1.0`；Watch `versionCode` 为 `10000`。

## 生产执行顺序

### 1. 准备公开仓 main

```bash
scripts/test-openwatcher-preflight.sh
scripts/test-release-product-scripts.sh
```

完成条件：

- 目标提交已经通过 PR 合入 `main`。
- `main` 分支保护要求 PR、发布门禁和管理员手动批准。

### 2. 清理公开仓历史发布面

清理对象：

- 旧 GitHub Release。
- 旧 release tag。
- 旧 GitHub Actions run。

完成条件：

- `gh release list --repo openwatcher-ai/openwatcher` 不再列出旧 release。
- 旧 release tag 不再存在。
- 旧 workflow run 不再出现在 repo 可见运行列表中。

### 3. 清理 official beta 发布面

先 dry-run：

```bash
gh workflow run reset-openwatcher-official-beta.yml \
  -R loccen/ow-official \
  -f dry_run=true
```

确认后真实执行：

```bash
gh workflow run reset-openwatcher-official-beta.yml \
  -R loccen/ow-official \
  -f dry_run=false \
  -f confirmation=RESET_OPENWATCHER_OFFICIAL_BETA \
  -f reason="formal release initialization"
```

完成条件：

- `https://openwatcher.ai/channels/beta.json` 返回明确 404 JSON。
- `https://openwatcher.ai/changelog.json` 返回明确 404 JSON。

### 4. 触发 Runtime Release

```bash
gh workflow run "OpenWatcher Publish Runtime" \
  -R openwatcher-ai/openwatcher \
  --ref main \
  -f runtime_version=v0.1.0 \
  -f desktop_min_version=0.1.0 \
  -f watch_version_name=0.1.0 \
  -f watch_version_code=10000 \
  -f platform_tools_version=<verified-platform-tools-version> \
  -f cloudflared_version=<verified-cloudflared-version>
```

完成条件：

- `runtime-v0.1.0` Release 存在。
- `runtime-manifest.json` 与 `runtime-checksums.txt` 存在并通过校验。

### 5. 触发 Product Release

```bash
gh workflow run "OpenWatcher Publish Beta" \
  -R openwatcher-ai/openwatcher \
  --ref main \
  -f release_scope=full \
  -f release_summary="OpenWatcher 初始正式发布，包含 Desktop、Watch、运行时依赖和本机服务组件。" \
  -f desktop_version=0.1.0 \
  -f watch_version_name=0.1.0 \
  -f watch_version_code=10000
```

完成条件：

- 新 `beta-YYYY.MM.DD.N` Product Release 存在。
- Release assets 包含 `release-manifest.json`、`checksums.txt`、`release-notes.md`、`changelog-entry.json` 和 `THIRD_PARTY_NOTICES.md`。
- Release manifest、checksums、notes 与真实产物一致。

### 6. 发布 official beta

```bash
gh workflow run publish-openwatcher-official-beta.yml \
  -R loccen/ow-official \
  -f release_tag=<new-beta-tag> \
  -f cleanup_current=true
```

完成条件：

- `channels/beta.json` 指向新的 Product Release。
- `changelog.json` 只有初始发布一条记录。
- Desktop、Watch 和 Runtime 下载 URL 全部指向 `https://openwatcher.ai`。

### 7. 验证官网与下载

```bash
gh workflow run verify-openwatcher-official-surface.yml \
  -R loccen/ow-official \
  -f branch=main

curl -fsSL https://openwatcher.ai/channels/beta.json | jq .
curl -fsSL https://openwatcher.ai/changelog.json | jq .
curl -I -L https://openwatcher.ai/download/desktop/macos-arm64
curl -I -L https://openwatcher.ai/download/desktop/windows-amd64
curl -I -L https://openwatcher.ai/file/beta/apk
```

必须满足：

- `changelog.json.entries | length == 1`。
- `changelog.json.entries[0].id` 等于当前 Product Release tag。
- Desktop 下载路由最终返回 200。
- Watch APK 下载入口最终返回 200。
- Runtime manifest 内部资源 URL 均为 `https://openwatcher.ai/downloads/beta/current/runtime/...`。

## 结果记录

正式发布完成后，记录：

- Runtime Release tag。
- Product Release tag。
- 发布 commit。
- GitHub Actions run id。
- official publish run id。
- 官网验证 run id。
- Desktop、Watch 和 Runtime 关键下载对象的 sha256 与 sizeBytes。
