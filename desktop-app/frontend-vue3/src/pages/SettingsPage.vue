<script setup>
import { computed } from "vue"
import { useAppStore } from "../state/useAppStore.js"
import AppButton from "../components/ui/AppButton.vue"
import FieldRow from "../components/ui/FieldRow.vue"
import InfoTable from "../components/ui/InfoTable.vue"
import PageTabs from "../components/ui/PageTabs.vue"
import ToneChip from "../components/ui/ToneChip.vue"

const store = useAppStore()

const developerSummary = computed(() => ({
  baseURL: store.selectors.developerHealthzUrl().replace(/\/healthz$/, ""),
  healthz: store.selectors.developerHealthzUrl(),
  bootstrap: store.state.generatedBootstrap?.bootstrapUri ? "已生成" : "未生成"
}))

const accessOptions = [
  { id: "emulator", label: "模拟器" },
  { id: "lan", label: "局域网" },
  { id: "tunnel", label: "开发隧道" },
  { id: "custom", label: "自定义地址" }
]

const generalToggleRows = [
  { key: "launchAtLogin", label: "开机启动 OpenWatcher" },
  { key: "autoStartBackend", label: "启动后自动启动本机服务" },
  { key: "minimizeToTray", label: "关闭窗口后最小化到托盘" }
]

const privacyToggleRows = [
  { key: "redactDiagnostics", label: "诊断包默认脱敏" },
  { key: "anonymousCompatibilityReports", label: "允许匿名兼容性上报" },
  { key: "allowUpdateChecks", label: "允许检查更新" },
  { key: "cleanupPairingLogs", label: "自动清理配对码日志" }
]

const advancedRows = [
  { key: "useSystemAdb", label: "使用系统 PATH 中的 adb", type: "toggle" },
  { key: "adbPort", label: "ADB 服务端端口", type: "text", value: "5037" },
  { key: "developerLogs", label: "启用开发者日志", type: "toggle" },
  { key: "showRawCommandOutput", label: "显示原始命令输出", type: "toggle" },
  { key: "experimentalManagedTunnel", label: "实验性托管隧道", type: "toggle" }
]

const hasUpdateResult = computed(() => Boolean(store.state.updateCheckResult && !store.state.updateCheckResult.error))

const updateRows = computed(() => {
  const result = store.state.updateCheckResult
  const showStatus = hasUpdateResult.value
  return [
    {
      label: "Desktop 当前版本",
      current: `${store.state.snapshot.productVersion || "dev"} (Technical Preview)`,
      status: !showStatus
        ? ""
        : (result.desktopUpdateAvailable && result.latestDesktopVersion
            ? `新版本 ${result.latestDesktopVersion}`
            : "已是最新版本"),
      tone: result?.desktopUpdateAvailable ? "available" : "current"
    },
    {
      label: "手表已安装版本",
      current: store.selectors.currentWatchInstalledVersionLabel(),
      status: !showStatus
        ? ""
        : (result.watchUpdateAvailable && result.latestWatchVersion
            ? `新版本 ${result.latestWatchVersion}`
            : "已是最新版本"),
      tone: result?.watchUpdateAvailable ? "available" : "current"
    }
  ]
})

const updateMetaLine = computed(() => {
  if (!hasUpdateResult.value) {
    return ""
  }
  const result = store.state.updateCheckResult
  const parts = [
    result.channel || "beta",
    formatCheckedAt(result.checkedAt),
    result.releaseSummary || ""
  ].filter(Boolean)
  return parts.join(" · ")
})

const desktopUpdateNotice = computed(() => {
  const status = store.state.desktopUpdateStatus
  if (!status?.message) {
    return ""
  }
  const version = status.version ? ` ${status.version}` : ""
  return `${status.message}${version}`
})

const desktopUpdateProgressStyle = computed(() => {
  const percent = Math.max(0, Math.min(100, Number(store.state.desktopUpdateProgress?.percent || 0)))
  return { width: `${percent}%` }
})

const releaseNotes = computed(() => Array.isArray(store.state.updateCheckResult?.releaseNotes) ? store.state.updateCheckResult.releaseNotes : [])

const codexHookTone = computed(() => store.state.codexHookState?.installed ? "ok" : "amber")

const codexHookStatusLabel = computed(() => store.state.codexHookState?.installed ? "已写入" : "未安装")

