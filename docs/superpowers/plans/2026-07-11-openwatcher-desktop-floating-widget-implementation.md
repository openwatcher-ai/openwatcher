# OpenWatcher Desktop 悬浮球实施计划

- 日期：2026-07-11
- 依据：已批准并提交的悬浮球设计规格与四象限视觉稿
- 分支：codex/desktop-floating-widget
- 工作区：独立 Git worktree
- 交付模式：真实 API、真实 helper 生命周期、macOS 可运行验证、Windows 编译与打包链路验证

## 1. 实施边界

本次实现包含 sidecar 状态契约、独立 Widget 鉴权、Desktop 设置与 helper 管理、第二个 Wails v2 + Vue 3 应用、四象限 UI、macOS/Windows 平台适配和打包脚本。不会在 Widget 中加入会话详情，不改变手表默认请求行为，不引入新的前端框架。

前端以 desktop-app/widget/ui-spec/ui-spec.json 为结构化验收依据。生成图只约束视觉方向；正方形热力格、真实数据维度、无重复指标等书面规则优先。

## 2. 分阶段实现

### 阶段 A：状态契约与只读监听

目标：让 Widget 通过与手表相同的状态结构取数，同时确保响应不含会话数组。

主要文件：

- internal/server/server.go
- internal/server/server_test.go
- internal/server/contract_fixtures_test.go
- internal/config/config.go
- internal/config/config_test.go
- cmd/openwatcher/main.go
- cmd/openwatcher/main_test.go

改动：

1. statusResponseOptions 增加 IncludeSessions。
2. 公共 GET 和 SSE 将 includeSessions 缺省解释为 true。
3. includeSessions=0 时完整快照彻底省略 sessions，SSE 不发送 status_sessions。
4. SSE 首帧与后续轮询使用相同的会话选项。
5. 不返回会话时跳过会话级 compaction 日志副作用。
6. Config 增加独立 WidgetTokenHash，不复用 beta/dev pairing slot。
7. App 增加只开放 GET /api/status 与 GET /api/status/stream 的 WidgetHandler。
8. WidgetHandler 强制 loopback、独立 token 和 IncludeSessions=false；主服务 no-auth 不影响它。
9. sidecar 增加 widget-listen 参数，用独立 http.Server 启动 loopback 监听，并输出可解析但不敏感的 endpoint 状态行。

验证：

- 默认 GET/SSE 契约保持 sessions。
- includeSessions=0 时 JSON 无 sessions，SSE 无 status_sessions。
- WidgetHandler 拒绝非 loopback、watch token、空 token、错误 token、写方法和其他路径。
- no-auth 主服务下 WidgetHandler 仍拒绝未授权请求。
- go test ./internal/server ./internal/config ./cmd/openwatcher

### 阶段 B：系统凭据、sidecar endpoint 与 helper 生命周期

目标：Desktop 持有 Widget 开关与进程，原始 token 只进系统凭据存储。

主要文件：

