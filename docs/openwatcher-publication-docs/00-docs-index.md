# OpenWatcher 公开发布文档索引

本文档集现在分成两层：

- 面向外部用户的安装与体验文档；
- 面向维护者和贡献者的设计与发布参考。

如果你是第一次接触 OpenWatcher，请先看“外部用户文档”。如果你是在维护发布流程、补功能或核对设计，再继续看后面的参考文档。

## 外部用户文档

1. `09-technical-preview-user-guide.md`
   当前公开技术预览的用户指南，覆盖下载与启动、首次安装向导、局域网模式、自定义公网 URL、故障排查、隐私与安全、已知限制、兼容性说明。

2. `10-technical-preview-release-checklist.md`
   technical preview 发布前 checklist，明确哪些文档与代码路径已经核对完成，哪些 Windows 和手表主路径已完成真实验收，以及哪些 macOS、公网模式和兼容性样本仍待补充。

## 维护者参考文档

1. `01-next-phase-roadmap.md`  
   后续阶段目标、发布阻断项和初始技术预览最小可发布范围。

2. `02-openwatcher-desktop-prd.md`  
   OpenWatcher Desktop 的产品定位、用户流程、页面结构和范围定义。

3. `03-desktop-technical-architecture.md`  
   桌面端技术架构、sidecar、资源打包、进程管理、日志与配置。

4. `04-watch-bootstrap-protocol.md`  
   桌面端通过 ADB 初始化手表 App 的协议，包括 `baseUrl`、`deviceToken` 和安全确认。

5. `05-adb-wireless-installer-wizard.md`  
   无线 ADB 安装向导状态机、输入项、命令执行和多设备处理。

6. `06-network-modes-and-managed-tunnel.md`  
   局域网、自定义公网 URL，以及已被真实 Worker 控制面替代的托管隧道历史设计。

7. `07-release-packaging-and-publication.md`  
   GitHub Release 交付物、打包要求、checksums、第三方 notices 和最终发布验收项。

8. `14-component-release-production-runbook.md`
   组件级发布模型的生产执行手册，记录当前线上仍是旧 schema 的证据、真实发布顺序、脚本入口和验收命令。

## 当前公开状态说明

- 项目正式名称：OpenWatcher。
- 官网域名：`openwatcher.ai`。
- 对外主入口：OpenWatcher Desktop。
- 当前公开技术预览不把 Google Play 或厂商手表商店作为首要安装渠道。
- 仓库内保留 Desktop 侧托管隧道兑换与本地 `cloudflared` 运行逻辑；控制面部署细节不包含在公开仓。
- Windows x64 release 安装器主路径已完成真实验收。
- 手表真机安装、启动和运行时配置写入已完成真实验收；公网模式与更多机型兼容性仍需继续补充样本。

## 阅读建议

- 外部用户：先读 `09`，再看 `10` 里的已知未完成项。
- 维护者：按 `01 -> 07` 的顺序阅读；需要补发布链路时优先看 `07`。
