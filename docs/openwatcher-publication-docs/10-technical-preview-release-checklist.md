# OpenWatcher 正式发布 Checklist

本文档用于正式发布初始化后的公开验收。它不保留清理前旧 beta、旧 runtime 或旧 changelog 作为当前发布记录。

## 发布前检查

- [ ] 公开仓 `main` 已通过 PR 合入目标提交。
- [ ] `main` 分支保护已要求 PR、发布门禁和管理员手动批准。
- [ ] 公开仓旧 GitHub Release、旧 tag 和旧 workflow run 已清理。
- [ ] `ow-official` official beta reset 已 dry-run 并真实执行。
- [ ] `https://openwatcher.ai/channels/beta.json` 在 reset 后返回明确 404 JSON。
- [ ] `https://openwatcher.ai/changelog.json` 在 reset 后返回明确 404 JSON。
- [ ] release-facing 文件公开残留扫描通过。

## Runtime Release

- [ ] 从最新 `main` 触发 `OpenWatcher Publish Runtime`。
- [ ] Runtime 版本为 `v0.1.0`。
- [ ] Desktop 最低版本为 `0.1.0`。
- [ ] Watch 版本为 `0.1.0`，`versionCode` 为 `10000`。
- [ ] GitHub Runtime Release 包含 `runtime-manifest.json` 与 `runtime-checksums.txt`。
- [ ] Runtime manifest 内部资源 sha256 和 size 与真实资产一致。

## Product Release

- [ ] 从最新 `main` 触发 `OpenWatcher Publish Beta`，`release_scope=full`。
- [ ] Desktop 版本为 `0.1.0`。
- [ ] Watch 版本为 `0.1.0`，`versionCode` 为 `10000`。
- [ ] GitHub Product Release 包含 `release-manifest.json`、`checksums.txt`、`release-notes.md`、`changelog-entry.json` 和 `THIRD_PARTY_NOTICES.md`。
- [ ] Product Release notes 与 `changelog-entry.json` 语义一致。
- [ ] Watch APK 为 release APK，不是 debug APK；ABI、签名和 sha256 已验证。

## Official 发布面

- [ ] `ow-official` 已消费新的 Product Release fact package。
- [ ] `cleanup_current=true` 后，R2 current 只保留当前 channel 引用对象。
- [ ] `https://openwatcher.ai/channels/beta.json` 指向最新 Product Release。
- [ ] `https://openwatcher.ai/changelog.json` 只有初始发布一条记录。
- [ ] `https://openwatcher.ai/changelog/<releaseTag>.json` 返回同一条记录。
- [ ] Desktop 下载路由最终能下载当前安装包。
- [ ] Watch APK 兼容下载入口最终能下载当前 APK。
- [ ] Runtime manifest 及其内部资源 URL 全部使用 `https://openwatcher.ai`。
- [ ] `ow-official` 发布面验证 workflow 通过。

## 验收结论

正式发布完成后，在这里记录本次 Product Release tag、Runtime Release tag、发布 commit、官网验证 run 和关键下载校验摘要。
