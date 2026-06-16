# GitHub Release 打包与 official 发布分工

## 角色边界

`openwatcher` 公开仓只负责生成公开事实包并发布 GitHub Product Release：

- 构建 Watch APK、Desktop 安装包和必要的 backend/sidecar 产物。
- 引用当前 Runtime Release 的 tag 与 `runtime-manifest.json` sha256。
- 生成 `release-manifest.json`、`checksums.txt`、`release-notes.md`、`changelog-entry.json`。
- 发布 GitHub Release，作为事实记录、历史归档和人工下载入口。

`ow-official` 私有仓负责官网与 R2：

- 读取公开仓 HEAD 与上一版 release commit 的 diff，判断 `release_scope`。
- 触发公开仓 `publish-beta.yml`。
- 读取新 GitHub Release 中的 `release-manifest.json`、`checksums.txt`、`changelog-entry.json` 和产物。
- 上传或复用 R2 当前对象。
- 生成 `https://openwatcher.ai/channels/beta.json`、`changelog.json` 和单 release changelog。
- 验证官网下载 URL、对象大小和 sha256。

公开仓 workflow 不调用 official Worker，不写 R2，不校验官网路径。

## Product Release

Product Release tag 固定为发布记录号：

```text
beta-YYYY.MM.DD.N
```

它不代表统一产品版本。客户端更新判断只看组件版本和 official channel 中的 sha256、size 等事实。

每次 Product Release 至少包含：

```text
release-manifest.json
checksums.txt
release-notes.md
changelog-entry.json
THIRD_PARTY_NOTICES.md
本次真正更新的 Watch APK / Desktop 安装包 / backend 产物
```

公开仓不生成客户端 `channel-beta.json`。

## Release Scope

`publish-beta.yml` 输入固定为：

```text
full | desktop | watch | runtime-pointer | docs | compatibility | metadata
```

语义：

- `full`：构建 Desktop、Watch、backend/sidecar，并引用当前 Runtime Release。
- `desktop`：只更新 Desktop，Watch 与 Runtime 事实复用上一版 `release-manifest.json`。
- `watch`：只更新 Watch APK，Desktop 与 Runtime 事实复用上一版 `release-manifest.json`。
- `runtime-pointer`：只更新 Runtime Release 引用。
- `docs`：只更新公开文档与 release notes。
- `compatibility`：只更新兼容性说明或 manifest 结构。
- `metadata`：只更新结构化发布说明，不改变下载事实。

首个公开版本必须使用 `full`。

## Release Manifest

`release-manifest.json` 是公开 release 事实，不是客户端更新入口。

它必须提供：

- release tag、scope、summary、commit、branch
- repository
- components 状态
- Watch artifact、versionName、versionCode、sha256、sizeBytes
- Desktop 各平台 artifact、sha256、sizeBytes
- Runtime releaseTag、manifestArtifact、manifestSha256
- 本次 GitHub Release 实际上传的 artifacts 列表

它不得包含：

- Watch 或 Desktop 的客户端下载 URL
- `fallbackDownloadUrl`
- `runtime.manifestUrl`
- GitHub asset 基地址
- `release-assets.githubusercontent.com`

`ow-official` 会根据 manifest 中的 artifact 名称和 sha256 读取 GitHub Release asset，并生成 official channel 中的官网 URL。

## Changelog

公开仓每次发布 `changelog-entry.json`。它至少包含：

- `id`
- `publishedAt`
- `scope`
- `components`
- `notes`
- `links.releaseUrl`
- `links.releaseManifestUrl`

官方站点聚合入口由 `ow-official` 写入：

```text
https://openwatcher.ai/changelog.json
https://openwatcher.ai/changelog/<releaseTag>.json
```

## Runtime

Runtime Release 仍由公开仓发布，但客户端不直接消费 GitHub Runtime manifest。

`ow-official` 发布官网 channel 时应把 Runtime manifest 及其资源发布到 R2 当前对象，并确保 official channel 的 `runtime.manifestUrl` 与 runtime manifest 内部资源 URL 都指向 `https://openwatcher.ai`。

## 验证

公开仓必须验证：

- `release-manifest.json`、`checksums.txt`、`release-notes.md`、`changelog-entry.json` 均存在。
- `checksums.txt` 覆盖所有 release 文件。
- manifest 中的 sha256 和 sizeBytes 与真实文件一致。
- release notes 与 `changelog-entry.json` 的组件状态一致。
- manifest 不包含客户端 GitHub 下载 URL 或旧 `fallbackDownloadUrl` 字段。

官网和 R2 验证由 `ow-official` 执行。
