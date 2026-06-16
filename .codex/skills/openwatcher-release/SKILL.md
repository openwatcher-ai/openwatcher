---
name: openwatcher-release
description: Use whenever Codex needs to diagnose, implement, plan, execute, or refine OpenWatcher public release workflows in the openwatcher repository, including beta Product Releases, Runtime Releases, component-scoped release metadata, watch APK release gates, GitHub Actions workflows, release notes, channel manifests, changelog entries, artifact validation, and post-release skill evolution.
---

# OpenWatcher 发版总控

## 作用

这个 Skill 是 OpenWatcher 开源仓库的发版总入口。它不是单纯“跑某个脚本”，而是先判断用户到底要做哪一类发布工作，再选择完整流程或局部流程。

适用场景包括：

- 公开 `beta` 发版
- Runtime Release
- 组件级 `beta` Product Release
- 手表更新链路改造
- GitHub Actions / GitHub Release 发布机制实现
- 手表 release APK 版本、签名、ABI、SHA256 和记录校验
- 发布诊断、补发、重发
- 发布经验回灌和 Skill 自进化

如果本次工作会构建、打包、交付或公开 OpenWatcher 手表 release APK，或者 `release_scope` 是 `watch` / `full`，读取 `references/watch-apk-release-gate.md`，把它当作 watch APK release gate。本仓库不再维护单独的 `watch-release-apk-build` Skill。

## 先做什么

先读这些文件：

- `references/intent-matrix.md`
  用于判断用户意图和是否直接执行
- `references/release-contract.md`
  用于确认 `beta`、Runtime、metadata 和验证项
- `references/release-notes.md`
  用于生成和审核公开 GitHub Release notes，确保每条变更都有类型、组件和可追溯说明
- `references/watch-apk-release-gate.md`
  仅在本次涉及 Watch APK 构建、打包、发布或交付时读取

只有在本次工作结束前复盘或需要沉淀经验时，再读：

- `references/release-lessons.md`

如果当前任务是在实现或修改发布机制，再同时参考仓库内已确认设计：

- `docs/superpowers/specs/2026-06-11-openwatcher-component-release-model-and-changelog.md`
- `docs/superpowers/specs/2026-06-09-openwatcher-release-skill-design.md`

## 硬规则

- 意图清晰时直接执行，不要把本可直接完成的发版动作改写成征询句。
- 意图模糊时，只澄清真正会改变结果的问题，不要泛泛追问。
- 不要绕过现有 release gate 直接宣称产物可交付。
- 默认优先消费 GitHub Actions 产物，不要把本地临时构建当作正式分发主流程。
- 公开发布说明必须按 `references/release-notes.md` 生成；不能只写“修复若干问题”“更新代码”“发布 beta”这类不可追溯描述。
- Product Release 现在是“发布记录”，不是统一产品语义版本。客户端更新判断应看组件自身版本与 runtime manifest，不看 Product Release tag。
- Desktop 和 Watch 的发布版本不得写死在源码、Gradle、Wails 配置或 Skill 文档示例里；需要发布或打包时必须通过 workflow input、脚本参数或环境变量显式传入。
- 触发 `publish-beta.yml` 时，`full` / `desktop` 必须提供 `desktop_version`，`full` / `watch` 必须提供 `watch_version_name` 和 `watch_version_code`。
- 触发 `publish-runtime.yml` 时，必须提供 `desktop_min_version`、`watch_version_name` 和 `watch_version_code`，同时显式提供 platform-tools 与 cloudflared 版本或 URL。
- 组件级发布时，未变化组件必须明确标记为 `reused` 或 `not_included`，不能制造新的 APK、桌面包或 changelog 事实。
- Watch APK 必须按 `references/watch-apk-release-gate.md` 验证最终 artifact，不能把 debug APK、Gradle 输出目录或聊天传输文件当作公开产物。
- 除非用户明确要求，不要引入新的发布基础设施、别名层或复杂镜像编排。
- 每次使用结束前，必须做一次轻量复盘，并把高复用经验沉淀回 Skill、reference、文档或检查脚本。

## 意图分流

先把用户请求归到以下四类之一：

### 1. 公开 `beta` 发版

典型说法：

- “发个 beta 版”
- “发 GitHub Release”
- “更新正式下载”
- “把这版给外部用户用”

动作：

- 读取仓库状态、workflow、发布文档和最近一次发布记录
- 判断这是完整公开发版，还是只补某一段
- 使用 `beta` 固定契约执行

### 2. Runtime Release

典型说法：

- “发 runtime”
- “更新运行时依赖”
- “发布 platform-tools / cloudflared runtime”
- “让 Desktop 下载新的 runtime manifest”

动作：

- 使用 `publish-runtime.yml`
- 确认 runtime 版本、platform-tools、cloudflared 输入
- 发布后验证 Runtime Release、`runtime-manifest.json` 和 checksum

### 3. 发布机制实现或改造

典型说法：

- “把组件级 beta 发布做出来”
- “给手表更新加主备”
- “调整 Runtime Release 指针”
- “做个发版 Skill”
- “改 workflow / notes / metadata”

动作：

- 进入实现模式，不直接发布真实产物
- 先用已确认设计约束自己，再改代码或文档

### 4. 发布诊断或流程建议

典型说法：

- “这次该走 beta 还是 runtime”
- “为什么这版没进更新”
- “帮我看发版哪步坏了”
- “应该新建 workflow 还是 job”

动作：

- 先定位问题
- 不直接替用户执行发布，除非用户语气已经是在让你直接修复或补发

