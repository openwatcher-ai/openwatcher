# OpenWatcher 发版经验

本文件只记录高复用、真实踩坑后确认有价值的发布经验，不记录流水账。

## 使用规则

- 每次使用 `openwatcher-release` 结束前，回顾本次遇到的真实问题。
- 如果问题只是一次性偶发情况，不写入本文件。
- 如果同类坑第二次出现，必须把规则补进本文件、Skill 或相关检查脚本。
- 每条经验尽量短，直接写“问题是什么，后续怎么避免”。

## 初始经验

### 2026-06-09

- `beta` 先保持简单：公开 Release 负责真实资产，私有平台只保留小 `channels/beta.json`，不要一开始就把多层稳定入口都铺开。
- `dev` 是私有开发通道，不做公开 GitHub Release，必须鉴权并命中 `deviceToken hash` 白名单。
- 当前没有正式首发历史包袱，更新 metadata 和更新入口可以直接按新契约设计，不为旧公开协议保留兼容层。
- 构建与分发要拆开：正式候选产物优先来自 GitHub Actions，本地直打不再作为默认主流程。
- `dev` 更新入口如果要做鉴权和白名单，手表端请求 metadata、changelog 和 APK 时都必须带 token 头，否则服务端无法同时满足“同一设备校验”和“自动更新体验”。
- `beta` 公共分发更稳的做法是先生成独立的 Pages 站点目录，再把同一批产物同步到固定 GitHub 备用 release，不要把 metadata 拼装、release 上传和站点发布全塞进 workflow 一段内联脚本里。
- 如果后端配置当前只保存“正在使用的那一台手表”，又想做 `dev` allowlist，就必须额外维护本地绑定历史，否则 CLI 永远只能看到当前唯一绑定。
- 这类本地运维入口优先做成 `list` + `add --index` 的非交互式命令，比直接引入交互式菜单更容易脚本化，也更适合在桌面、SSH 和 CI 场景复用。
- `dev` 不应该被实现成“只改更新源的通道”，而应该是一整套运行环境：业务服务 host、更新 host、设备身份都一起切。
- “发送开发环境到手表”更适合放到 Desktop 设置页的开发者 tab，而不是安装向导；它本质上是开发运维动作，不是普通用户的首次安装路径。
- GitHub runner 上不要默认依赖 `rg`。涉及变更范围判定的 workflow 脚本要么先检查 `rg` 是否存在并回退到 `grep`，要么显式安装；否则 `watch-involved` 可能被静默误判成 `desktop-only`。
- 任何要调用仓库脚本或 `gh release` 的 job，都要先 `checkout` 仓库。仅下载 artifact 不会带上 `.git` 和 `scripts/`，脚本会直接找不到，`gh` 也可能因为不在 git 仓库里而失败。
- `gh release upload file#label` 只会改展示标签，不会改 release 下载 URL 使用的真实资产名。只要手表或客户端代码写死了 `.../latest.json` 这种固定路径，就必须上传一个真实文件名也叫 `latest.json` 的资产，不能只靠 label 伪装。

### 2026-06-10

- 发布前残留扫描会检查普通测试代码。需要写旧品牌、私有域名、本机路径等负向样本时，优先放进专用扫描脚本的排除范围或用既有允许文件，不要在新增单测里直接写禁止字面量，避免 preflight 把测试样本当成公开残留。

### 2026-06-11

- 公开发布链路收敛到 `channels/beta.json` 后，`file/beta/latest.json` 只能当兼容层，不能再当主入口；Desktop 和 Watch 都应先读 channel manifest。
- Pages 下载路由要做 302 跳转，目标直接取 channel manifest 里的 GitHub Release asset URL，不要再在站点层拼大文件镜像路径。
- 私有 Worker 只维护小 JSON 时，release sync 的 subrequests 压力会明显下降；真正该避免的是把全量公开资产和 runtime companion 资产继续塞进一次同步里。
- `watch-beta-stable`、`runtime-stable` 和 `current/*` 只适合作为历史说明或短期回滚背景，不要继续写成主流程术语。
- 如果兼容旧客户端必须保留 `file/beta/latest.json`，那它应当从 channel manifest 派生，不能反过来把它当成新的事实来源。
- 新链路里，Desktop 的 runtime 下载和 Watch 的 APK 下载都应该由同一份 channel manifest 驱动，避免 Product Release、Runtime Release、Pages 代理和旧稳定 tag 之间互相抢事实。
- 公开仓发布不能依赖私有 Worker、官网 Pages 或托管隧道控制面。Product Release 只上传 `release-manifest.json`、`checksums.txt`、`release-notes.md`、`changelog-entry.json` 和本次产物；官网 channel 与 R2 发布交给 `ow-official`。
- macOS Desktop 发布包要分清外层资产名和内部 bundle 名：Release 资产可以带平台后缀，但 DMG/ZIP 内部应保持 `OpenWatcher.app`。Wails 的 `build/appicon.png` 如果在 `.gitignore` 下，CI 不会自动带上图标源，发布脚本必须从已跟踪资产生成它，避免回退默认图标。
- 官网下载区展示版本也必须从 `channels/beta.json` 读取，不能保留独立硬编码或旧 `site-data` 事实源；否则 Release 和 manifest 正确，用户看到的下载卡片仍会显示旧版本。
- Wails 的 macOS `build/darwin/Info.plist` 同样在 ignored 的 `build/` 目录里；正式 bundle id 不能只改本机模板，发布脚本要在构建前生成受控模板，避免 CI 继续产出 `com.wails.*`。