- desktop-app/internal/widgetauth/*
- desktop-app/internal/backend/manager.go
- desktop-app/internal/backend/manager_test.go
- desktop-app/internal/widget/*
- desktop-app/internal/settings/desktop_settings.go
- desktop-app/internal/settings/store_test.go
- desktop-app/app.go
- desktop-app/app_test.go
- desktop-app/app_actions.go
- desktop-app/app_actions_test.go

改动：

1. 定义 SecretStore 接口、随机 token 生成和 hash 同步。
2. macOS 使用 Keychain API，Windows 使用 Credential Manager API；其他平台返回明确不支持错误，测试使用内存 store。
3. token 不进入命令行、环境变量、日志、诊断或 Vue。
4. Backend StartConfig 增加 WidgetListen，Desktop 主 sidecar 使用 127.0.0.1:0。
5. Backend Manager 解析 sidecar 的结构化 endpoint 行，并通过状态或等待方法提供给 Desktop。
6. helper Manager 负责定位、启动、停止、单实例、三次限频重启和 endpoint 变化后的重启。
7. helper 命令行只接收非敏感 endpoint；Desktop 从系统存储读取凭据，并通过匿名标准输入管道一次性传给 helper，避免依赖跨应用 Keychain 权限组。
8. DesktopSettings 增加 FloatingWidgetEnabled，并通过原始 JSON 字段存在性实现：新文件默认 true，旧文件缺字段迁移 false。
9. startup 在 sidecar 前准备凭据和 hash；启用时启动 helper。shutdown 先停 helper 再停 sidecar。
10. 手动启动、停止和重启 sidecar 后刷新 helper endpoint。

验证：

- 设置新装、升级和显式 false/true 四类测试。
- token 生成、hash 对齐、日志脱敏和参数检查。
- helper 启停、崩溃重试、shutdown 顺序和 endpoint 变化测试。
- go test ./desktop-app/... ./internal/widgetauth/...

### 阶段 C：独立 Wails helper 与状态客户端

目标：建立可独立构建的 openwatcher-widget，并由 Go 掌握网络和窗口状态。

主要文件：

- desktop-app/widget/wails.json
- desktop-app/widget/main.go
- desktop-app/widget/app.go
- desktop-app/widget/internal/widgetapi/*
- desktop-app/widget/internal/widgetvm/*
- desktop-app/widget/internal/widgetwindow/*
- desktop-app/widget/internal/widgetprefs/*
- desktop-app/widget/platform_*

改动：

1. 创建第二个 Wails v2 应用，输出名 openwatcher-widget。
2. 使用透明、无边框、置顶、禁止手动缩放、单实例窗口。
3. Go HTTP 客户端先请求完整快照，再维护 SSE；始终发送 includeDailyTrend30d=1 和 includeSessions=0。
4. SSE 实现 status_snapshot、status_quota、status_heatmap24h、status_errors 与 heartbeat。
5. 30 秒进入重连，60 秒进入 stale；完整刷新覆盖启动、手动刷新、重连与 API 时区跨日。
6. ViewModel 只含额度、热力图、今日统计、30 日统计、连接状态和时间。
7. 暴露给 Vue 的动作限定为刷新、展开、收起、吸附和打开主应用。
8. 位置偏好只保存显示器、边缘和归一化位置，不保存业务数据。
9. macOS 使用 NSScreen visibleFrame 与 accessory activation policy；Windows 使用 MonitorInfo、tool window 样式和 DPI 感知。
10. 收起窗口为 56×56；展开面板约 1120×580，并限制在当前工作区。

验证：

- Go DTO/SSE 夹具测试。
- 30s/60s 状态机、跨日刷新和部分数据缺失测试。
- 位置吸附、显示器移除回退和尺寸计算测试。
- macOS 实机运行；Windows 平台文件至少交叉编译检查。

### 阶段 D：Vue 四象限界面

目标：以现有 Vue 3/Vite 风格实现可交互、可视觉验收的真实界面。

主要文件：

- desktop-app/widget/frontend-vue3/package.json
- desktop-app/widget/frontend-vue3/src/App.vue
- desktop-app/widget/frontend-vue3/src/services/wails.js
- desktop-app/widget/frontend-vue3/src/state/useWidgetStore.js
- desktop-app/widget/frontend-vue3/src/components/*
- desktop-app/widget/frontend-vue3/src/styles/*

改动：

1. FloatingOrb 复用 QuotaRing，5h/7d 各 4 秒，350ms 淡入，hover 暂停，reduced-motion 取消淡入。
2. 左上同时展示 5h/7d 剩余额度、重置时间、计划与 freshness。
3. 右上渲染严格 24 根小时柱和三段今日组成条，价值直接使用接口 label。
4. 左下渲染 7×24、共 168 个正方形格；最新日期在上。
5. 右下渲染 5×7、共 35 个正方形槽；30 个日期按 weekday occurrence 映射，5 个占位。
6. hover 显示临时 tooltip，点击固定；更新按日期或 hourStart 保持选择。
7. 加载、reconnecting、stale、offline、invalid credential 和局部缺失均按规格显示。
8. 使用 data-ui-id 标注 UI 规格中的关键节点。
9. Vite fallback 使用视觉稿示例数据，仅供开发预览和视觉 QA，不进入 Wails 生产数据路径。

验证：

- node:test 覆盖 store、30 日映射、格子数量、组成计算和状态转换。
- npm run test
- npm run build
- 浏览器检查 1672×941、1120×580 与 920×500。

### 阶段 E：设置 UI 与打包

目标：用户能控制悬浮球，正式产物包含 helper 且不出现额外应用入口。

主要文件：

- desktop-app/frontend-vue3/src/pages/SettingsPage.vue
- desktop-app/frontend-vue3/src/state/defaults.js
- desktop-app/frontend-vue3/src/state/useAppStore.js
- desktop-app/frontend-vue3/src/services/wails.js
- desktop-app/cmd/syncbundle/main.go
- desktop-app/cmd/syncbundle/main_test.go
- scripts/package-desktop-release.sh
- scripts/package-desktop-windows-release.sh
- scripts/check-release-artifacts.sh

改动：

1. 常规设置增加“桌面悬浮球”开关和状态反馈。
2. 主应用 Wails 绑定增加读取/写入 Widget 设置。
3. release 构建先构建 Widget，再构建主 Desktop。
4. macOS 将 Widget 作为嵌套 helper app 放入主包，Info.plist 使用 LSUIElement，并纳入 codesign。
5. Windows 将 openwatcher-widget.exe 放入 bundled，启动时隐藏控制台并使用 tool window。
6. syncbundle、发布脚本和 artifact 检查增加 helper 存在性与版本一致性验证。

验证：

- 主前端 npm run build 与 store 测试。
- syncbundle 单元测试。
- macOS app bundle 检查、codesign 验证和运行检查。
- Windows 构建脚本静态检查与 CI 可执行路径。

## 3. 综合验证

1. gofmt 并运行 go test ./...
2. 构建主 Desktop 与 Widget 两套 Vue 前端。
3. 构建 macOS Widget 和主 Desktop。
4. 启动 sidecar，使用专用凭据验证真实 GET 与 SSE。
5. 证明 Widget 请求和 ViewModel 中没有 sessions、会话标题或消息。
6. 使用内置 Browser 验证开发预览的交互、格子数量和响应式。
7. 运行 image2code visual-qa-runner，清除全部 high/medium 失败。
8. 对已批准概念和最新实现截图分别运行 view_image，记录至少五项 fidelity 对比。
9. 检查 git diff、敏感残留、打包内容和工作区状态。

## 4. 协作与合并

- sidecar 契约、Widget helper、Desktop 集成分别在独立 worktree 中实现。
- 每个子任务在自己的分支提交并记录测试。
- 主实施分支使用 git merge --no-ff 合并，结合测试和真实运行结果验收。
- 最终把已验收的实施分支使用 git merge --no-ff 合入 main。
- 不使用 cherry-pick 或手工复制子任务 diff。
