# Contributing

感谢关注 OpenWatcher。当前仓库是公开技术预览版本，欢迎提交问题报告、兼容性样本、文档修正和小范围修复。

## 开始前

请先阅读：

- [README.md](README.md)
- [用户指南](docs/openwatcher-publication-docs/09-technical-preview-user-guide.md)
- [发布文档索引](docs/openwatcher-publication-docs/00-docs-index.md)
- [SECURITY.md](SECURITY.md)
- [PRIVACY.md](PRIVACY.md)

不要在公开 issue 或 PR 里提交 token、cookie、配置码、tunnel 凭据、签名材料、完整本机路径或完整隐私日志。

## 本地验证

基础验证：

```bash
go test ./...
cd desktop-app && go test ./...
cd ../watch-app && ./gradlew --no-daemon testDebugUnitTest
```

发布前公开仓检查：

```bash
scripts/test-openwatcher-preflight.sh
```

发布脚本局部验证：

```bash
scripts/test-release-product-scripts.sh
```

## 代码与文档约定

- 面向公开发布的内容只提交到本仓库。
- 私有官网、Worker 控制面、签名材料和运维脚本不属于本公开仓。
- 文档中的当前发布事实应以最新 GitHub Release、`https://openwatcher.ai/channels/beta.json` 和 `https://openwatcher.ai/changelog.json` 为准。
- macOS / Windows 签名、公证、托管隧道和真机兼容性状态不得写成已完成，除非有可复验记录。

## 提交 issue

请优先使用 GitHub issue templates：

- Bug report：功能错误、安装失败、更新失败。
- Compatibility report：电脑系统、手表型号、网络模式和 ADB 兼容性样本。

安全漏洞不要走公开 issue。请按 [SECURITY.md](SECURITY.md) 使用私密报告路径。

## Pull Request

PR 描述建议包含：

- 改了什么。
- 为什么需要改。
- 验证命令和结果。
- 是否涉及发布产物、下载入口、安全边界或兼容性声明。

如果 PR 修改 release workflow、打包脚本或公开文档，请同步检查 `docs/openwatcher-publication-docs/10-technical-preview-release-checklist.md` 和 `scripts/check-release-artifacts.sh`。