### 2026-06-12

- 远程初始化类发布/配置机制要先确认命名、隐私边界和仓库边界；临时配置码只中转 `apiBase` 与 `environment`，不要设计成长期设备身份，也不要默认绑定托管隧道。
- 公开仓 release preflight 不能把 Desktop / Watch 版本固定为首发初始值；首发后应校验 SemVer、`versionCode` 下限和递增事实，否则第二次发布会被旧预检误拦。
- `release_summary` 不能只出现在 Release 正文摘要里，必须同步进入 `changelog-entry.json` 的固定分类，并由 release notes 渲染出来；如果发布后补正 metadata，要同时更新 GitHub Release 正文、`changelog-entry.json`、`release-notes.md`、`checksums.txt` 和官方 changelog 对象。
- 私有 Worker 同步、官网 Pages 部署和开发通道运维不属于公开 `openwatcher` 仓库 Product Release 主流程；相关规则应放在承载 Worker / site 的仓库级 Skill 中。
- GitHub Actions 组件级发布中，如果上游非目标组件 job 会被 skipped，后续发布 job 的 `if` 必须使用 `always()` 加显式 `needs.*.result` 判断；只写普通表达式仍会叠加默认 `success()`，导致 Release 创建或 channel 同步被跳过。

### 2026-06-13

- 判断 `release_scope` 不能只看当前未提交 diff；如果本地 `main` 已领先远端或刚推送了积压提交，必须同时比较上一版 `release-manifest.json` 的组件版本、当前源码版本和目标 commit 的组件 diff。Watch `versionName` / `versionCode` 已递增时，即使当前未提交改动只在 Desktop，也必须选择 `watch` 或 `full`。
- Watch 更新说明展示必须和当前 `changelog.json` schema 一起验收：`watch.changelogUrl` 指向聚合 changelog 时，客户端要按 `components.watch.versionCode` 和 `notes.*[].component == "手表应用"` 解析，不能只测旧 `file/beta/changelog.json` 的 `entries[].summary`。
- Watch APK 下载失败诊断不能只看更新检查 HTTP 200；如果诊断包里有 `download_app_update`，但没有 `apk_update` 进度、`download_completed`、安装权限或安装器事件，应优先核对 GitHub Release 资产域名在手表网络下是否可用，并补记录 APK 下载 HTTP 状态、IOException 类型和正文读取进度。
- Watch `0.2.1` 检查更新时，如果 `channels/beta.json.watch.fallbackDownloadUrl` 为空，会在主入口 200 后继续同步请求 GitHub Releases API 和 release asset 中的 `channel-beta.json` 来补备用下载地址；手表访问 GitHub 不稳时会把“检查更新”也拖成超时。后续公开版本不再兼容这条旧链路，Watch beta 检查只读 `openwatcher.ai/channels/beta.json`，客户端可消费下载地址只能来自 official channel。
- official 编排模式下，公开仓 `release-manifest.json` 不能写任何客户端 `downloadUrl`、`fallbackDownloadUrl`、`runtime.manifestUrl` 或 GitHub asset 基地址；这些字段一旦回到公开 manifest，就会让后续 agent 误以为 GitHub 仍是客户端下载源。
- 公开仓 preflight 会阻止 `docs/superpowers/` 等内部设计路径；official 编排设计应沉淀到公开 Skill、发布契约或 official 仓文档，不能把内部设计文档留在公开仓。
- 组件级发布复用上一版 manifest 时，不能原样复制旧版本里的 `downloadUrl`、`fallbackDownloadUrl`、`manifestUrl` 或 `assetBaseUrl`；复用组件只应保留版本、artifact、sha256、size 等事实字段，official URL 由 official 仓发布 channel 时补齐。

### 2026-06-14

- Desktop 前端 fallback/mock 数据也会进入公开仓残留扫描和打包产物；示例 Codex 路径不要写 `/Users/example/...`，应使用 `~/.codex`、`CODEX_HOME` 这类不会被当作本机路径的占位值。
- Desktop 有用户可见更新时必须在 workflow input 或 `OPENWATCHER_DESKTOP_VERSION` 中传入新的 Desktop 版本，否则 official channel 即使换了安装包，Desktop 更新检查也会因为版本号相同而提示“已是最新版本”。
- 如果上一版 beta 已发布某个 Watch `versionName` / `versionCode`，后续目标 commit 又包含 Watch 代码、资源、依赖、配置或服务端契约行为变化，发布前必须通过 workflow input 或 `OPENWATCHER_WATCH_VERSION_NAME` / `OPENWATCHER_WATCH_VERSION_CODE` 传入递增后的 Watch 版本，不能复用上一版 APK 版本重打 release。

### 2026-06-16

- 重建公开仓后，仓库内 `watch-app/RELEASE_BUILDS.md` 可能仍带有旧私有构建记录；首版和后续发版都不能参考这类仓库内构建表决定版本号、组件复用或 changelog 历史。正式事实源必须是最新公开 GitHub Release 的 `release-manifest.json` 和 official 仓发布的 channel metadata。
- 公开资产文件名不要同时塞入组件版本、release tag、commit 和构建时间；人读文件名只需要组件、版本和必要平台/架构，完整追溯事实应放在 `release-manifest.json`、组件 metadata、GitHub Release 和 `checksums.txt` 中。
