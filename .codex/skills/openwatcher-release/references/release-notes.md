# OpenWatcher Release Notes 规则

本文件约束公开 GitHub Product Release 的发布说明。目标是让用户和维护者一眼看出：本次改了什么类型的问题、影响哪个组件、哪些产物真的更新、哪些只是复用上一版，以及这份 Markdown 如何与结构化 `changelog-entry.json` 保持一致。

## 基本原则

- 发布说明默认使用中文。
- 不要求中英双语。
- 每条变更必须同时包含“变更类型”和“组件”。
- 不写空泛条目，例如“更新代码”“修复若干问题”“优化发布流程”。
- 不泄露 token、cookie、通知 topic、私有域名、私有路径、完整日志或未脱敏诊断数据。
- 不把 debug APK、本地临时构建、聊天文件、临时路径或 Gradle 原始输出写成正式 release artifact。
- Product Release tag 现在是发布记录号，不是统一产品版本号。
- release notes 和 `changelog-entry.json` 必须来自同一组事实：发布范围、组件状态、分类条目、产物引用不能互相矛盾。
- 官网、Pages、Worker、私有控制面只属于私有仓库运维增强，不作为公开仓 Product Release notes 的固定组件。若私有同步影响公开下载或公开 channel manifest，只按公开影响归入【兼容性】或相关公开组件。

## 固定组件

公开发布说明的组件名固定使用中文方括号：

- 【桌面应用】
- 【手表应用】
- 【运行时依赖】
- 【兼容性】
- 【文档】

如果 agent 判断某项变更不明显属于以上组件，可以根据语义自行归类，但必须优先使用这些固定组件之一。

归类建议：

- 桌面安装包、Desktop UI、Desktop 首启、sidecar 启动、macOS/Windows 打包、bundle id、图标、下载入口客户端行为：归入【桌面应用】。
- Watch APK、Wear OS UI、手表更新检查、手表安装、手表诊断上传、手表端配置：归入【手表应用】。
- platform-tools、cloudflared、Watch APK runtime 缓存、runtime manifest、Desktop 首启下载的依赖资源：归入【运行时依赖】。
- channel manifest schema、发布范围复用、升级注意事项、配置目录变化、签名/公证限制、版本兼容、下载 URL 语义：归入【兼容性】。
- README、官网公开文档、用户指南、检查清单、发布说明规范、开发文档：归入【文档】。

## 固定分类

Product Release notes 使用以下分类。没有内容的分类可以省略。

### 新增功能

新增用户可见能力、新的安装/更新能力、新的公开分发能力。

条目格式：

```md
- 【桌面应用】新增首次启动时的运行时依赖下载状态展示。
```

### 功能优化

已有行为、体验、性能、发布流程或校验能力的改善。

条目格式：

```md
- 【运行时依赖】优化 platform-tools 和 cloudflared 的缓存校验，避免重复下载已匹配 sha256 的资源。
```

### 问题修复

修复错误、缺失产物、错误 manifest、错误跳转、安装失败、图标或名称异常等。

条目格式：

```md
- 【桌面应用】修复 macOS DMG 内部应用名称带平台后缀、图标回退为默认图标的问题。
```

### 兼容性与升级说明

用户升级前需要知道的变化，或组件级发布范围导致的复用关系。

条目格式：

```md
- 【兼容性】本次仅更新桌面应用，手表应用继续复用上一版 Watch APK，不会触发手表端版本更新。
```

### 产物与校验

只写发布证据，不写产品卖点。必须覆盖本次 Product Release 和其引用的 Runtime Release。

条目格式：

```md
- Product Release: `beta-2026.06.11.2`
- Runtime Release: `runtime-vX.Y.Z`
- Release manifest: `release-manifest.json`
- Changelog entry: `changelog-entry.json`
- Checksums: `checksums.txt`
```

## 发布范围

每次 Product Release notes 顶部必须写发布范围，明确每个组件是“更新”还是“复用”：

```md
## 发布范围

- 【桌面应用】更新
- 【手表应用】复用上一版
- 【运行时依赖】复用当前 Runtime Release
- 【兼容性】更新
- 【文档】未包含
```

允许的状态只有：

- `更新`
- `复用上一版`
- `复用当前 Runtime Release`
- `未包含`

如果发布流程支持组件级发布，agent 必须先读取 workflow 输入、上一版 `release-manifest.json`、当前 `release-manifest.json`、`changelog-entry.json` 和本次产物清单，再判断每个组件状态。

## 结构化 changelog entry 对齐

每次 Product Release 都必须生成 `changelog-entry.json`。它不是 release notes 的附件装饰，而是结构化事实源。

最少要覆盖：

- `id`
- `publishedAt`
- `scope`
- `components`
- `notes.features`
- `notes.improvements`
- `notes.fixes`
- `notes.compatibility`
- `links.releaseUrl`
- `links.releaseManifestUrl`

对齐规则：

- `## 发布范围` 要能一一映射到 `components.*.status`
- `新增功能` 对应 `notes.features`
- `功能优化` 对应 `notes.improvements`
- `问题修复` 对应 `notes.fixes`
- `兼容性与升级说明` 对应 `notes.compatibility`
- `产物与校验` 中的 release tag、runtime tag、release manifest 文件名要和结构化字段一致

## 完整模板

```md
# OpenWatcher Beta beta-YYYY.MM.DD.N

本次发布聚焦一句话概述。

## 发布范围

- 【桌面应用】更新
- 【手表应用】复用上一版
- 【运行时依赖】复用当前 Runtime Release
- 【兼容性】更新
- 【文档】未包含

## 新增功能

- 【桌面应用】...

## 功能优化

- 【运行时依赖】...

## 问题修复

- 【手表应用】...

## 兼容性与升级说明

- 【兼容性】...

## 产物与校验

- Product Release: `beta-YYYY.MM.DD.N`
- Runtime Release: `runtime-vX.Y.Z`
- Release manifest: `release-manifest.json`
- Changelog entry: `changelog-entry.json`
- Checksums: `checksums.txt`
```

## Agent 生成要求

- 先确定发布范围，再写分类条目。
- 只写本次真实变化，不把未变化组件写成更新。
- 对复用组件必须明确写“复用上一版”或“复用当前 Runtime Release”。
- 每条变更尽量使用用户可理解的结果描述，而不是只写内部实现。
- 有 PR、issue 或提交范围时，可以在条目末尾补充引用；没有时不强行编造。
- 如果 release notes 与 `release-manifest.json`、`changelog-entry.json` 或产物清单矛盾，应先修正 notes 或发布流程，不能发布矛盾说明。
