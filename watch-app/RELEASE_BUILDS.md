# 手表 APK 构建记录

公开仓从初始化发布开始记录手表 release APK 构建历史。旧私有构建记录不再带入。

| 构建时间 UTC | versionName | versionCode | Git commit | 构建分支 | APK 文件 | SHA256 | 说明类型 | 变更摘要 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-06-12T11:21:41Z | 0.2.1 | 10002 | 9ed1463797167b5b0c16a9e0a5e8b85e639c54df | main | openwatcher-watchapp-v0.2.1-beta-2026.06.12.3-9ed1463-20260612-112141.apk | 71823a2fcbc2b0c5db4b1933d142bf531aa700c5ee6eee35fbf8bb058482f95f | 问题修复 | 修复手表远程初始化启动闪退，完善无配置和离网提示，并更新临时配置码页面的桌面端配置入口说明。 |
| 2026-06-12T19:26:56Z | 0.3.0 | 10003 | e774918 | detached | dist/openwatcher-watchapp-v0.3.0-init-flow-e774918-20260612-192656.apk | 730389b1d1a5fdc3ea7bc7f6b4e832b8155ee827bcea97719d8185dbca8c9f15 | user | 优化手表端初始化配置体验：服务不可达时显示服务地址和处理建议，配置链接可直接写入并覆盖旧配置，减少手表端确认步骤。 |
