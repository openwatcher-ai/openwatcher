# Watch APK Release Gate

本文件是 `openwatcher-release` 的条件引用。只有在构建、打包、交付、发布、发送或暴露 OpenWatcher Watch release APK 时读取。

## 硬规则

- 不要用未变化的 `versionName` 或复用的 `versionCode` 构建新的 release APK。
- 不要把未经最终复制或包装后验证的 APK 描述成最新 release。
- 不要把 debug APK 作为公开发布产物。
- 不要把聊天传输、本地临时文件或 `watch-app/app/build/outputs/apk/...` 下的 Gradle 输出当作主 release artifact。
- 每次执行 release package 都必须提供 `RELEASE_SUMMARY`，内容描述用户可感知变化，不能回退到最新 git commit 标题。
- release artifact 文件名保持短名，Watch Product Release 使用 `watchapp_v<versionName>.apk`，Runtime 随附 Watch APK 使用 `watchapp-runtime_v<versionName>.apk`；release tag、commit、构建时间和 sha256 放在 manifest、metadata 与 checksums 中，不塞进文件名。
- Release notes、metadata、headers、文件名和文档不得重新引入旧私有品牌、私有域名、本机用户路径或旧包名。

## 版本纪律

构建 release 前必须显式选择并传入 Watch APK 版本：

- `versionCode` 必须比上一版 release 严格递增至少 1。
- `versionName` 使用类似 SemVer 的 `major.minor.patch`。
- 小修复和视觉微调用 patch。
- 兼容的 UI、行为、API 消费、配对、交付或 workflow 变化用 minor。
- 不兼容的配对、存储、token、包身份或服务端契约变化才用 major。
- 如果 release 构建失败后从同一 commit 重试，且 app code、资源、依赖、配置、服务端契约行为都没有变化，可以复用同一版本，但要记录为构建重试。
- 不要通过修改 `watch-app/app/build.gradle.kts` 来升版本；release 版本必须通过 `OPENWATCHER_WATCH_VERSION_NAME` 和 `OPENWATCHER_WATCH_VERSION_CODE`，或 GitHub Actions 的 `watch_version_name` / `watch_version_code` input 传入。
- `watch-app/app/build.gradle.kts` 只负责消费构建输入和生成本地 dev 标识，不应保存具体 release 版本号。
- 判断“上一版 release”和递增版本时，只能读取最新公开 GitHub Release 的 `release-manifest.json`，并结合 official 仓发布的 channel metadata。`watch-app/RELEASE_BUILDS.md`、`dist/latest-apk.json` 和本地历史构建输出都不能作为正式版本事实来源。

## 必须流程

1. 检查当前状态：

```bash
git status --short
sed -n '1,120p' watch-app/RELEASE_BUILDS.md
gh release list --repo openwatcher-ai/openwatcher --limit 20 || true
cat dist/latest-apk.json 2>/dev/null || true
cat dist/latest-apk-changelog.json 2>/dev/null || true
```

`watch-app/RELEASE_BUILDS.md` 和 `dist/latest-apk*.json` 在这里仅用于发现本地残留或审计输出，不参与版本号选择。

如果只是本地模拟器试装 release APK，且主工作区存在无关未提交改动，不要为了通过打包脚本而暂存、stash 或回退用户改动。可以从目标提交创建干净临时 worktree，在该 worktree 中复制本机忽略的 `watch-app/local.properties` 和签名文件后运行打包脚本；最终 artifact、metadata 和构建记录再同步回主工作区。最终答复必须说明这是本地验证构建，不是公开发布。

2. 如果代码、资源、依赖、配置或服务端契约行为自上一版公开 release metadata 后有变化，在任何 release build 命令前确定新的 `versionName` 和 `versionCode`，并准备通过构建输入传入。

3. 运行目标范围测试，至少：

```bash
cd watch-app && ./gradlew :app:testDebugUnitTest
```

4. 如果 packaging script 要求 tracked worktree 干净，先提交代码改动；不要为了升版本提交 Gradle 版本号改动。

5. 从本次真实可见变化起草面向用户的 `RELEASE_SUMMARY`。不要写 commit 标题、类名、内部任务名或纯实现细节。

6. 优先通过仓库 release 脚本构建最终 artifact：

```bash
RELEASE_SUMMARY="这里写用户可感知的更新说明" \
OPENWATCHER_WATCH_VERSION_NAME="<versionName>" \
OPENWATCHER_WATCH_VERSION_CODE="<versionCode>" \
scripts/package-watch-release.sh <short-release-slug>
```

Slug 规则：短而描述性，例如 `release`、`ui-fix`、`pairing-fix`。不要包含 `v`、`versionName`、时间戳或 commit id；追溯信息由 metadata、manifest 和 checksums 保存。
脚本只用 slug 判断是否为 Runtime 随附 APK，不会把 slug 写进最终文件名。

7. 验证最终 artifact，而不是只验证 Gradle 原始输出：

```bash
cat dist/latest-apk.json 2>/dev/null || true
cat dist/latest-apk-changelog.json 2>/dev/null || true
cat dist/<apk-file>.sha256
cat dist/<apk-file>.libs.txt

APK_SIGNER="$(
  { if [[ -n "${ANDROID_HOME:-}" ]]; then find "$ANDROID_HOME/build-tools" -type f -name apksigner 2>/dev/null; fi
    find "$HOME/Library/Android/sdk/build-tools" -type f -name apksigner 2>/dev/null; } |
    sort | tail -1
)"
"$APK_SIGNER" verify -v --print-certs dist/<apk-file>
```

`dist/latest-apk.json` 中的 `artifact`、changelog metadata、`.sha256` 文件、ABI 列表和验签 APK 文件名必须都指向 `dist/` 下同一个最终文件。任何一项失败，都不能告诉用户 release APK 已准备好。

8. 在 `watch-app/RELEASE_BUILDS.md` 记录 release。

此记录只是人工审计日志。CI 生成的 `latest-apk-changelog.json` 默认只包含本次构建说明，不从仓库内历史构建表聚合旧版本说明；正式跨版本 changelog 由 Product Release 的 `changelog-entry.json` 和 official channel changelog 承担。

必须包含：

- UTC build time
- `versionName`
- `versionCode`
- Git commit
- branch
- APK filename
- SHA256
- summary type
- user-visible summary

9. 除非用户明确要求，或未来公开发布流明确 vendored release artifacts，否则忽略的 `dist/` artifacts 不入库。

## 公开发布安全检查

最终回复或公开发布前，扫描 release-facing 文件和 metadata：

```bash
rg -n "top\\.uuss|Codex Watcher|codex-watcher|CODEX_WATCHER|X-Codex-Watcher|BuildConfig\\.WATCHER_BASE_URL|watcher\\.uuss\\.top|/Users/" \
  AGENTS.md README.md docs watch-app desktop-app scripts .github 2>/dev/null || true
```

剩余命中必须是历史迁移说明、显式负向检查，或不会进入公开 artifact 的内部源码包名。不要在旧品牌或私有基础设施出现在用户可见 release 文件时发布。

## 最终答复要求

涉及 Watch APK 交付时，最终答复必须报告：

- APK filename
- `versionName` 和 `versionCode`
- APK 使用的 Git commit
- `RELEASE_SUMMARY`
- `.sha256` 文件中的 SHA256
- `.libs.txt` 中的 ABI 列表
- 最终 artifact 的 `apksigner verify` 结果
- 任何下载 URL 或分发路径，且只能在确认它指向同一个已验证 artifact 后报告