更细的例句和澄清规则见 `references/intent-matrix.md`。

## 公共入口流程

无论最终落在哪条路径，先做这些检查：

1. 查看仓库状态

```bash
git status --short
git branch --show-current
ai-task show
```

2. 读取当前发布相关上下文

```bash
sed -n '1,260p' .github/workflows/publish-beta.yml
sed -n '1,260p' .github/workflows/publish-runtime.yml
sed -n '1,220p' desktop-app/README.md
sed -n '1,220p' watch-app/RELEASE_BUILDS.md
```

3. 判断这次是：

- 执行既有发布流程
- 补发布机制实现
- 先诊断再决定动作

4. 如果用户要求“发当前代码”，但工作区状态和目标提交不清晰，只澄清这一个问题：

- 到底发哪个 commit

## `beta` 路径

处理公开 `beta` 时：

1. 判断只补哪一段，还是完整发布：

- GitHub Release
- watch 更新 metadata
- watch APK
- runtime / Desktop
- 全链路

2. 优先消费 GitHub Actions 产物。

- 如果目标 commit 没有对应 CI 产物，先触发或补跑流水线
- 不默认退回本地直打 release APK

3. 强制遵守 `beta` 契约：

- 客户端更新源：`openwatcher.ai`
- GitHub Release 是公开事实记录、历史归档和人工下载入口
- 公开仓 workflow 不调用 official Worker，不写 R2，不校验官网路径

4. 发布后至少验证：

- GitHub Release 附件和 notes 正常
- GitHub Release notes 按固定分类和组件名描述本次真实变更，并与 `changelog-entry.json` 语义一致
- 本次更新的 Desktop / Watch 版本来自 workflow input 或环境变量，不来自仓库源码中的固定版本号
- `release-manifest.json`、`checksums.txt`、`release-notes.md`、`changelog-entry.json` 同时存在于公开 release
- `release-manifest.json` 不包含客户端 `downloadUrl`、`fallbackDownloadUrl`、GitHub asset 基地址或 `runtime.manifestUrl`
- release manifest、changelog entry、release notes、真实产物之间的版本号、commit、sha256、大小和组件状态一致
- 需要官网/R2 发布时，切到 `ow-official` 仓按其 Skill 编排

如果你发现这次根本还缺发布机制本身，例如 workflow 还会调用 official Worker，或者仍在 manifest 中写 GitHub 客户端下载地址，不要伪装成发布成功，直接切回“发布机制实现路径”。

## Runtime 路径

处理 Runtime Release 时：

1. 使用 `.github/workflows/publish-runtime.yml`。
2. 发布前确认：

- `runtime_version` 符合 `vX.Y.Z` 或兼容后缀格式
- `desktop_min_version` 显式传入且符合组件版本规则
- `watch_version_name` 和 `watch_version_code` 显式传入，且 versionCode 是正整数
- `platform_tools_version` 与 `platform_tools_url` 二选一
- `cloudflared_version` 与 `cloudflared_url` 二选一
- 当前分支和目标 commit 明确

3. 发布后至少验证：

- GitHub Runtime Release 存在
- `runtime-manifest.json` 存在且可下载
- manifest 中的平台、URL、sha256 与真实资产一致
- 后续 `beta` Product Release 引用的是这个 Runtime Release

## 发布机制实现路径

当任务是在“做出机制”而不是“立刻发版”时：

- 先用仓库内已确认 spec 对齐边界
- 只做当前目标需要的最小实现
- 不要提前扩出复杂的别名层、动态切源控制面或额外基础设施

第一版实现优先级：

1. 组件级 Product Release 模型、channel manifest 当前态与 changelog 历史结构
2. GitHub Actions 与公开分发路径对接，支持 `release_scope` 和组件复用
3. 手表端更新通道与主备回退逻辑
4. Watch / Desktop / Runtime 组件状态复用规则
5. GitHub Release notes / metadata 固化

## 诊断路径

只做诊断时：

- 先确认是 workflow、metadata、更新入口、artifact、manifest，还是 GitHub Release 问题
- 只要诊断结果已经明确说明“只差补某一段流程”，就把动作并回 `beta` 或 Runtime 路径
- 除非用户已经明确是在让你直接补救，否则不要偷跑真实发布

## Watch APK release gate

出现以下任一情况时，读取 `references/watch-apk-release-gate.md`：

- 构建或重打手表 release APK
- 补发或公开 watch APK
- 改动 watch APK metadata / changelog / 版本号策略
- 任何可能影响 watch APK 命名、签名、校验和公开交付方式的工作

本仓库已把 watch APK gate 合并进本 Skill。出现以上任一情况时读取 `references/watch-apk-release-gate.md`，按其中要求验证最终 artifact。

## 自进化收尾动作

每次使用结束前，必须做一次轻量复盘：

1. 总结本次真实踩到的坑。
2. 判断坑属于：

- Skill 指令不清
- 发布契约缺失
- 文档过时
- workflow / 脚本缺少保护
- 验证项不够

3. 如果问题能通过更新 Skill 解决，就在同一轮顺手更新 Skill。
4. 如果问题应落在代码、workflow 或文档，就在对应位置修正，并在 Skill 里补新的检查或说明。
5. 只沉淀高复用经验，不记录流水账。

硬要求：

- 每次使用结束前，输出“本次新增了哪些发布经验”
- 同类坑第二次出现时，必须更新 Skill、reference 或相关检查脚本

复盘和经验沉淀写入 `references/release-lessons.md`。
