# OpenWatcher 发布契约

本文件固定公开仓 `openwatcher` 的发版边界。当前模型是：

- GitHub Product Release 是公开事实记录、历史归档和人工下载入口。
- `release-manifest.json` 是公开仓流水线的主输出，用于让 `ow-official` 校验并发布官网当前下载对象。
- 客户端更新检查只使用 `https://openwatcher.ai/channels/beta.json`。
- 客户端下载 Watch APK、Desktop 安装包和 Runtime 资源时，只使用 `https://openwatcher.ai` URL。
- 公开仓 workflow 不调用 official Worker，不写 R2，不校验官网当前路径。

`ow-official` 负责读取公开 release 事实、上传或复用 R2 当前对象、生成 `channels/beta.json`、写入 changelog 并验证官网路径。

## 1. Product Release 规则

Product Release tag 固定为发布记录号：

```text
beta-YYYY.MM.DD.N
```

约束：

- `N` 是当天递增序号。
- Product Release tag 不代表统一产品语义版本。
- 客户端更新判断不依赖 Product Release tag。
- Desktop / Watch 组件版本必须来自发布 workflow input、打包脚本环境变量或已验证产物 metadata，不能从源码文件中的固定版本号读取。
- 版本递增、组件复用和发布范围判断必须以最新公开 GitHub Release 的 `release-manifest.json` 以及 official 仓发布的 channel metadata 为准；仓库内 `watch-app/RELEASE_BUILDS.md` 只作为人工审计日志，不能作为正式版本事实来源。
- 正式发布产物必须来自 GitHub Actions。
- 未变化组件必须明确标记为 `reused` 或 `not_included`，不得重新构造下载 URL。

## 2. 公开 release 事实包

公开 Product Release 至少包含：

- `release-manifest.json`
- `checksums.txt`
- `release-notes.md`
- `changelog-entry.json`
- 本次真正更新的 Watch APK、Desktop 安装包、backend 或其他公开产物
- `THIRD_PARTY_NOTICES.md`

公开 Product Release 不生成客户端 `channel-beta.json`。如果有人手动运行 `scripts/generate-channel-manifest.sh`，脚本应直接失败并提示 channel 由 `ow-official` 生成。

## 3. `release-manifest.json`

`release-manifest.json` 是不可变公开事实，不是客户端更新入口。

必须包含：

- `schemaVersion`
- `channel`
- `release.tag`
- `release.scope`
- `release.summary`
- `release.commit`
- `release.branch`
- `product.name`
- `product.repository`
- `components.*.status`
- `watch.versionName`
- `watch.versionCode`
- `watch.artifact`
- `watch.sha256`
- `watch.sizeBytes`
- `desktop.version`
- `desktop.platforms.*.artifact`
- `desktop.platforms.*.sha256`
- `desktop.platforms.*.sizeBytes`
- `runtime.releaseTag`
- `runtime.manifestArtifact`
- `runtime.manifestSha256`
- `artifacts[]`
- `checksums.artifact`

不得包含：

- `watch.downloadUrl`
- `watch.fallbackDownloadUrl`
- `desktop.platforms.*.downloadUrl`
- `runtime.manifestUrl`
- `runtime.assetBaseUrl`
- `product.releaseAssetBaseUrl`
- 任何 `release-assets.githubusercontent.com` URL
- 任何供 Watch 或 Desktop 直接消费的 GitHub 下载地址

`ow-official` 可根据 `release.tag`、`product.repository` 和 artifact 文件名读取 GitHub Release asset，再把需要的对象发布到 R2。

## 4. 组件状态与发布范围

固定组件：

- `desktop`
- `watch`
- `runtime`
- `compatibility`
- `docs`

组件状态只能是：

- `updated`
- `reused`
- `not_included`

`publish-beta.yml` 的 `release_scope` 固定取值：

```text
full | desktop | watch | runtime-pointer | docs | compatibility | metadata
```

约束：

- `full` 构建 Desktop、Watch 与 backend/sidecar，并引用当前 Runtime Release。
- `desktop` 只更新 Desktop，Watch 和 Runtime 事实复用上一版 release manifest。
- `watch` 只更新 Watch APK，Desktop 和 Runtime 事实复用上一版 release manifest。
- `runtime-pointer` 不构建 Runtime，只把公开事实指向指定 Runtime Release。
- `docs`、`compatibility`、`metadata` 不改变下载事实。
- 首个公开版本必须使用 `full`。

## 5. `changelog-entry.json`

`changelog-entry.json` 是本次发布记录的结构化说明，至少包含：

- `schemaVersion`
- `channel`
- `id`
- `publishedAt`
- `scope`
- `components`
- `notes`
- `links.releaseUrl`
- `links.releaseManifestUrl`

约束：

- release notes Markdown 和 `changelog-entry.json` 必须来自同一组语义。
- 相同 `id` 的补发布应覆盖旧 entry，保证幂等。
- `links.releaseManifestUrl` 指向 GitHub Release 中的 `release-manifest.json`。
- 不再写 `links.channelManifestUrl`。

## 6. 官网 channel 契约

官网 channel 只由 `ow-official` 生成：

- `https://openwatcher.ai/channels/beta.json`

客户端可消费字段中的 `downloadUrl` 与 `manifestUrl` 必须以 `https://openwatcher.ai/` 开头。不得出现：

- `github.com`
- `api.github.com`
- `release-assets.githubusercontent.com`

公开仓只需保证 `release-manifest.json` 有足够的 artifact 名称、大小和 sha256，供 `ow-official` 生成官网 channel。

## 7. 必过验证

公开仓 `beta` workflow 必须验证：

- `release-manifest.json` schema 正确。
- `checksums.txt` 覆盖所有公开 release 文件。
- manifest 中的 `sizeBytes` 和 `sha256` 与真实文件一致。
- release notes 和 `changelog-entry.json` 与组件状态一致。
- manifest 不包含客户端下载 URL、GitHub asset 基地址或旧 `fallbackDownloadUrl` 字段。
- GitHub Release 附件完整。

`ow-official` 的官网/R2 验证不在公开仓 workflow 中执行。