const codexHookRows = computed(() => {
  const status = store.state.codexHookState || {}
  return [
    ["hooks.json", status.hooksPathLabel || status.hooksPath || "~/.codex/hooks.json"],
    ["备份文件", status.backupPath || "本次未生成备份"]
  ]
})

const floatingWidgetRows = computed(() => [
  ["显示内容", "5h / 7d 额度环与四象限用量面板"],
  ["数据来源", "OpenWatcher 本机接口（不读取会话详情）"],
  ["状态更新", store.state.floatingWidget.checkedAt ? formatCheckedAt(store.state.floatingWidget.checkedAt) : "尚未刷新"],
  ["异常恢复", store.state.floatingWidget.restartAttempts > 0
    ? `近 5 分钟已自动重试 ${store.state.floatingWidget.restartAttempts} 次`
    : "辅助程序异常退出后自动重试"]
])

function formatCheckedAt(raw) {
  if (!raw) {
    return ""
  }
  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) {
    return raw
  }
  return parsed.toLocaleString("zh-CN", { hour12: false })
}
</script>

<template>
  <section class="page-stack" :class="{ 'developer-page-stack': store.state.settingsTab === 'developer' }">
    <PageTabs v-model="store.state.settingsTab" :tabs="store.constants.settingsTabs" />

    <section v-if="store.state.settingsTab === 'developer'" class="developer-page-layout">
      <div class="developer-main-column">
        <section class="section-head developer-page-head">
          <div>
            <h2>开发环境</h2>
            <p>在 Desktop 中启动和管理当前仓库的开发环境，并发送到手表进行调试。</p>
          </div>
          <div class="actions right">
            <AppButton tone="danger" icon="Trash2" :disabled="store.selectors.pairingActionBusy('dev')" @click="store.actions.clearBackendPairingAction('dev')">
              {{ store.selectors.pairingActionLabel("dev", "重置开发配对", "正在重置...") }}
            </AppButton>
            <AppButton icon="RefreshCw" :loading="store.state.globalHealthRunning" :disabled="store.state.developerAction.busy || store.state.globalHealthRunning" @click="store.actions.runGlobalHealthCheck({ manual: true })">重新检测</AppButton>
            <AppButton icon="FileText" @click="store.state.developerConfirmModalOpen = true">使用说明</AppButton>
          </div>
        </section>

        <article class="panel compact-panel">
          <header class="panel-head">
            <div>
              <h2>开发环境配置</h2>
              <p>配置仓库目录、访问方式和服务地址。</p>
            </div>
          </header>
          <div class="settings-form">
            <FieldRow label="仓库目录">
              <div class="inline-field">
                <input v-model="store.state.developerForm.repoPath" placeholder="/path/to/repo" @change="store.actions.persistDeveloperFormState" />
                <AppButton icon="FolderOpen" @click="store.actions.selectDeveloperRepoDirAction">选择文件夹</AppButton>
              </div>
            </FieldRow>
            <FieldRow label="启动脚本">
              <select disabled>
                <option>{{ store.selectors.developerStartCommand() || "scripts/start-local.sh" }}</option>
              </select>
              <small>实际执行：{{ store.selectors.developerStartCommand() || "scripts/start-local.sh" }}</small>
            </FieldRow>
            <div class="field-row">
              <span>访问方式</span>
              <div class="radio-grid">
                <button
                  v-for="option in accessOptions"
                  :key="option.id"
                  class="radio-line"
                  :class="{ 'is-selected': store.state.developerForm.accessMode === option.id }"
                  type="button"
                  @click="store.actions.setDeveloperAccessMode(option.id)"
                >
                  <span class="radio-dot"></span>
                  <strong>{{ option.label }}</strong>
                </button>
              </div>
            </div>
            <FieldRow label="服务地址">
              <div class="inline-field">
                <input
                  v-model="store.state.developerForm.devBaseUrl"
                  :readonly="store.state.developerForm.accessMode !== 'custom'"
                  placeholder="http://10.0.2.2:18787"
                  @change="store.actions.persistDeveloperFormState"
                />
                <AppButton icon="Copy" :copied="store.selectors.copyFeedbackActive('dev-service-base-url')" @click="store.actions.copyText(developerSummary.baseURL, 'dev-service-base-url')">复制</AppButton>
              </div>
            </FieldRow>
            <FieldRow label="Healthz 地址">
              <div class="inline-field">
                <input :value="store.selectors.developerHealthzUrl()" readonly />
                <AppButton icon="Copy" :copied="store.selectors.copyFeedbackActive('dev-healthz-url')" @click="store.actions.copyText(store.selectors.developerHealthzUrl(), 'dev-healthz-url')">复制</AppButton>
              </div>
            </FieldRow>
            <FieldRow label="环境变量">
              <div class="inline-field">
                <input :value="store.selectors.developerEnvFileLabel()" readonly />
                <AppButton icon="FolderOpen" :disabled="!store.selectors.developerStatus()?.envFilePresent" @click="store.actions.openFolderAction('OpenDeveloperEnvFile', '已打开 .env.development。', store.state.developerForm.repoPath.trim())">查看</AppButton>
              </div>
            </FieldRow>
          </div>
        </article>

        <article class="panel compact-panel">
          <header class="panel-head">
            <div>
              <h2>启动日志</h2>
              <p>{{ store.selectors.developerLogFileLabel() ? `日志文件：${store.selectors.developerLogFileLabel()}` : "日志会写入 Desktop 配置目录下的 logs 文件夹。" }}</p>
            </div>
            <div class="actions right">
              <AppButton icon="Trash2" @click="store.actions.clearDeveloperLogsAction">清空日志</AppButton>
              <AppButton icon="FolderOpen" :disabled="!store.selectors.developerLogFileLabel()" @click="store.actions.openFolderAction('OpenDeveloperLogFile', '已打开开发环境日志文件。')">打开日志文件</AppButton>
            </div>
          </header>
          <div class="developer-console">
            <div v-if="store.selectors.developerLogEntries(24).length === 0" class="developer-log-entry is-empty">
              <span>--:--:--</span>
              <ToneChip>INFO</ToneChip>
              <em>暂无开发环境日志</em>
            </div>
            <template v-else>
              <div v-for="line in store.selectors.developerLogEntries(24)" :key="line.at + line.message" class="developer-log-entry">
                <span>{{ store.selectors.formatDeveloperLogTime(line.at) }}</span>
                <ToneChip :tone="store.selectors.developerLogLevel(line.message) === 'error' ? 'warn' : (store.selectors.developerLogLevel(line.message) === 'warn' ? 'amber' : 'blue')">
                  {{ store.selectors.developerLogLevelLabel(line.message) }}
                </ToneChip>
                <em>{{ line.message }}</em>
              </div>
            </template>
          </div>
        </article>
      </div>

      <aside class="developer-side-column">
        <article class="panel compact-panel side-panel">
          <header class="panel-head">
            <div>
              <h2>环境控制</h2>
              <p>当前仓库</p>
            </div>
          </header>
          <strong class="repo-label">{{ store.selectors.developerCurrentRepoLabel() }}</strong>
          <div class="runtime-pill" :class="`tone-${store.selectors.developerStateTone()}`">
            <div>
              <strong>{{ store.selectors.developerStateLabel() }}</strong>
              <span>{{ store.selectors.developerStartedDurationLabel() }}</span>
            </div>
            <div class="actions no-wrap">
              <AppButton
                :tone="store.selectors.developerStatusPhase() === 'running' ? 'danger' : 'secondary'"
                icon-only
                :icon="store.selectors.developerStatusPhase() === 'running' ? 'X' : 'Play'"
                :disabled="store.selectors.developerStatusPhase() === 'starting' || store.selectors.developerStatusPhase() === 'stopping'"
                @click="store.actions.toggleDeveloperEnvironmentAction"
              />
              <AppButton icon-only icon="RefreshCw" :disabled="store.state.developerAction.busy" @click="store.actions.restartDeveloperEnvironmentAction" />
            </div>
          </div>
        </article>

        <article class="panel compact-panel side-panel">
          <header class="panel-head">
            <div>
              <h2>开发隧道</h2>
              <p>{{ store.selectors.developerTunnelStateLabel() }}</p>
            </div>
            <button class="toggle" :class="{ 'is-on': store.selectors.developerTunnelIsManaged() }" type="button" @click="store.actions.toggleDeveloperTunnel"></button>
          </header>
          <div class="inline-field">
            <input v-model="store.state.developerForm.tunnelCode" placeholder="输入隧道配置码" />
            <AppButton icon="Cloud" @click="store.actions.redeemDeveloperTunnelAction">激活</AppButton>
          </div>
          <div v-if="store.selectors.developerTunnelBaseUrl()" class="summary-box tunnel-summary">
            <div class="mini-table single">
              <span>隧道地址</span>
              <strong>{{ store.selectors.developerTunnelBaseUrl() }}</strong>
            </div>
            <AppButton small icon="Copy" :copied="store.selectors.copyFeedbackActive('dev-tunnel-url')" @click="store.actions.copyText(store.selectors.developerTunnelBaseUrl(), 'dev-tunnel-url')">复制地址</AppButton>
          </div>
        </article>

        <article class="panel compact-panel side-panel">
          <header class="panel-head">
            <div>
              <h2>发送到手表</h2>
              <p>目标设备</p>
            </div>
            <AppButton small icon="RefreshCw" :loading="store.state.installerRefreshRunning" :disabled="store.state.installerRefreshRunning" @click="store.actions.refreshInstallerState">刷新</AppButton>
          </header>
          <div class="device-line">
            <strong>{{ store.selectors.developerDeviceConnectionLabel() }}</strong>
            <ToneChip :tone="store.selectedDevice.value ? 'ok' : 'soft'">{{ store.selectedDevice.value ? "已连接" : "未连接" }}</ToneChip>
          </div>
          <InfoTable compact :rows="[
            ['Base URL', developerSummary.baseURL || '未设置'],
            ['Healthz', developerSummary.healthz ? developerSummary.healthz.replace(developerSummary.baseURL, '') || '/healthz' : '未设置'],
            ['Bootstrap', developerSummary.bootstrap]
          ]" />
          <AppButton block tone="primary" icon="Watch" :disabled="!store.selectors.developerCanSendToWatch()" @click="store.actions.applyDevWatchBootstrapAction">发送开发环境到手表</AppButton>
          <p v-if="store.selectors.developerSendDisabledReason()" class="muted-line">{{ store.selectors.developerSendDisabledReason() }}</p>
          <div class="actions two">
            <AppButton block icon="Copy" :copied="store.selectors.copyFeedbackActive('dev-bootstrap-uri')" @click="store.actions.prepareDevWatchBootstrapAction('dev-bootstrap-uri')">生成并复制 Bootstrap URI</AppButton>
            <AppButton block icon="Copy" :copied="store.selectors.copyFeedbackActive('dev-send-base-url')" @click="store.actions.copyText(developerSummary.baseURL, 'dev-send-base-url')">复制 Base URL</AppButton>
          </div>
        </article>
      </aside>
    </section>

    <section v-else-if="store.state.settingsTab === 'danger'" class="danger-zone">
      <header class="danger-zone-head">
        <h2>危险操作</h2>
        <p>这些操作不可逆，执行前请确认目标范围。</p>
      </header>
      <div class="danger-grid">
        <article class="danger-card">
          <h3>重置 Desktop 配置</h3>
          <p>恢复默认状态，保留日志与诊断数据。</p>
          <AppButton block tone="danger" @click="store.actions.showNotice('重置 Desktop 配置 动作尚未接入。')">重置</AppButton>
        </article>
        <article class="danger-card">
          <h3>重置本机服务配置</h3>
          <p>清除本机服务配置文件，恢复默认配置。</p>
          <AppButton block tone="danger" @click="store.actions.showNotice('重置本机服务配置 动作尚未接入。')">重置</AppButton>
        </article>
        <article class="danger-card">
          <h3>清空 beta 配对</h3>
          <p>只清除本机服务 beta 槽位中的配对信息。</p>
          <AppButton block tone="danger" :disabled="store.selectors.pairingActionBusy()" @click="store.actions.clearBackendPairingAction('beta')">
            {{ store.selectors.pairingActionLabel("beta", "清空", "正在清空...") }}
          </AppButton>
        </article>
        <article class="danger-card">
          <h3>撤销托管隧道</h3>
          <p>撤销所有托管隧道与相关凭证。</p>
          <AppButton block tone="danger" @click="store.actions.showNotice('撤销托管隧道 动作尚未接入。')">撤销</AppButton>
        </article>
      </div>
    </section>

    <section v-else class="settings-grid">
      <template v-if="store.state.settingsTab === 'general'">
        <article class="panel compact-panel">
          <header class="panel-head"><h2>常规设置</h2></header>
          <div class="toggle-list">
            <button
              v-for="row in generalToggleRows"
              :key="row.key"
              class="toggle-row"
              type="button"
              :aria-pressed="store.state.settingsPreferences[row.key]"
              @click="store.actions.toggleSettingsPreference(row.key)"
            >
              <span>{{ row.label }}</span>
              <span class="toggle" :class="{ 'is-on': store.state.settingsPreferences[row.key] }" aria-hidden="true"></span>
            </button>
          </div>
          <div class="form-grid two">
            <FieldRow label="语言"><input value="简体中文" readonly /></FieldRow>
            <FieldRow label="主题">
              <div class="theme-segmented" role="group" aria-label="主题">
                <button
                  type="button"
                  :class="{ 'is-active': store.state.theme === 'dark' }"
                  @click="store.actions.setTheme('dark')"
                >
                  深色（默认）
                </button>
                <button
                  type="button"
                  :class="{ 'is-active': store.state.theme === 'light' }"
                  @click="store.actions.setTheme('light')"
                >
                  浅色
                </button>
              </div>
            </FieldRow>
          </div>
        </article>

        <article class="panel compact-panel widget-settings-card">
          <header class="panel-head widget-settings-head">
            <div>
              <h2>桌面悬浮球</h2>
              <p>常驻显示额度环，单击后直接展开完整用量面板。</p>
            </div>
            <div class="widget-settings-control">
              <ToneChip :tone="store.selectors.floatingWidgetStatusTone()">
                {{ store.selectors.floatingWidgetStatusLabel() }}
              </ToneChip>
              <button
                class="widget-toggle-button"
                type="button"
                :aria-label="store.state.floatingWidget.enabled ? '关闭桌面悬浮球' : '开启桌面悬浮球'"
                :aria-pressed="store.state.floatingWidget.enabled"
                :disabled="store.state.floatingWidget.busy || store.state.floatingWidget.repairing || store.state.floatingWidget.refreshing"
                @click="store.actions.toggleFloatingWidgetAction"
              >
                <span class="toggle" :class="{ 'is-on': store.state.floatingWidget.enabled }" aria-hidden="true"></span>
              </button>
            </div>
          </header>
          <div class="inline-alert" :class="{ error: store.state.floatingWidget.enabled && !store.state.floatingWidget.running && store.state.floatingWidget.message }">
            <strong>{{ store.selectors.floatingWidgetStatusLabel() }}</strong>
            <span>{{ store.selectors.floatingWidgetStatusDetail() }}</span>
          </div>
          <InfoTable :rows="floatingWidgetRows" />
          <div class="actions">
            <AppButton
              icon="RefreshCw"
              :loading="store.state.floatingWidget.refreshing"
              :disabled="store.state.floatingWidget.busy || store.state.floatingWidget.repairing || store.state.floatingWidget.refreshing"
              @click="store.actions.refreshFloatingWidgetStatus({ notify: true })"
            >刷新状态</AppButton>
            <AppButton
              icon="ShieldCheck"
              :loading="store.state.floatingWidget.repairing"
              :disabled="store.state.floatingWidget.busy || store.state.floatingWidget.repairing || store.state.floatingWidget.refreshing"
              @click="store.actions.repairFloatingWidgetCredentialAction"
            >重新授权</AppButton>
          </div>
        </article>
      </template>

      <article v-else-if="store.state.settingsTab === 'backend'" class="panel compact-panel">
        <header class="panel-head"><h2>本机服务</h2></header>
        <InfoTable :rows="[
          ['本机服务构建号', store.live.value.backendBuildVersion || '未上报'],
          ['配置文件路径', store.state.snapshot.backend?.configPathLabel || '~/.openwatcher/config.json'],
          ['默认监听地址', store.live.value.listen],
          ['截图目录', '~/.openwatcher/screenshots'],
          ['诊断目录', '~/.openwatcher/diagnostics']
        ]" />
        <div class="actions">
          <AppButton icon="FolderOpen" @click="store.actions.openFolderAction('OpenBackendConfigDir', '已打开本机服务配置目录。')">打开配置目录</AppButton>
          <AppButton icon="RefreshCw" @click="store.actions.restartBackendAction">重启本机服务</AppButton>
          <AppButton icon="RefreshCw" :loading="store.state.backendHealthCheckRunning" :disabled="store.state.backendHealthCheckRunning" @click="store.actions.runHealthCheckAction">健康检查</AppButton>
        </div>
      </article>

      <template v-else-if="store.state.settingsTab === 'codex'">
        <article class="panel compact-panel">
          <header class="panel-head"><h2>Codex 环境</h2></header>
          <FieldRow label="Codex Home"><input :value="store.live.value.codexHomeLabel" readonly /></FieldRow>
          <InfoTable :rows="[
            ['auth.json', store.live.value.codex.authDetected ? '已检测' : '未检测'],
            ['sessions', store.live.value.codex.sessionsDetected ? '已检测' : '未检测']
          ]" />
          <div class="actions">
            <AppButton icon="FolderOpen" @click="store.actions.openFolderAction('OpenCodexHome', '已打开 Codex 目录。')">打开 Codex 目录</AppButton>
            <AppButton icon="RefreshCw" :loading="store.state.globalHealthRunning" :disabled="store.state.globalHealthRunning" @click="store.actions.runGlobalHealthCheck({ manual: true })">重新检测</AppButton>
          </div>
        </article>

        <article class="panel compact-panel">
          <header class="panel-head">
            <div>
              <h2>压缩状态 hooks</h2>
              <p>{{ store.state.codexHookState?.message || "用于让手表显示 Codex 压缩状态。" }}</p>
            </div>
            <ToneChip :tone="codexHookTone">{{ codexHookStatusLabel }}</ToneChip>
          </header>
          <InfoTable :rows="codexHookRows" />
          <div class="inline-alert">
            <strong>Codex App 审核</strong>
            <span>Desktop 只能确认 hooks 已写入；信任状态请到 Codex App 设置页左侧“钩子”的“来自用户配置”中查看。</span>
          </div>
          <div class="actions">
            <AppButton tone="primary" icon="Anchor" :loading="store.state.codexHooksInstalling" :disabled="store.state.codexHooksInstalling" @click="store.actions.installCodexHooksAction">
              {{ store.state.codexHookState?.installed ? "更新 hooks" : "安装 hooks" }}
            </AppButton>
            <AppButton icon="RefreshCw" :disabled="store.state.codexHooksInstalling" @click="store.actions.refreshCodexHookStatusAction({ notifySuccess: true })">刷新状态</AppButton>
            <AppButton icon="FolderOpen" :disabled="!store.state.codexHookState?.hooksPath" @click="store.actions.openFolderAction('OpenCodexHooksFile', '已打开 Codex hooks.json。')">打开 hooks.json</AppButton>
          </div>
        </article>
      </template>

      <article v-else-if="store.state.settingsTab === 'resources'" class="panel compact-panel">
        <header class="panel-head"><h2>手表安装资源</h2></header>
        <InfoTable :rows="[
          ['ADB 版本', store.state.installerState?.adb?.version || '未检测'],
          ['缓存安装包版本', store.selectors.currentWatchPackageVersionLabel()],
          ['手表已安装版本', store.selectors.currentWatchInstalledVersionLabel()],
          ['APK 发布策略', store.state.installerState?.apk?.debug ? '仅开发 / 模拟器可用' : 'release 优先'],
          ['SHA-256', store.state.installerState?.apk?.sha256 ? store.state.installerState.apk.sha256.slice(0, 16) + '...' : '未检测'],
          ['cloudflared 状态', store.selectors.currentTunnel().running ? '运行中' : (store.selectors.currentTunnel().resolvedBinary ? '已检测' : '未检测')]
        ]" />
        <div class="actions">
          <AppButton icon="RefreshCw" :loading="store.state.installerRefreshRunning" :disabled="store.state.installerRefreshRunning" @click="store.actions.refreshInstallerState">重新检测资源</AppButton>
          <AppButton icon="Watch" @click="store.state.currentPage = 'install'">返回安装向导</AppButton>
        </div>
      </article>

      <article v-else-if="store.state.settingsTab === 'privacy'" class="panel compact-panel">
        <header class="panel-head"><h2>隐私与安全</h2></header>
        <div class="toggle-list">
          <button
            v-for="row in privacyToggleRows"
            :key="row.key"
            class="toggle-row"
            type="button"
            :aria-pressed="store.state.settingsPreferences[row.key]"
            @click="store.actions.toggleSettingsPreference(row.key)"
          >
            <span>{{ row.label }}</span>
            <span class="toggle" :class="{ 'is-on': store.state.settingsPreferences[row.key] }" aria-hidden="true"></span>
          </button>
        </div>
        <p class="muted-line">不会在未经同意的情况下上传任何个人信息。</p>
      </article>

      <article v-else-if="store.state.settingsTab === 'updates'" class="panel compact-panel">
        <header class="panel-head"><h2>更新</h2></header>
        <div class="updates-matrix" :class="{ 'has-result': hasUpdateResult }">
          <span class="updates-col updates-col-label updates-head">项目</span>
          <span class="updates-col updates-col-current updates-head">当前版本</span>
          <span v-if="hasUpdateResult" class="updates-col updates-col-status updates-head">检查结果</span>

          <template v-for="row in updateRows" :key="row.label">
            <span class="updates-col updates-col-label">{{ row.label }}</span>
            <strong class="updates-col updates-col-current">{{ row.current }}</strong>
            <span v-if="hasUpdateResult" class="updates-col updates-col-status update-status" :class="`is-${row.tone}`">{{ row.status }}</span>
          </template>
        </div>
        <AppButton icon="RefreshCw" :loading="store.state.updateCheckRunning" :disabled="store.state.updateCheckRunning" @click="store.actions.checkForUpdatesAction">检查更新</AppButton>
        <div v-if="hasUpdateResult && store.state.updateCheckResult?.desktopUpdateAvailable" class="update-actions">
          <AppButton
            tone="primary"
            icon="Download"
            :loading="store.state.desktopUpdateRunning"
            :disabled="store.state.desktopUpdateRunning || !store.state.updateCheckResult?.desktopInstallable"
            @click="store.actions.installDesktopUpdateAction"
          >
            下载并安装 Desktop 更新
          </AppButton>
          <span v-if="!store.state.updateCheckResult?.desktopInstallable" class="muted-line">{{ store.state.updateCheckResult?.desktopInstallMessage || "当前更新包不能自动安装。" }}</span>
        </div>
        <div v-if="store.state.desktopUpdateProgress" class="update-progress">
          <div class="update-progress-track"><span :style="desktopUpdateProgressStyle"></span></div>
          <p class="muted-line">{{ store.selectors.desktopUpdateProgressLabel() }}</p>
        </div>
        <div v-if="desktopUpdateNotice" class="inline-alert" :class="{ error: store.state.desktopUpdateStatus?.phase === 'failed' }">
          <strong>{{ store.state.desktopUpdateStatus?.phase === "failed" ? "Desktop 更新失败" : "Desktop 更新" }}</strong>
          <span>{{ desktopUpdateNotice }}</span>
        </div>
        <div v-if="releaseNotes.length" class="update-notes">
          <h3>更新日志</h3>
          <ul>
            <li v-for="note in releaseNotes" :key="`${note.component}-${note.text}`">
              <strong>{{ note.component }}</strong>
              <span>{{ note.text }}</span>
            </li>
          </ul>
        </div>
        <div v-if="store.state.updateCheckResult?.error" class="inline-alert error">
          <strong>更新检查失败</strong>
          <span>{{ store.state.updateCheckResult.error }}</span>
        </div>
        <p v-else-if="hasUpdateResult" class="muted-line update-meta">{{ updateMetaLine }}</p>
      </article>

      <article v-else class="panel compact-panel">
        <header class="panel-head"><h2>高级</h2></header>
        <div class="toggle-list">
          <component
            :is="row.type === 'toggle' ? 'button' : 'div'"
            v-for="row in advancedRows"
            :key="row.key"
            class="toggle-row"
            :type="row.type === 'toggle' ? 'button' : undefined"
            :aria-pressed="row.type === 'toggle' ? store.state.settingsPreferences[row.key] : undefined"
            @click="row.type === 'toggle' ? store.actions.toggleSettingsPreference(row.key) : undefined"
          >
            <span>{{ row.label }}</span>
            <strong v-if="row.type === 'text'">{{ row.value }}</strong>
            <span v-else class="toggle" :class="{ 'is-on': store.state.settingsPreferences[row.key] }" aria-hidden="true"></span>
          </component>
        </div>
        <AppButton icon="FolderOpen" @click="store.actions.openFolderAction('OpenDesktopConfigDir', '已打开 Desktop 配置目录。')">打开 Desktop 目录</AppButton>
      </article>
    </section>
  </section>
</template>
