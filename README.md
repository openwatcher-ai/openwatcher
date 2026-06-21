# OpenWatcher

OpenWatcher 是一个开源的 Codex 使用状态辅助项目，当前公开核心包括 Go 后端、OpenWatcher Desktop 和 Watch app。

## 推荐安装方式

推荐使用支持 Skill 的 agent 安装 OpenWatcher Desktop。先让 agent 从 GitHub 加载或安装 OpenWatcher Desktop 安装 Skill，再使用该 Skill 自动完成下载、校验、安装和启动。

Skill 位置：

```text
https://github.com/openwatcher-ai/openwatcher/tree/main/.codex/skills/openwatcher-desktop-install
```

如果 agent 支持 GitHub repo/path 形式，也可以使用：

```text
openwatcher-ai/openwatcher/.codex/skills/openwatcher-desktop-install
```

示例提示词：

```text
请从 GitHub 加载或安装 OpenWatcher Desktop 安装 Skill：
https://github.com/openwatcher-ai/openwatcher/tree/main/.codex/skills/openwatcher-desktop-install

然后使用 $openwatcher-desktop-install 在这台电脑安装最新 OpenWatcher Desktop，完成校验、安装、必要的系统限制处理，并启动应用。
```

macOS 可使用更明确的提示：

```text
请加载 OpenWatcher Desktop 安装 Skill，并使用 $openwatcher-desktop-install 在 macOS 上安装最新 OpenWatcher Desktop，移除 OpenWatcher.app 的 quarantine 限制并打开应用。
```

Windows 可使用更明确的提示：

```text
请加载 OpenWatcher Desktop 安装 Skill，并使用 $openwatcher-desktop-install 在 Windows 上静默安装最新 OpenWatcher Desktop，创建桌面快捷方式并打开应用。
```

当前 macOS 技术预览包未签名公证，Skill 只会移除 OpenWatcher 应用自身的隔离属性，不会关闭系统级 Gatekeeper。Windows 安装默认使用当前用户权限，并创建当前用户桌面快捷方式。

## 仓库内容

- `cmd/`、`internal/`：本机后端、配对逻辑、状态接口与通用能力。
- `desktop-app/`：Desktop 主程序、安装向导、运行时资源管理和诊断入口。
- `watch-app/`：Wear OS / Android 手表端应用。
- `scripts/`：公开测试、打包、发布和扫描脚本。
- `.codex/skills/`：面向支持 Skill 的 agent 的安装、发布等仓库内置 Skill。
- `docs/openwatcher-publication-docs/`：公开用户文档与发布参考。
- `testsupport/`、`testdata/`：测试夹具与契约样本。

## 不包含的内容

公开仓不包含下面这些私有平台内容：

- 官网静态站点与部署脚本
- Cloudflare Worker 控制面
- `.agent-handoff/`、`docs/superpowers/`
- 构建缓存、运行日志、签名材料、Cloudflare 凭据、用户配置

## 发布模型

- Product Release tag 是发布记录号，不是统一产品版本。
- Desktop 和 Watch 的发布版本必须由构建输入传入，不写死在源码或配置文件里。
- 本机服务随 Desktop 打包交付，只保留构建信息，不作为独立产品版本展示。
- Runtime Release 提供运行时依赖的公开事实记录。
- Product Release 提供 `release-manifest.json`、`checksums.txt`、`release-notes.md`、`changelog-entry.json` 和 `THIRD_PARTY_NOTICES.md`。
- `openwatcher.ai` 是公开文档、更新检查和客户端下载入口，GitHub Release 是事实记录、历史归档和人工下载入口。

当前发布事实以 GitHub Releases、`https://openwatcher.ai/channels/beta.json` 和 `https://openwatcher.ai/changelog.json` 为准。正式发布初始化后，公开仓不保留旧 beta 或旧 changelog 记录作为当前版本说明。

## 快速开始

本地启动后端：

```bash
scripts/start-local.sh
```

运行基础测试：

```bash
go test ./...
cd desktop-app && go test ./...
cd ../watch-app && ./gradlew --no-daemon testDebugUnitTest
```

执行公开仓 preflight：

```bash
scripts/test-openwatcher-preflight.sh
```

## 公开文档

- [用户指南](docs/openwatcher-publication-docs/09-technical-preview-user-guide.md)
- [发布前 Checklist](docs/openwatcher-publication-docs/10-technical-preview-release-checklist.md)
- [发布文档索引](docs/openwatcher-publication-docs/00-docs-index.md)
- [Desktop 目录说明](desktop-app/README.md)

## 参与和安全

- [贡献说明](CONTRIBUTING.md)
- [安全策略](SECURITY.md)
- [隐私说明](PRIVACY.md)

## 许可

OpenWatcher 公开仓当前按 GNU Affero General Public License v3.0 发布，详见 [LICENSE](LICENSE)。
