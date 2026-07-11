# OpenWatcher Desktop

OpenWatcher Desktop 是当前公开技术预览的桌面端主入口。它不再只是工程骨架，而是面向外部用户的安装向导和本机控制台。

## 当前主路径

当前代码路径已经覆盖以下能力：

- 检查本机 `~/.codex` 或 `CODEX_HOME` 是否可读。
- 定位并启动 OpenWatcher 本机服务组件。
- 请求 `http://127.0.0.1:8787/healthz` 或当前配置地址完成自检。
- 在首次启动或依赖缺失时，自动下载并缓存 ADB / platform-tools。
- 在首次启动或依赖缺失时，自动下载并缓存手表 APK，执行无线 ADB 配对、连接、安装、启动和 bootstrap。
- 配置局域网模式或自定义公网 URL。
- 在桌面显示 5h / 7d 额度环，并通过四象限面板查看 7×24 小时、今日 24 小时和最近 30 天用量；数据由本机服务只读接口提供，不包含会话详情。
- 输入托管隧道配置码，调用已上线 Worker redeem API，安全保存绑定信息，并在需要时自动下载 `cloudflared` 后用独立运行配置拉起本地隧道。
- 生成脱敏日志和诊断摘要，便于外部用户反馈问题。

当前不应对外宣称的能力：

- 托管隧道仍依赖管理员发码，不提供公开自助申请或公开控制台。
- 不应把公网模式和所有手表机型兼容性写成已完整覆盖。

## 给外部用户的文档

- [Desktop Technical Preview 用户指南](../docs/openwatcher-publication-docs/09-technical-preview-user-guide.md)
- [Technical Preview 发布前 Checklist](../docs/openwatcher-publication-docs/10-technical-preview-release-checklist.md)

如果你只是想安装和体验 OpenWatcher，请优先阅读上面的用户文档，而不是直接从源码目录推断流程。

## 仓库与 release 的区别

- GitHub Release 的 Desktop 包附带主程序、sidecar、更新辅助程序和桌面悬浮球辅助程序。
- ADB / platform-tools、`cloudflared` 和手表 APK 通过 `https://openwatcher.ai/channels/beta.json` 指向的版本化 Runtime Release 分发，并在 Desktop 首启或依赖缺失时下载到本机缓存目录。
- `bundled/` 目录保留 sidecar 和 channel manifest 地址的查找约定，也兼容开发环境手动放置资源。

资源查找约定：

```text
bundled/
  openwatcher/<platform>/
  runtime/channel-manifest-url.txt
  widget/<platform>/openwatcher-widget.exe  # Windows
```

macOS 将悬浮球作为 `OpenWatcher.app/Contents/Library/Helpers/OpenWatcher Widget.app` 嵌入主应用，并使用 `LSUIElement` 隐藏独立 Dock 入口。

## 本地开发

当前目录提供 Wails 入口和前端代码。桌面端当前打包入口为 `frontend-vue3/`；旧 `frontend/` 目录暂时保留为迁移参考，不参与新产物。若本机已安装 Wails CLI，可在此目录启动开发模式：

```bash
wails dev
```

基础测试：

```bash
go test ./...
```

悬浮球前端位于 `widget/frontend-vue3/`；正式 Desktop 构建会先构建该前端，再把对应平台的 helper 放入主安装包。

若要在源码仓库里验证 sidecar 查找逻辑，可在仓库根目录准备以下任一位置：

- `bin/openwatcher`
- `build/bin/openwatcher`
- `desktop-app/bundled/openwatcher/<platform>/openwatcher`
