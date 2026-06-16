# OpenWatcher 发版意图矩阵

本文件用于把用户说法快速映射到执行路径，减少每次临场判断。

## 1. 公开 `beta` 发版

命中例句：

- “发个 beta 版”
- “发 GitHub Release”
- “更新正式下载”
- “把这版给外部用户用”

默认执行：

- 进入 `beta` 路径
- 优先消费 GitHub Actions 产物
- 检查是否需要完整公开发版，还是只补其中一段

如果用户同时提到：

- “先把发布链路做出来”

则不要误发版，切到“发布机制实现路径”。

## 2. Runtime Release

命中例句：

- “发 runtime”
- “更新运行时依赖”
- “发布 platform-tools / cloudflared”
- “让 Desktop 使用新的 runtime manifest”

默认执行：

- 进入 Runtime 路径
- 使用 `publish-runtime.yml`
- 检查 Runtime Release 版本、依赖来源、manifest、checksums 和 GitHub Release assets

## 3. 发布机制实现或改造

命中例句：

- “把组件级 beta 发布做出来”
- “给手表更新加主备”
- “调整 Runtime Release 指针”
- “做个发版 Skill”
- “改 workflow / notes / metadata”

默认执行：

- 进入实现模式
- 先参考仓库内已确认设计
- 不直接发布真实产物

## 4. 发布诊断或建议

命中例句：

- “这次该走 beta 还是 runtime”
- “为什么这版没进更新”
- “帮我看发版哪步坏了”
- “应该新建 workflow 还是 job”

默认执行：

- 先读现状
- 只做诊断或建议
- 除非用户明确在让你直接补救，否则不偷跑发布

## 5. 直接执行规则

满足以下任一条件，直接执行：

- 用户明确说了渠道或发布对象：`beta`、Runtime、watch、Desktop
- 用户明确说了交付物：GitHub Release、watch 更新、Desktop 包、runtime
- 用户明确说了动作：发版、补发、重发、修更新入口、改 workflow

## 6. 必须澄清的情况

只有在这些情况下才停下来问：

- 同时提到 `beta` 和 Runtime，但没说要做哪一个
- 既像“做机制”，又像“立刻发版”
- 只说“发一下”，但没说是 Product Release、Runtime Release 还是组件级发布
- 目标产物不清楚

澄清时只问会改变结果的问题，不要泛泛追问。
