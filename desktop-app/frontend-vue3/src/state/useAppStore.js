import { computed, inject, reactive } from "vue"
import {
  DEFAULT_DEV_SIDECAR_PORT,
  DEFAULT_SIDECAR_PORT,
  DEVELOPER_STARTUP_WAIT_MS,
  GLOBAL_HEALTH_INTERVAL_MS,
  createInitialState,
  createInstallConfigEntries,
  fallbackInstallerState,
  fallbackSnapshot,
  installConfigMeta,
  installWizardStages,
  navItems,
  prepareManualChecks,
  sampleDevices,
  sampleEventTimeline,
  sampleRawLogs,
  settingsTabs,
  suggestedLanURL
} from "./defaults.js"
import { copyToClipboard, invoke, onRuntimeEvent } from "../services/wails.js"
import { filenameFromPath, formatByteSize, formatDeveloperLogTime, formatTimeAgo, shortTimeLabel, trimTrailingSlash } from "../utils/format.js"

const DEV_ENV_STORAGE_KEY = "openwatcher-dev-environment"
const INSTALL_NETWORK_STORAGE_KEY = "openwatcher-install-network"
const THEME_STORAGE_KEY = "openwatcher-theme"
const SETTINGS_PREFERENCES_STORAGE_KEY = "openwatcher-settings-preferences"

export const appStoreKey = Symbol("openwatcher-app-store")

export function useAppStore() {
  const store = inject(appStoreKey)
  if (!store) {
    throw new Error("OpenWatcher store 未初始化")
  }
  return store
}

export function createAppStore() {
  const state = reactive(createInitialState())
  let copyFeedbackTimer = null
  let desktopUpdateProgressOff = null
  let desktopStateChangedOff = null
  let installerProgressTicker = null

  const live = computed(() => deriveLiveState(state.snapshot))
  const topbarItems = computed(() => topbarStatusItems(live.value))
  const selectedDevice = computed(() => selectedInstallerDevice())
  const installerDevicesList = computed(() => installerDevices())
  const currentApi = computed(() => currentApiBase(live.value))
  const watchApi = computed(() => currentWatchApiBase(live.value))
  const unreadNotificationCount = computed(() => state.notifications.filter((item) => !item.read).length)
  const currentWizardStage = computed(() => wizardStageMeta())
  const wizardFailure = computed(() => wizardCurrentStageError())
  const rawLogLines = computed(() => filteredRawLogs())
  const allRawLogLines = computed(() => combinedRawLogs())
  const eventTimeline = computed(() => filteredTimeline())
  const watchSummaryCards = computed(() => buildWatchSummaryCards())
  const logsSummaryCards = computed(() => buildLogsSummaryCards())

  function availableNetworkOptions(snapshot = state.snapshot) {
    const context = snapshot.networkContext || fallbackSnapshot.networkContext
    return Array.isArray(context.interfaces) && context.interfaces.length > 0
      ? context.interfaces
      : fallbackSnapshot.networkContext.interfaces
  }

  function currentSelectedIP(snapshot = state.snapshot) {
    return state.networkForm.selectedIp
      || snapshot.networkContext?.recommendedIp
      || fallbackSnapshot.networkContext.recommendedIp
  }

  function currentListenPort(currentLive = live.value) {
    const listen = currentLive?.listen || state.snapshot?.backend?.lastHealth?.config?.listen || state.snapshot?.backend?.configuredListen || ""
    return listen.split(":").pop() || state.networkForm.port || DEFAULT_SIDECAR_PORT
  }

  function currentApiBase(currentLive = live.value) {
    const port = currentListenPort(currentLive)
    const selectedIP = currentSelectedIP()
    if (state.networkMode === "public") {
      return state.networkForm.customUrl.trim() || "https://openwatcher.example.com"
    }
    if (state.networkMode === "tunnel") {
      return state.snapshot?.tunnel?.publicBaseUrl || "https://等待兑换.openwatcher.ai"
    }
    return `http://${selectedIP}:${port}`
  }

  function currentWatchApiBase(currentLive = live.value) {
    const port = currentListenPort(currentLive)
    const device = preferredBackendDevice()
    if (device?.isEmulator && state.networkMode === "lan") {
      return `http://${device.hostAlias || "10.0.2.2"}:${port}`
    }
    return currentApiBase(currentLive)
  }

  function currentAccessModeLabel() {
    if (state.networkMode === "public") {
      return "自定义公网 URL"
    }
    if (state.networkMode === "tunnel") {
      return "OpenWatcher 托管隧道"
    }
    return "局域网模式"
  }

  function currentTunnel(snapshot = state.snapshot) {
    return snapshot?.tunnel || fallbackSnapshot.tunnel
  }

  function deriveLiveState(snapshot) {
    const backend = snapshot.backend || fallbackSnapshot.backend
    const codex = snapshot.codex || fallbackSnapshot.codex
    const tunnel = snapshot.tunnel || fallbackSnapshot.tunnel
    const networkContext = snapshot.networkContext || fallbackSnapshot.networkContext
    const health = backend.lastHealth || {}
    const healthConfig = health.config || {}
    const publicBaseUrl = healthConfig.publicBaseUrl || backend.configuredPublicBaseUrl || "http://127.0.0.1:8787"
    const listen = healthConfig.listen || backend.configuredListen || "127.0.0.1:8787"
    const backendHealthy = Boolean(health.ok)
    const backendErrored = backend.state === "error"
    const codexHealthy = Boolean(codex.readable && codex.authDetected && codex.sessionsDetected)
    const managedTunnelSelected = Boolean(tunnel.publicBaseUrl && publicBaseUrl === tunnel.publicBaseUrl)
    const accessMode = managedTunnelSelected
      ? "托管隧道"
      : publicBaseUrl.includes("192.168.")
        ? "局域网模式"
        : publicBaseUrl.startsWith("https://")
          ? "公网模式"
          : "本机模式"

    return {
      backend,
      codex,
      tunnel,
      publicBaseUrl,
      listen,
      backendHealthy,
      codexHealthy,
      accessMode,
      networkContext,
      codexHomeLabel: codex.homeLabel === "CODEX_HOME" ? "CODEX_HOME" : "~/.codex",
      codexStatusLabel: codexHealthy ? "已检测" : "待处理",
      codexStatusNote: codex.message || "未检测到 Codex 目录",
      backendStatusLabel: backendHealthy ? "运行中" : (backendErrored ? "异常" : "未启动"),
      backendStatusNote: backendHealthy ? listen : (backendErrored ? (backend.friendlyError || backend.message || "启动失败") : "等待启动本机服务"),
      backendBuildVersion: health.build?.version || "",
      accessStatusLabel: accessMode,
      accessStatusNote: publicBaseUrl,
      remoteExposureLabel: managedTunnelSelected
        ? (tunnel.running ? "已连接" : (tunnel.configured ? "待启动" : "未配置"))
        : (publicBaseUrl.startsWith("https://") ? "已启用" : "未启用"),
      networkHealthy: backendHealthy,
      healthConfig,
      codexHealth: health.codex || {}
    }
  }

  function topbarStatusItems(currentLive) {
    const targets = state.globalHealthSummary?.targets || {}
    const items = [
      {
        id: "codex",
        icon: "Terminal",
        label: "Codex",
        ok: Boolean(targets.codex?.ok ?? currentLive.codexHealthy)
      },
      {
        id: "backend",
        icon: "Server",
        label: "本机服务",
        ok: Boolean(targets.backend?.ok ?? currentLive.backendHealthy)
      },
      {
        id: "resources",
        icon: "PackageCheck",
        label: "安装资源",
        ok: Boolean(targets.resources?.ok ?? (state.installerState?.adb?.available && state.installerState?.apk?.available))
      }
    ]

    if (developerStatusVisible()) {
      items.push({
        id: "developer",
        icon: "Wand2",
        label: "开发环境",
        ok: Boolean(targets.developer?.ok ?? developerIsHealthy())
      })
    }
    if (developerTunnelStatusVisible()) {
      items.push({
        id: "developerTunnel",
        icon: "Cloud",
        label: "开发隧道",
        ok: Boolean(targets.developerTunnel?.ok ?? developerTunnelIsHealthy())
      })
    }
    if (targets.publicEntry) {
      items.push({
        id: "public",
        icon: "Globe2",
        label: "公网入口",
        ok: Boolean(targets.publicEntry.ok)
      })
    }
    if (targets.tunnelEntry) {
      items.push({
        id: "tunnel",
        icon: "Cloud",
        label: "托管隧道",
        ok: Boolean(targets.tunnelEntry.ok)
      })
    }
    return items
  }

  function installerDevices() {
    return Array.isArray(state.installerState?.devices) ? state.installerState.devices : []
  }

  function connectedEmulatorDevices() {
    return installerDevices().filter((device) => device.isEmulator && device.state === "device")
  }

  function singleConnectedEmulator() {
    const devices = connectedEmulatorDevices()
    return devices.length === 1 ? devices[0] : null
  }

  function installerSelectionRequired() {
    return installerDevices().length > 1 && !state.installerState?.selectedSerial
  }

  function selectedInstallerDevice() {
    const devices = installerDevices()
    const selectedSerial = state.installerState?.selectedSerial
    if (selectedSerial) {
      return devices.find((device) => device.serial === selectedSerial) || null
    }
    if (devices.length === 1) {
      return devices[0]
    }
    return null
  }

  function preferredBackendDevice() {
    return selectedInstallerDevice() || singleConnectedEmulator()
  }

  function installerStatusTone() {
    if (!state.installerState?.adb?.available) {
      return "amber"
    }
    return selectedInstallerDevice() || installerSelectionRequired() ? "green" : "amber"
  }

  function installerStatusLabel() {
    const device = selectedInstallerDevice()
    if (device) {
      return state.installerState.apk?.available ? "已连接 / 可安装" : "已连接"
    }
    return state.installerState?.adb?.available ? "等待连接" : "ADB 不可用"
  }

  function installerStatusNote() {
    const currentInstaller = state.installerState || fallbackInstallerState
    const device = selectedInstallerDevice()
    if (!currentInstaller.adb?.available) {
      return currentInstaller.adb?.message || "未检测到 ADB"
    }
    if (device) {
      return `${device.displayName || device.serial} · ${device.serial}`
    }
    if (installerSelectionRequired()) {
      return `已发现 ${installerDevices().length} 个 ADB 设备，请先选择目标手表`
    }
    if (pairingSkipReason()) {
      return pairingSkipReason()
    }
    return currentInstaller.message || "等待无线 ADB 配对或现有模拟器设备"
  }

  function installerSummaryNote() {
    const currentInstaller = state.installerState || fallbackInstallerState
    if (currentInstaller.message) {
      return currentInstaller.message
    }
    if (currentInstaller.apk?.devFallback) {
      return "当前仅检测到 debug APK。它只用于开发或模拟器验证，真实设备与公开发布请改用 release 包。"
    }
    if (currentInstaller.apk?.available) {
      return `已检测安装包：${currentInstaller.apk.versionName || "未知版本"}`
    }
    return currentInstaller.apk?.message || "尚未找到可安装的手表 APK。"
  }

  function installerRuntimeResource(kind) {
    return state.installerState?.runtime?.resources?.[kind] || null
  }

  function installerRuntimeResourceActive(kind) {
    const progress = installerRuntimeResource(kind)
    return Boolean(progress && !progress.ready && ["checking", "downloading", "verifying", "extracting"].includes(progress.phase))
  }

  function installerRuntimeProgressActive() {
    return installerRuntimeResourceActive("platformTools") || installerRuntimeResourceActive("watchApk")
  }

  function updateInstallerProgressTicker() {
    if (typeof window === "undefined") {
      return
    }
    if (installerRuntimeProgressActive()) {
      if (installerProgressTicker) {
        return
      }
      const setIntervalFn = window.setInterval || setInterval
      installerProgressTicker = setIntervalFn(async () => {
        state.installerState = await loadInstallerStatus()
        syncWizardWithInstallerState()
      }, 1000)
      return
    }
    if (installerProgressTicker) {
      const clearIntervalFn = window.clearInterval || clearInterval
      clearIntervalFn(installerProgressTicker)
      installerProgressTicker = null
    }
  }

  function runtimeProgressVisible(progress) {
    return Boolean(progress && !progress.ready && ["downloading", "verifying", "extracting"].includes(progress.phase))
  }

  function runtimeProgressPercent(progress) {
    if (!progress) {
      return 0
    }
    const explicit = Number(progress.percent || 0)
    if (explicit > 0) {
      return Math.max(0, Math.min(100, explicit))
    }
    const downloaded = Number(progress.downloadedBytes || 0)
    const total = Number(progress.totalBytes || 0)
    if (total <= 0 || downloaded <= 0) {
      return progress.phase === "extracting" || progress.phase === "verifying" ? 100 : 0
    }
    return Math.max(0, Math.min(100, Math.round((downloaded / total) * 100)))
  }

  function runtimeProgressLabel(progress) {
    if (!progress) {
      return ""
    }
    const message = progress.message || runtimePhaseLabel(progress.phase)
    const downloaded = Number(progress.downloadedBytes || 0)
    const total = Number(progress.totalBytes || 0)
    const sizeLabel = total > 0 && downloaded > 0
      ? `${formatByteSize(downloaded)} / ${formatByteSize(total)}`
      : ""
    const speed = Number(progress.bytesPerSecond || 0)
    const speedLabel = progress.phase === "downloading" && speed > 0
      ? `${formatByteSize(speed)}/s`
      : ""
    return [message, speedLabel, sizeLabel].filter(Boolean).join(" · ")
  }

  function runtimePhaseLabel(phase) {
    if (phase === "downloading") {
      return "正在下载"
    }
    if (phase === "verifying") {
      return "正在校验"
    }
    if (phase === "extracting") {
      return "正在解压"
    }
    if (phase === "error") {
      return "准备失败"
    }
    return "准备中"
  }

  function runtimeProgressTag(progress, fallback) {
    if (!progress || progress.ready) {
      return fallback
    }
    if (progress.phase === "downloading") {
      return "下载中"
    }
    if (progress.phase === "verifying") {
      return "校验中"
    }
    if (progress.phase === "extracting") {
      return "解压中"
    }
    if (progress.phase === "error") {
      return "失败"
    }
    return "准备中"
  }

  function currentDeveloperHostAlias() {
    const device = preferredBackendDevice()
    return String(device?.hostAlias || state.developerForm.hostAlias || "10.0.2.2").trim()
  }

  function normalizeDeveloperAccessMode(value) {
    return ["emulator", "lan", "tunnel", "custom"].includes(value) ? value : "emulator"
  }

  function developerTunnelBaseUrl() {
    return trimTrailingSlash(developerTunnelStatus()?.publicBaseUrl || "")
  }

  function developerSuggestedBaseUrl(accessMode = state.developerForm.accessMode) {
    const mode = normalizeDeveloperAccessMode(accessMode)
    const port = developerPortValue()
    const hostAlias = currentDeveloperHostAlias() || "10.0.2.2"
    if (mode === "lan") {
      return `http://${currentSelectedIP()}:${port}`
    }
    if (mode === "tunnel") {
      return developerTunnelBaseUrl() || state.developerForm.devBaseUrl.trim() || `http://${hostAlias}:${port}`
    }
    if (mode === "custom") {
      return state.developerForm.devBaseUrl.trim()
    }
    return `http://${hostAlias}:${port}`
  }

  function developerBaseUrlForCurrentForm() {
    return trimTrailingSlash(state.developerForm.devBaseUrl) || trimTrailingSlash(developerSuggestedBaseUrl())
  }

  function developerPortValue(accessMode = state.developerForm.accessMode) {
    const mode = normalizeDeveloperAccessMode(accessMode)
    if (mode === "emulator" || mode === "lan" || mode === "tunnel") {
      return DEFAULT_DEV_SIDECAR_PORT
    }
    const currentBaseURL = trimTrailingSlash(state.developerForm.devBaseUrl || developerStatus()?.baseURL || "")
    if (currentBaseURL) {
      try {
        const parsed = new URL(currentBaseURL)
        return parsed.port || DEFAULT_DEV_SIDECAR_PORT
      } catch {
        return DEFAULT_DEV_SIDECAR_PORT
      }
    }
    return DEFAULT_DEV_SIDECAR_PORT
  }

  function updateDeveloperBaseUrlFromAccessMode(nextMode = state.developerForm.accessMode) {
    const normalizedMode = normalizeDeveloperAccessMode(nextMode)
    state.developerForm.accessMode = normalizedMode
    if (normalizedMode !== "custom") {
      state.developerForm.devBaseUrl = trimTrailingSlash(developerSuggestedBaseUrl(normalizedMode))
    }
  }

  function developerAccessModeFromBaseUrl(baseURL = developerBaseUrlForCurrentForm()) {
    const normalized = trimTrailingSlash(baseURL)
    if (!normalized) {
      return state.developerForm.accessMode || "emulator"
    }
    const tunnelBaseURL = developerTunnelBaseUrl()
    if (tunnelBaseURL && normalized === tunnelBaseURL) {
      return "tunnel"
    }
    try {
      const parsed = new URL(normalized)
      const host = parsed.hostname
      const hostAlias = currentDeveloperHostAlias()
      const selectedIP = currentSelectedIP()
      if ([hostAlias, "10.0.2.2", "10.0.3.2"].includes(host)) {
        return "emulator"
      }
      if (host === selectedIP) {
        return "lan"
      }
    } catch {
      return "custom"
    }
    return "custom"
  }

  function developerHealthzUrl() {
    const baseURL = developerBaseUrlForCurrentForm()
    return baseURL ? `${baseURL}/healthz` : ""
  }

  function developerEnvironmentRequest(override = {}) {
    return {
      enabled: Object.prototype.hasOwnProperty.call(override, "enabled") ? Boolean(override.enabled) : Boolean(state.developerForm.enabled),
      mode: "workspace",
      repoPath: (override.repoPath ?? state.developerForm.repoPath).trim(),
      hostAlias: override.hostAlias || currentDeveloperHostAlias(),
      baseURL: trimTrailingSlash(override.baseURL || developerBaseUrlForCurrentForm()),
      deviceName: (override.deviceName ?? state.developerForm.deviceName).trim(),
      managedTunnelEnabled: Object.prototype.hasOwnProperty.call(override, "managedTunnelEnabled")
        ? Boolean(override.managedTunnelEnabled)
        : Boolean(state.developerForm.managedTunnelEnabled)
    }
  }

  function developerStatus() {
    return state.developerSnapshot?.status || null
  }

  function developerTunnelStatus() {
    return state.developerSnapshot?.tunnel || null
  }

  function developerTunnelIsManaged() {
    return Boolean(state.developerForm.managedTunnelEnabled)
  }

  function developerTunnelStateLabel() {
    const tunnel = developerTunnelStatus()
    if (!developerTunnelIsManaged()) {
      return "已关闭"
    }
    if (tunnel?.lastHealth && !tunnel.lastHealth.ok) {
      return "公网异常"
    }
    if (tunnel?.running && tunnel?.lastHealth?.ok) {
      return "运行中"
    }
    if (tunnel?.running) {
      return "检测中"
    }
    if (tunnel?.configured) {
      return "待启动"
    }
    return "未激活"
  }

  function developerLogs() {
    return Array.isArray(state.developerSnapshot?.logs) ? state.developerSnapshot.logs : []
  }

  function developerLogFileLabel() {
    return String(developerStatus()?.logFileLabel || "").trim()
  }

  function developerLogEntries(limit = 0) {
    const lines = developerLogs()
    return limit > 0 ? lines.slice(-limit) : lines
  }

  function developerLogLines(limit = 0) {
    return developerLogEntries(limit).map((line) => `[${line.at || "--:--:--"}] [developer] ${line.message || ""}`)
  }

  function developerRecentLogMessage() {
    const lines = developerLogs()
    return lines.length > 0 ? (lines[lines.length - 1]?.message || "") : ""
  }

  function developerStatusPhase() {
    const currentStatus = developerStatus()
    if (state.developerAction.busy) {
      return state.developerAction.targetEnabled ? "starting" : "stopping"
    }
    if (!currentStatus) {
      return "stopped"
    }
    if (currentStatus.running && currentStatus.lastHealth?.ok) {
      return "running"
    }
    if (currentStatus.state === "recovering" || currentStatus.running) {
      return "starting"
    }
    if (currentStatus.state === "error") {
      return "failed"
    }
    return "stopped"
  }

  function developerIsRunning() {
    return developerStatusPhase() === "running" || developerStatusPhase() === "starting"
  }

  function developerIsHealthy() {
    return Boolean(developerStatus()?.lastHealth?.ok)
  }

  function developerTunnelIsHealthy() {
    const tunnel = developerTunnelStatus()
    return Boolean(developerTunnelIsManaged() && tunnel?.running && tunnel?.lastHealth?.ok)
  }

  function developerStatusVisible() {
    const status = developerStatus()
    return Boolean(
      state.developerAction.busy ||
      state.developerForm.enabled ||
      status?.running ||
      ["healthy", "recovering", "error"].includes(status?.state)
    )
  }

  function developerTunnelStatusVisible() {
    const tunnel = developerTunnelStatus()
    return developerStatusVisible() && Boolean(
      state.developerForm.managedTunnelEnabled ||
      tunnel?.running ||
      tunnel?.configured
    )
  }

  function developerStateTone() {
    const phase = developerStatusPhase()
    if (phase === "starting" || phase === "stopping") {
      return "blue"
    }
    if (phase === "running") {
      return "green"
    }
    if (phase === "failed") {
      return "amber"
    }
    return "neutral"
  }

  function developerStateLabel() {
    switch (developerStatusPhase()) {
      case "running":
        return "运行中"
      case "starting":
        return "启动中"
      case "stopping":
        return "停止中"
      case "failed":
        return "启动失败"
      default:
        return "未启动"
    }
  }

  function developerStartedDurationLabel() {
    const startedAt = developerStatus()?.startedAt
    if (!startedAt) {
      return developerStatus()?.externallyManaged ? "已检测到现有服务" : "等待启动"
    }
    const started = new Date(startedAt)
    if (Number.isNaN(started.getTime())) {
      return "运行中"
    }
    const diffMinutes = Math.floor(Math.max(0, Date.now() - started.getTime()) / (60 * 1000))
    if (diffMinutes < 1) {
      return "已启动不到 1 分钟"
    }
    if (diffMinutes < 60) {
      return `已启动 ${diffMinutes} 分钟`
    }
    const hours = Math.floor(diffMinutes / 60)
    const minutes = diffMinutes % 60
    return minutes === 0 ? `已启动 ${hours} 小时` : `已启动 ${hours} 小时 ${minutes} 分钟`
  }

  function developerStartCommand() {
    return String(developerStatus()?.startCommand || developerStatus()?.resolvedScriptPath || "").trim()
  }

  function developerCurrentRepoLabel() {
    const repoPath = developerStatus()?.resolvedRepoPath || state.developerForm.repoPath
    return repoPath ? filenameFromPath(repoPath) || repoPath : "未选择仓库"
  }

  function developerDeviceConnectionLabel() {
    const device = selectedInstallerDevice()
    if (device) {
      return `${device.displayName || state.developerForm.deviceName || "watch"}（已连接）`
    }
    return `${state.developerForm.deviceName || "watch"}（未连接）`
  }

  function developerCanSendToWatch() {
    return developerIsHealthy() && Boolean(selectedInstallerDevice())
  }

  function developerSendDisabledReason() {
    if (!developerIsHealthy()) {
      return "请先启动开发环境"
    }
    if (!selectedInstallerDevice()) {
      return "未检测到目标设备"
    }
    return ""
  }

  function developerEnvFileLabel() {
    return developerStatus()?.envFilePresent ? "从仓库 .env.development 加载" : "未检测到 .env.development"
  }

  function developerLogLevel(message) {
    const text = String(message || "").toUpperCase()
    if (text.includes("ERROR") || text.includes("失败") || text.includes("退出")) {
      return "error"
    }
    if (text.includes("WARN") || text.includes("WARNING") || text.includes("占用") || text.includes("变更")) {
      return "warn"
    }
    return "info"
  }

  function developerLogLevelLabel(message) {
    const level = developerLogLevel(message)
    if (level === "error") {
      return "ERROR"
    }
    if (level === "warn") {
      return "WARN"
    }
    return "INFO"
  }

  function wizardStageIndex(stageId) {
    return installWizardStages.findIndex((stage) => stage.id === stageId)
  }

  function wizardStageMeta(stageId = state.wizard.currentStage) {
    return installWizardStages.find((stage) => stage.id === stageId) || installWizardStages[0]
  }

  function setWizardStage(stageId, note = "") {
    state.wizard.currentStage = stageId
    state.wizard.maxUnlockedIndex = Math.max(state.wizard.maxUnlockedIndex, wizardStageIndex(stageId))
    if (note) {
      state.wizard.stageNotes[stageId] = note
    }
  }

  function markWizardStageCompleted(stageId, note = "") {
    if (!state.wizard.completedStages.includes(stageId)) {
      state.wizard.completedStages.push(stageId)
    }
    if (note) {
      state.wizard.stageNotes[stageId] = note
    }
  }

  function wizardStageCompleted(stageId) {
    return state.wizard.completedStages.includes(stageId)
  }

  function wizardStageUnlocked(stageId) {
    return wizardStageIndex(stageId) <= state.wizard.maxUnlockedIndex
  }

  function wizardCurrentStageError() {
    return state.wizard.error?.stage === state.wizard.currentStage ? state.wizard.error : null
  }

  function clearWizardStageError() {
    state.wizard.error = null
  }

  function setWizardStageError(stageId, message) {
    state.wizard.error = {
      stage: stageId,
      message: message || "当前步骤执行失败。"
    }
    if (message) {
      state.wizard.stageNotes[stageId] = message
    }
  }

  function pairingSkipReason() {
    const emulator = singleConnectedEmulator()
    return emulator ? `检测到已连接模拟器 ${emulator.serial}，已跳过无线配对。` : ""
  }

  function syncInstallWizardDraft(snapshot = state.snapshot) {
    const nextSuggestedLanURL = suggestedLanURL(snapshot)
    const lanEntry = state.wizard.configEntries.lan
    if (!lanEntry.url || lanEntry.url === state.wizard.suggestedLanURL) {
      lanEntry.url = nextSuggestedLanURL
    }
    state.wizard.suggestedLanURL = nextSuggestedLanURL

    const tunnelDomain = snapshot?.tunnel?.publicBaseUrl || ""
    if (tunnelDomain) {
      state.wizard.configEntries.tunnel.redeemedDomain = tunnelDomain
      if (state.wizard.configEntries.tunnel.validation === "pending") {
        state.wizard.configEntries.tunnel.validation = "idle"
      }
      if (!state.wizard.configEntries.tunnel.message || state.wizard.configEntries.tunnel.message === "先输入配置码再兑换") {
        state.wizard.configEntries.tunnel.message = "已兑换，可继续校验"
      }
    }

    if (!state.installForm.useSeparateConnectIP) {
      state.installForm.connectIp = state.installForm.pairIp
    }
  }

  function syncWizardWithInstallerState() {
    syncInstallWizardDraft(state.snapshot)
    if (state.installerState?.phase === "troubleshooting" && state.wizard.currentStage !== "prepare") {
      setWizardStageError(state.wizard.currentStage, state.installerState?.message || "当前步骤执行失败。")
    } else if (state.wizard.error && state.installerState?.phase !== "troubleshooting") {
      clearWizardStageError()
    }

    if (pairingSkipReason() && state.wizard.currentStage === "connect" && selectedInstallerDevice()) {
      state.wizard.stageNotes.connect = pairingSkipReason()
    }

    if (selectedInstallerDevice()) {
      state.wizard.stageNotes.connect = state.installerState?.message || `${selectedInstallerDevice().displayName || selectedInstallerDevice().serial} 已准备好继续安装。`
    }

    if (state.installerState?.apk?.installed && !wizardStageCompleted("install")) {
      markWizardStageCompleted("connect", state.wizard.stageNotes.connect || "设备已连接")
    }
    updateInstallerProgressTicker()
  }

  function wizardAutoChecks(currentLive = live.value) {
    const currentInstaller = state.installerState || fallbackInstallerState
    const apkName = currentInstaller.apk?.path ? filenameFromPath(currentInstaller.apk.path) : (currentInstaller.apk?.label || "未检测到安装包")
    const toolProgress = installerRuntimeResource("platformTools")
    const apkProgress = installerRuntimeResource("watchApk")
    const toolProgressVisible = runtimeProgressVisible(toolProgress)
    const apkProgressVisible = runtimeProgressVisible(apkProgress)
    return [
      {
        id: "tool",
        label: "安装工具可用",
        ok: Boolean(currentInstaller.adb?.available),
        tag: currentInstaller.adb?.available ? "已就绪" : runtimeProgressTag(toolProgress, "未就绪"),
        detail: currentInstaller.adb?.available
          ? ["已缓存到本机", `版本 ${currentInstaller.adb?.version || "已找到"}`]
          : (toolProgressVisible ? [] : [currentInstaller.adb?.message || toolProgress?.message || "未检测到安装工具"]),
        progress: toolProgressVisible ? {
          percent: runtimeProgressPercent(toolProgress),
          label: runtimeProgressLabel(toolProgress)
        } : null
      },
      {
        id: "package",
        label: "手表安装包可用",
        ok: Boolean(currentInstaller.apk?.available),
        tag: currentInstaller.apk?.available ? "已就绪" : runtimeProgressTag(apkProgress, "未就绪"),
        detail: currentInstaller.apk?.available
          ? [`文件 ${apkName}`, `版本 ${currentInstaller.apk?.versionName || "未知版本"} · ${currentInstaller.apk?.debug ? "debug" : "release"}`]
          : (apkProgressVisible ? [] : [currentInstaller.apk?.message || apkProgress?.message || "未检测到安装包"]),
        progress: apkProgressVisible ? {
          percent: runtimeProgressPercent(apkProgress),
          label: runtimeProgressLabel(apkProgress)
        } : null
      },
      {
        id: "backend",
        label: "本机服务可访问",
        ok: Boolean(currentLive.backendHealthy),
        tag: currentLive.backendHealthy ? "已就绪" : "未就绪",
        detail: [currentLive.backendStatusNote || "等待本机服务启动"]
      }
    ]
  }

  function wizardManualChecksDone() {
    return prepareManualChecks.every((item) => Boolean(state.wizard.manualChecks[item.id]))
  }

  function toggleAllWizardManualChecks() {
    const nextValue = !wizardManualChecksDone()
    for (const item of prepareManualChecks) {
      state.wizard.manualChecks[item.id] = nextValue
    }
  }

  function wizardPrepareReady(currentLive = live.value) {
    return wizardAutoChecks(currentLive).every((item) => item.ok) && wizardManualChecksDone()
  }

  function wizardConnectReady() {
    return Boolean(selectedInstallerDevice()) && !installerSelectionRequired()
  }

  function wizardConfigEntry(entryId) {
    return state.wizard.configEntries[entryId]
  }

  function wizardEnabledConfigEntries() {
    return Object.entries(state.wizard.configEntries).filter(([, entry]) => entry.enabled)
  }

  function wizardConfigChecksInProgress() {
    return wizardEnabledConfigEntries().some(([, entry]) => entry.validation === "checking")
  }

  function wizardConfigCheckingLabel() {
    const active = wizardEnabledConfigEntries().find(([, entry]) => entry.validation === "checking")
    return active ? installConfigMeta[active[0]].label : ""
  }

  function wizardInstallReady() {
    return Boolean(wizardStageCompleted("install") || state.installerState?.apk?.installed)
  }

  function wizardConfigReady() {
    return Boolean(wizardStageCompleted("config") || state.generatedBootstrap?.apiBase)
  }

  function configWriteActionLabel() {
    return state.generatedBootstrap?.apiBase ? "重新写入" : "写入配置"
  }

  function buildEnabledBootstrapEndpoints() {
    const selected = preferredBackendDevice()
    const isEmulator = Boolean(selected?.isEmulator)
    const emulatorHostAlias = selected?.hostAlias || "10.0.2.2"
    const endpoints = []

    if (wizardConfigEntry("lan")?.enabled && wizardConfigEntry("lan").url.trim()) {
      const target = new URL(wizardConfigEntry("lan").url.trim())
      endpoints.push({
        id: "lan",
        label: "局域网",
        url: isEmulator ? `http://${emulatorHostAlias}:${target.port || DEFAULT_SIDECAR_PORT}` : `${target.protocol}//${target.host}`,
        priority: endpoints.length
      })
    }
    if (wizardConfigEntry("public")?.enabled && wizardConfigEntry("public").url.trim()) {
      endpoints.push({
        id: "public",
        label: "公网",
        url: wizardConfigEntry("public").url.trim(),
        priority: endpoints.length
      })
    }
    if (wizardConfigEntry("tunnel")?.enabled && wizardConfigEntry("tunnel").redeemedDomain.trim()) {
      endpoints.push({
        id: "managedTunnel",
        label: "托管隧道",
        url: wizardConfigEntry("tunnel").redeemedDomain.trim(),
        priority: endpoints.length
      })
    }
    return endpoints
  }

  function wizardFailureTips() {
    const message = String(state.installerState?.message || "").toLowerCase()
    if (message.includes("多个 adb 设备") || message.includes("多个设备")) {
      return ["请在本页确认这次要安装的目标手表，再重新继续。"]
    }
    if (message.includes("认证失败") || message.includes("配对码")) {
      return ["重新打开手表的配对页面，确认地址、端口和配对码仍然有效。"]
    }
    if (message.includes("同一 wi")) {
      return ["确认电脑与手表在同一 Wi-Fi，避免访客网络、隔离网络或 VPN 干扰。"]
    }
    if (message.includes("abi")) {
      return ["当前安装包和设备不兼容，请换用匹配当前设备的安装包。"]
    }
    if (message.includes("签名")) {
      return ["当前安装包与设备中已有应用不兼容，请先确认安装包来源和版本。"]
    }
    if (message.includes("已存在配对信息")) {
      return ["如果你确实要重置当前本机配对，可直接去危险操作页清空。"]
    }
    if (message.includes("后端未就绪") || message.includes("本机服务未就绪")) {
      return ["请先确认本机服务可用，再重新写入配置。"]
    }
    return ["请重试当前步骤。", "如果问题重复出现，导出诊断包后再继续排查。"]
  }

  function currentBackendRequest() {
    const selected = preferredBackendDevice()
    const isEmulator = Boolean(selected?.isEmulator)
    const emulatorHostAlias = selected?.hostAlias || "10.0.2.2"
    const port = state.networkForm.port
    return {
      mode: state.networkMode,
      selectedIP: isEmulator && state.networkMode === "lan" ? "127.0.0.1" : currentSelectedIP(),
      port,
      customURL: state.networkForm.customUrl,
      tunnelCode: state.networkForm.tunnelCode,
      deviceName: state.watchForm.deviceName,
      publicBaseURL: isEmulator && state.networkMode === "lan" ? `http://${emulatorHostAlias}:${port || "8787"}` : "",
      endpoints: buildEnabledBootstrapEndpoints()
    }
  }

  function buildBackendRequestForEntry(entryId, options = {}) {
    const selected = preferredBackendDevice()
    const isEmulator = Boolean(selected?.isEmulator)
    const emulatorHostAlias = selected?.hostAlias || "10.0.2.2"
    const endpoints = options.endpoints || []
    if (entryId === "lan") {
      const target = new URL(wizardConfigEntry("lan").url)
      const effectivePort = target.port || DEFAULT_SIDECAR_PORT
      return {
        mode: "lan",
        selectedIP: isEmulator ? "127.0.0.1" : target.hostname,
        port: effectivePort,
        customURL: "",
        tunnelCode: "",
        deviceName: state.watchForm.deviceName,
        publicBaseURL: isEmulator ? `http://${emulatorHostAlias}:${effectivePort}` : "",
        endpoints
      }
    }
    if (entryId === "public") {
      return {
        mode: "public",
        selectedIP: currentSelectedIP(),
        port: state.networkForm.port || DEFAULT_SIDECAR_PORT,
        customURL: wizardConfigEntry("public").url.trim(),
        tunnelCode: "",
        deviceName: state.watchForm.deviceName,
        publicBaseURL: "",
        endpoints
      }
    }
    return {
      mode: "tunnel",
      selectedIP: currentSelectedIP(),
      port: DEFAULT_SIDECAR_PORT,
      customURL: "",
      tunnelCode: "",
      deviceName: state.watchForm.deviceName,
      publicBaseURL: "",
      endpoints
    }
  }

  function remoteBootstrapDefaultApiBase() {
    if (state.remoteBootstrapForm.environment === "dev") {
      return developerBaseUrlForCurrentForm() || suggestedDevBaseUrl()
    }
    return currentWatchApiBase(live.value)
  }

  function remoteBootstrapHealthLabel(result = state.remoteBootstrapForm.result) {
    if (!result) {
      return "尚未提交"
    }
    if (result.health?.ok) {
      return "已通过 /healthz"
    }
    return result.health?.message || "未通过 /healthz"
  }

  function remoteBootstrapEnvironmentLabel(value = state.remoteBootstrapForm.environment) {
    return value === "dev" ? "dev" : "beta"
  }

  function suggestedDevBaseUrl() {
    const selected = preferredBackendDevice()
    if (selected?.isEmulator) {
      return `http://${selected.hostAlias || "10.0.2.2"}:${DEFAULT_DEV_SIDECAR_PORT}`
    }
    return `http://${currentSelectedIP()}:${DEFAULT_DEV_SIDECAR_PORT}`
  }

  function normalizeLogLines(lines) {
    if (!Array.isArray(lines) || lines.length === 0) {
      return sampleRawLogs
    }
    return lines.map((line) => `[${line.at || "--:--:--"}] ${line.message || ""}`)
  }

  function installerLogLines() {
    const lines = state.installerState?.logs
    if (!Array.isArray(lines) || lines.length === 0) {
      return sampleRawLogs
    }
    return lines.map((line) => `[${line.at || "--:--:--"}] [${line.source || "adb"}] ${line.message || ""}`)
  }

  function combinedRawLogs() {
    return [...installerLogLines(), ...developerLogLines(), ...normalizeLogLines(state.backendLogs)]
  }

  function filteredRawLogs() {
    const rawLogs = combinedRawLogs()
    if (state.selectedLogSource === "all") {
      return rawLogs
    }
    const sourceMap = {
      Desktop: "[desktop]",
      本机服务: "[backend]",
      开发环境: "[developer]",
      "ADB 安装": "[adb]",
      "手表 App": "[watch]",
      网络访问: "[network]",
      托管隧道: "[tunnel]",
      更新: "[update]",
      安全: "[security]"
    }
    const token = sourceMap[state.selectedLogSource]
    if (!token) {
      return rawLogs
    }
    const matched = rawLogs.filter((line) => line.toLowerCase().includes(token))
    return matched.length > 0 ? matched : rawLogs
  }

  function filteredTimeline() {
    const dynamicTimeline = Array.isArray(state.installerState?.logs) && state.installerState.logs.length > 0
      ? state.installerState.logs.slice(-8).map((line) => ({
        time: String(line.at || "").slice(11, 19) || String(line.at || "").slice(0, 8) || "--:--:--",
        event: line.message || "",
        level: /fail|error|异常|失败/i.test(line.message || "") ? "ERROR" : "INFO",
        source: line.source === "adb" ? "ADB 安装" : (line.source || "Desktop")
      }))
      : sampleEventTimeline
    return state.selectedLogSource === "all"
      ? dynamicTimeline
      : dynamicTimeline.filter((item) => item.source === state.selectedLogSource)
  }

  function buildWatchSummaryCards() {
    const device = selectedInstallerDevice()
    const installer = state.installerState || fallbackInstallerState
    return [
      { icon: "Watch", title: "手表连接", value: device ? "已连接" : "未连接", meta: installerStatusNote(), tone: device ? "green" : "amber" },
      { icon: "Cpu", title: "当前设备", value: device?.displayName || "未选择设备", meta: device?.isWatch ? "Wear / Watch 类设备" : "等待连接", tone: device ? "blue" : "amber" },
      { icon: "PackageCheck", title: "Watch 版本", value: installer.apk?.versionName || "未检测", meta: installer.apk?.debug ? "debug 本地验证" : "release 缓存就绪", tone: installer.apk?.available ? "purple" : "amber" },
      { icon: "Globe2", title: "访问地址", value: currentApi.value, meta: "按当前模式生成", tone: "cyan" }
    ]
  }

  function buildLogsSummaryCards() {
    return [
      { icon: "Monitor", title: "Desktop", value: "正常", meta: state.snapshot.productVersion || "dev", tone: "green" },
      { icon: "Server", title: "本机服务", value: live.value.backendHealthy ? "正常" : "异常", meta: live.value.listen, tone: live.value.backendHealthy ? "green" : "amber" },
      { icon: "Watch", title: "ADB", value: state.installerState?.selectedSerial ? "已连接" : "未连接", meta: installerStatusNote(), tone: state.installerState?.selectedSerial ? "green" : "amber" },
      { icon: "PackageCheck", title: "手表 App", value: state.installerState?.apk?.available ? "可安装" : "未检测", meta: state.installerState?.apk?.versionName || "待下载", tone: state.installerState?.apk?.available ? "blue" : "amber" },
      { icon: "Globe2", title: "网络访问", value: currentAccessModeLabel(), meta: `${currentSelectedIP(state.snapshot)} 可访问`, tone: "blue" }
    ]
  }

  function deviceHistoryRows() {
    const installer = state.installerState || fallbackInstallerState
    if (installerDevices().length > 0) {
      return installerDevices().map((device) => ({
        name: device.displayName || device.serial,
        model: device.model || device.product || "未检测",
        android: device.isEmulator ? "Wear OS Emulator" : "Android / Wear OS",
        lastSeen: device.serial,
        adbState: device.state === "device" ? "已连接" : device.state,
        compatibility: device.isWatch ? "完全兼容" : "待验证",
        badge: device.serial === installer.selectedSerial ? "当前" : ""
      }))
    }
    return sampleDevices
  }

  function currentWatchVersionLabel() {
    return state.installerState?.apk?.installedVersionName
      || state.installerState?.apk?.versionName
      || "未检测"
  }

  function currentWatchInstalledVersionLabel() {
    return state.installerState?.apk?.installedVersionName || "未检测"
  }

  function currentWatchPackageVersionLabel() {
    return state.installerState?.apk?.versionName || "未检测"
  }

  function notificationLevelTone(level = "info") {
    switch (level) {
      case "success":
        return "green"
      case "warning":
        return "amber"
      case "error":
        return "red"
      default:
        return "blue"
    }
  }

  function nextNotificationId() {
    state.notificationCounter += 1
    return `notice-${state.notificationCounter}`
  }

  function pushNotification({ title, detail = "", level = "info", source = "系统", dedupeKey = "" }) {
    const normalizedTitle = String(title || "").trim()
    const normalizedDetail = String(detail || "").trim()
    if (!normalizedTitle) {
      return
    }
    const now = new Date()
    const key = dedupeKey || `${source}|${normalizedTitle}|${normalizedDetail}`
    const existing = state.notifications.find((item) => item.key === key)
    if (existing) {
      existing.at = now.toISOString()
      existing.timeLabel = shortTimeLabel(now)
      existing.read = false
      existing.level = level
      existing.title = normalizedTitle
      existing.detail = normalizedDetail
      existing.source = source
    } else {
      state.notifications.unshift({
        id: nextNotificationId(),
        key,
        at: now.toISOString(),
        timeLabel: shortTimeLabel(now),
        title: normalizedTitle,
        detail: normalizedDetail,
        level,
        source,
        read: false
      })
      if (state.notifications.length > 60) {
        state.notifications = state.notifications.slice(0, 60)
      }
    }
  }

  function showNotice(text) {
    pushNotification({
      title: "提示",
      detail: String(text || "").trim(),
      level: "info",
      source: "系统"
    })
  }

  function markNotificationsRead() {
    for (const item of state.notifications) {
      item.read = true
    }
  }

  function toggleNotificationPanel() {
    state.notificationPanelOpen = !state.notificationPanelOpen
    if (state.notificationPanelOpen) {
      markNotificationsRead()
    }
  }

  function closeNotificationPanel() {
    state.notificationPanelOpen = false
  }

  function activateCopyFeedback(key) {
    if (!key) {
      return
    }
    state.copyFeedbackKey = key
    if (copyFeedbackTimer) {
      window.clearTimeout(copyFeedbackTimer)
    }
    copyFeedbackTimer = window.setTimeout(() => {
      if (state.copyFeedbackKey === key) {
        state.copyFeedbackKey = ""
      }
      copyFeedbackTimer = null
    }, 1200)
  }

  function copyFeedbackActive(key) {
    return Boolean(key) && state.copyFeedbackKey === key
  }

  async function copyText(text, feedbackKey = "") {
    try {
      await copyToClipboard(text)
      state.copiedText = text
      activateCopyFeedback(feedbackKey)
    } catch {
      showNotice("复制失败，请手动复制。")
    }
  }

  function persistDeveloperFormState() {
    try {
      window.localStorage.setItem(DEV_ENV_STORAGE_KEY, JSON.stringify(state.developerForm))
    } catch {
      // ignore
    }
  }

  function loadDeveloperFormState() {
    try {
      const raw = window.localStorage.getItem(DEV_ENV_STORAGE_KEY)
      if (!raw) {
        return null
      }
      const parsed = JSON.parse(raw)
      return {
        enabled: Boolean(parsed.enabled),
        mode: String(parsed.mode || "workspace").trim(),
        repoPath: String(parsed.repoPath || "").trim(),
        hostAlias: String(parsed.hostAlias || "").trim(),
        devBaseUrl: String(parsed.devBaseUrl || "").trim(),
        deviceName: String(parsed.deviceName || "").trim(),
        managedTunnelEnabled: Boolean(parsed.managedTunnelEnabled),
        tunnelCode: String(parsed.tunnelCode || "").trim(),
        accessMode: ["emulator", "lan", "tunnel", "custom"].includes(parsed.accessMode) ? parsed.accessMode : ""
      }
    } catch {
      return null
    }
  }

  function persistInstallNetworkState() {
    try {
      window.localStorage.setItem(INSTALL_NETWORK_STORAGE_KEY, JSON.stringify({
        networkMode: state.networkMode,
        networkForm: {
          customUrl: state.networkForm.customUrl,
          port: state.networkForm.port
        },
        configEntries: state.wizard.configEntries
      }))
    } catch {
      // ignore
    }
  }

  function loadInstallNetworkState() {
    try {
      const raw = window.localStorage.getItem(INSTALL_NETWORK_STORAGE_KEY)
      if (!raw) {
        return null
      }
      const parsed = JSON.parse(raw)
      return {
        networkMode: String(parsed.networkMode || "").trim(),
        networkForm: {
          customUrl: String(parsed.networkForm?.customUrl || "").trim(),
          port: String(parsed.networkForm?.port || "").trim()
        },
        configEntries: parsed.configEntries || {}
      }
    } catch {
      return null
    }
  }

  function normalizeTheme(value) {
    return value === "light" ? "light" : "dark"
  }

  function loadThemeState() {
    try {
      return normalizeTheme(window.localStorage.getItem(THEME_STORAGE_KEY))
    } catch {
      return "dark"
    }
  }

  function persistThemeState() {
    try {
      window.localStorage.setItem(THEME_STORAGE_KEY, state.theme)
    } catch {
      // ignore
    }
  }

  function setTheme(value) {
    state.theme = normalizeTheme(value)
    persistThemeState()
  }

  function toggleTheme() {
    setTheme(state.theme === "dark" ? "light" : "dark")
  }

  function normalizeSettingsPreferences(value) {
    const defaults = state.settingsPreferences || {}
    if (!value || typeof value !== "object") {
      return { ...defaults }
    }
    return Object.fromEntries(
      Object.keys(defaults).map((key) => [key, Object.prototype.hasOwnProperty.call(value, key) ? Boolean(value[key]) : defaults[key]])
    )
  }

  function loadSettingsPreferencesState() {
    try {
      const raw = window.localStorage.getItem(SETTINGS_PREFERENCES_STORAGE_KEY)
      return raw ? normalizeSettingsPreferences(JSON.parse(raw)) : normalizeSettingsPreferences(null)
    } catch {
      return normalizeSettingsPreferences(null)
    }
  }

  function persistSettingsPreferencesState() {
    try {
      window.localStorage.setItem(SETTINGS_PREFERENCES_STORAGE_KEY, JSON.stringify(state.settingsPreferences))
    } catch {
      // ignore
    }
  }

  function applyDesktopSettings(desktopSettings) {
    const nextEnabled = desktopSettings?.autoStartBackend !== false
    state.settingsPreferences.autoStartBackend = nextEnabled
    persistSettingsPreferencesState()
    return nextEnabled
  }

  function applyDeveloperDesktopSettings(desktopSettings) {
    const developer = desktopSettings?.developerEnvironment
    if (!developer) {
      return
    }
    const next = {
      enabled: Boolean(developer.enabled),
      managedTunnelEnabled: Boolean(developer.managedTunnelEnabled)
    }
    if (developer.mode) {
      next.mode = String(developer.mode).trim()
    }
    if (developer.repoPath) {
      next.repoPath = String(developer.repoPath).trim()
    }
    if (developer.hostAlias) {
      next.hostAlias = String(developer.hostAlias).trim()
    }
    const baseURL = String(developer.baseUrl || developer.baseURL || "").trim()
    if (baseURL) {
      next.devBaseUrl = trimTrailingSlash(baseURL)
    }
    if (developer.deviceName) {
      next.deviceName = String(developer.deviceName).trim()
    }
    state.developerForm = {
      ...state.developerForm,
      ...next
    }
    state.developerForm.accessMode = developerAccessModeFromBaseUrl(state.developerForm.devBaseUrl)
  }

  async function loadDesktopSettings() {
    try {
      return await invoke("GetDesktopSettings")
    } catch {
      return {
        autoStartBackend: state.settingsPreferences.autoStartBackend,
        developerEnvironment: {
          enabled: Boolean(state.developerForm.enabled),
          mode: state.developerForm.mode,
          repoPath: state.developerForm.repoPath,
          baseUrl: state.developerForm.devBaseUrl,
          deviceName: state.developerForm.deviceName,
          hostAlias: state.developerForm.hostAlias,
          managedTunnelEnabled: Boolean(state.developerForm.managedTunnelEnabled)
        }
      }
    }
  }

  async function setAutoStartBackendPreference(enabled) {
    const previous = state.settingsPreferences.autoStartBackend
    state.settingsPreferences.autoStartBackend = Boolean(enabled)
    persistSettingsPreferencesState()
    try {
      const saved = await invoke("SetAutoStartBackend", Boolean(enabled))
      const nextEnabled = applyDesktopSettings(saved)
      if (nextEnabled) {
        if (state.snapshot?.backend?.running) {
          showNotice("已开启“启动后自动启动本机服务”。")
          return nextEnabled
        }
        const result = await startBackendAction({ notifySuccess: false, notifyFailure: false })
        if (result?.running) {
          showNotice("已开启“启动后自动启动本机服务”，并已启动本机服务。")
        } else {
          showNotice("已开启“启动后自动启动本机服务”。当前未能启动本机服务。")
        }
        return nextEnabled
      }
      showNotice(previous && state.snapshot?.backend?.running
        ? "已关闭“启动后自动启动本机服务”。当前已运行的服务会继续保持。"
        : "已关闭“启动后自动启动本机服务”。")
      return nextEnabled
    } catch (error) {
      state.settingsPreferences.autoStartBackend = previous
      persistSettingsPreferencesState()
      showNotice(`保存“启动后自动启动本机服务”失败：${String(error)}`)
      return previous
    }
  }

  async function toggleSettingsPreference(key) {
    if (!Object.prototype.hasOwnProperty.call(state.settingsPreferences, key)) {
      return
    }
    if (key === "autoStartBackend") {
      return setAutoStartBackendPreference(!state.settingsPreferences.autoStartBackend)
    }
    state.settingsPreferences[key] = !state.settingsPreferences[key]
    persistSettingsPreferencesState()
  }

  async function loadSnapshot() {
    try {
      return await invoke("GetSnapshot")
    } catch (error) {
      return {
        ...fallbackSnapshot,
        backend: {
          ...fallbackSnapshot.backend,
          friendlyError: `读取桌面状态失败：${String(error)}`
        }
      }
    }
  }

  async function loadBackendLogs() {
    try {
      return await invoke("GetBackendLogs")
    } catch {
      return []
    }
  }

  async function loadInstallerStatus() {
    try {
      return await invoke("GetInstallerStatus")
    } catch {
      return fallbackInstallerState
    }
  }

  async function loadCodexHookStatus() {
    try {
      return await invoke("GetCodexHookStatus")
    } catch (error) {
      return {
        ...state.codexHookState,
        installed: false,
        changed: false,
        message: `读取 Codex hooks 状态失败：${String(error)}`
      }
    }
  }

  function hydrateStateFromSnapshot(snapshot) {
    const networkOptions = availableNetworkOptions(snapshot)
    const selectedExists = networkOptions.some((item) => item.ip === state.networkForm.selectedIp)
    const usingFallbackIP = state.networkForm.selectedIp === fallbackSnapshot.networkContext.recommendedIp
    if ((!selectedExists || usingFallbackIP) && networkOptions.length > 0) {
      const preferred = networkOptions.find((item) => item.recommended) || networkOptions[0]
      state.networkForm.selectedIp = preferred.ip
      state.networkForm.selectedInterface = preferred.label
    } else {
      const selected = networkOptions.find((item) => item.ip === state.networkForm.selectedIp)
      if (selected) {
        state.networkForm.selectedInterface = selected.label
      }
    }
    if (!state.developerForm.devBaseUrl) {
      state.developerForm.devBaseUrl = suggestedDevBaseUrl()
    }
    state.developerForm.hostAlias = currentDeveloperHostAlias()
    state.developerForm.accessMode = developerAccessModeFromBaseUrl(state.developerForm.devBaseUrl)
    if (state.developerForm.accessMode !== "custom") {
      updateDeveloperBaseUrlFromAccessMode(state.developerForm.accessMode)
    }
    if (!state.developerForm.deviceName) {
      state.developerForm.deviceName = state.watchForm.deviceName
    }
  }

  function hydrateDeveloperStateFromSnapshot(snapshot) {
    const repositories = Array.isArray(snapshot?.repositories) ? snapshot.repositories : []
    const currentStatus = snapshot?.status || null
    const preferredRepo = repositories.find((item) => item.autoDetected && item.valid) || repositories.find((item) => item.valid) || repositories[0]
    if (currentStatus?.resolvedRepoPath) {
      state.developerForm.repoPath = currentStatus.resolvedRepoPath
    } else if (!state.developerForm.repoPath && preferredRepo?.path) {
      state.developerForm.repoPath = preferredRepo.path
    }
    state.developerForm.hostAlias = currentDeveloperHostAlias() || "10.0.2.2"
    if (currentStatus?.baseURL) {
      state.developerForm.devBaseUrl = trimTrailingSlash(currentStatus.baseURL)
    } else if (!state.developerForm.devBaseUrl) {
      state.developerForm.devBaseUrl = trimTrailingSlash(developerSuggestedBaseUrl(state.developerForm.accessMode))
    }
    state.developerForm.accessMode = developerAccessModeFromBaseUrl(state.developerForm.devBaseUrl)
    if (state.developerForm.accessMode !== "custom") {
      updateDeveloperBaseUrlFromAccessMode(state.developerForm.accessMode)
    }
    state.developerForm.enabled = developerStatusPhase() === "running" || developerStatusPhase() === "starting"
  }

  async function loadDeveloperEnvironmentSnapshot({ ensure = false } = {}) {
    try {
      const method = ensure ? "EnsureDeveloperEnvironment" : "GetDeveloperEnvironmentSnapshot"
      const snapshot = await invoke(method, developerEnvironmentRequest())
      state.developerSnapshot = snapshot
      state.developerRepositories = Array.isArray(snapshot?.repositories) ? snapshot.repositories : []
      hydrateDeveloperStateFromSnapshot(snapshot)
      return snapshot
    } catch (error) {
      pushNotification({
        title: "开发环境状态读取失败",
        detail: String(error),
        level: "error",
        source: "开发环境"
      })
      return null
    }
  }

  function mergeSnapshot(nextSnapshot, backendStatus) {
    return {
      ...nextSnapshot,
      backend: {
        ...(nextSnapshot.backend || {}),
        ...(backendStatus || {})
      }
    }
  }

  async function refreshInstallerState(options = {}) {
    if (state.installerRefreshRunning) {
      return
    }
    state.installerRefreshRunning = true
    try {
      state.installerState = await loadInstallerStatus()
      syncWizardWithInstallerState()
      hydrateStateFromSnapshot(state.snapshot)
      if (options.notifySuccess) {
        showNotice("ADB 设备状态已刷新。")
      }
    } finally {
      state.installerRefreshRunning = false
    }
  }

  async function refreshState() {
    state.snapshot = await loadSnapshot()
    hydrateStateFromSnapshot(state.snapshot)
    state.backendLogs = await loadBackendLogs()
    state.installerState = await loadInstallerStatus()
    state.codexHookState = await loadCodexHookStatus()
    await loadDeveloperEnvironmentSnapshot({ ensure: developerIsRunning() })
    syncWizardWithInstallerState()
    maybeShowTunnelExpiryNotice()
  }

  async function startBackendAction(options = {}) {
    const notifySuccess = options.notifySuccess !== false
    const notifyFailure = options.notifyFailure !== false
    const requestOverride = options.requestOverride || null
    try {
      const backendStatus = await invoke("StartBackendWithRequest", requestOverride || currentBackendRequest())
      state.snapshot.backend = backendStatus
      const refreshed = await loadSnapshot()
      state.snapshot = mergeSnapshot(refreshed, state.snapshot.backend)
      hydrateStateFromSnapshot(state.snapshot)
      syncInstallWizardDraft(state.snapshot)
      state.backendLogs = await loadBackendLogs()
      maybeShowTunnelExpiryNotice()
      if (backendStatus?.running) {
        if (notifySuccess) {
          showNotice("已触发本机服务启动。")
        }
        return backendStatus
      }
      if (notifyFailure) {
        showNotice(`启动本机服务失败：${backendStatus?.friendlyError || backendStatus?.message || "服务未启动"}`)
      }
      return backendStatus
    } catch (error) {
      if (notifyFailure) {
        showNotice(`启动本机服务失败：${String(error)}`)
      }
      return null
    }
  }

  async function restartBackendAction() {
    try {
      state.snapshot.backend = await invoke("RestartBackendWithRequest", currentBackendRequest())
      const refreshed = await loadSnapshot()
      state.snapshot = mergeSnapshot(refreshed, state.snapshot.backend)
      hydrateStateFromSnapshot(state.snapshot)
      syncInstallWizardDraft(state.snapshot)
      state.backendLogs = await loadBackendLogs()
      maybeShowTunnelExpiryNotice()
      showNotice("本机服务已重启。")
    } catch (error) {
      showNotice(`重启本机服务失败：${String(error)}`)
    }
  }

  async function runHealthCheckAction() {
    if (state.backendHealthCheckRunning) {
      showNotice("正在执行健康检查，请稍候。")
      return
    }
    state.backendHealthCheckRunning = true
    try {
      const result = await invoke("CheckHealth")
      state.healthCheckResult = {
        ok: Boolean(result?.ok),
        result: result?.message || "已完成"
      }
      const refreshed = await loadSnapshot()
      state.snapshot = mergeSnapshot(refreshed, backendStatusWithHealthResult(refreshed.backend, result))
      hydrateStateFromSnapshot(state.snapshot)
      syncInstallWizardDraft(state.snapshot)
      state.backendLogs = await loadBackendLogs()
      state.globalHealthSummary = {
        ...state.globalHealthSummary,
        checking: false,
        lastCheckedAt: new Date().toISOString(),
        targets: {
          ...(state.globalHealthSummary.targets || {}),
          backend: backendTargetFromHealthResult(result, live.value.backendStatusNote)
        }
      }
      maybeShowTunnelExpiryNotice()
      showNotice(result?.ok ? "健康检查通过。" : `健康检查完成：${result?.message || "服务不可达"}`)
    } catch (error) {
      const result = {
        ok: false,
        message: `健康检查失败：${String(error)}`
      }
      state.healthCheckResult = {
        ok: false,
        result: "检查失败"
      }
      state.snapshot = mergeSnapshot(state.snapshot, backendStatusWithHealthResult(state.snapshot.backend, result))
      state.globalHealthSummary = {
        ...state.globalHealthSummary,
        checking: false,
        lastCheckedAt: new Date().toISOString(),
        targets: {
          ...(state.globalHealthSummary.targets || {}),
          backend: backendTargetFromHealthResult(result)
        }
      }
      showNotice(result.message)
    } finally {
      state.backendHealthCheckRunning = false
    }
  }

  function backendStatusWithHealthResult(backend, health) {
    const next = { ...(backend || {}) }
    if (!health) {
      return next
    }
    next.lastHealth = health
    if (!health.ok) {
      const message = health.message || "本机服务未通过 /healthz"
      next.message = message
      next.friendlyError = message
      next.state = "error"
    } else if (next.running && next.state === "error") {
      next.state = "running"
      next.message = "OpenWatcher 本机服务已启动。"
      next.friendlyError = ""
    }
    return next
  }

  function backendTargetFromHealthResult(health, fallbackDetail = "本机服务未就绪") {
    return {
      ok: Boolean(health?.ok),
      detail: health?.message || fallbackDetail,
      source: "本机服务"
    }
  }

  async function ensureRuntimeDependenciesIfNeeded({ manual = false } = {}) {
    const missingInstaller = !state.installerState?.adb?.available || !state.installerState?.apk?.available
    const missingTunnel = !state.snapshot?.tunnel?.resolvedBinary
    if (!manual && !missingInstaller && !missingTunnel) {
      return
    }
    state.snapshot = await invoke("EnsureRuntimeDependencies")
    state.installerState = await loadInstallerStatus()
    syncWizardWithInstallerState()
  }

  function buildGlobalHealthTargets(currentLive, developerSnapshot, auxiliaryChecks = {}) {
    const installerHealthy = Boolean(state.installerState?.adb?.available && state.installerState?.apk?.available)
    const currentDeveloperStatus = developerSnapshot?.status || null
    const developerTunnel = developerSnapshot?.tunnel || null
    const developerHealthy = currentDeveloperStatus
      ? Boolean(currentDeveloperStatus.running && currentDeveloperStatus.lastHealth?.ok)
      : null
    const developerTunnelHealthy = developerTunnelIsManaged()
      ? Boolean(developerTunnel?.running && developerTunnel?.lastHealth?.ok)
      : null
    const includeDeveloper = developerStatusVisible()
    const includeDeveloperTunnel = developerTunnelStatusVisible()
    return {
      codex: { ok: Boolean(currentLive.codexHealthy), detail: currentLive.codexStatusNote || "Codex 未就绪", source: "Codex" },
      backend: { ok: Boolean(currentLive.backendHealthy), detail: currentLive.backendStatusNote || "本机服务未就绪", source: "本机服务" },
      resources: { ok: installerHealthy, detail: installerHealthy ? "运行时依赖齐全" : "存在缺失的运行时依赖", source: "安装资源" },
      developer: !includeDeveloper || developerHealthy == null ? null : {
        ok: developerHealthy,
        detail: currentDeveloperStatus?.message || "开发环境未就绪",
        source: "开发环境"
      },
      developerTunnel: !includeDeveloperTunnel || developerTunnelHealthy == null ? null : {
        ok: developerTunnelHealthy,
        detail: developerTunnel?.message || "开发隧道未就绪",
        source: "开发隧道"
      },
      publicEntry: auxiliaryChecks.publicEntry || null,
      tunnelEntry: auxiliaryChecks.tunnelEntry || null
    }
  }

  function applyHealthNotifications(nextTargets) {
    const previousTargets = state.globalHealthSummary.targets || {}
    for (const [key, next] of Object.entries(nextTargets)) {
      if (!next) {
        continue
      }
      const previous = previousTargets[key]
      if (!previous && !next.ok) {
        pushNotification({
          title: `${next.source}状态异常`,
          detail: next.detail,
          level: "warning",
          source: next.source,
          dedupeKey: `health:${key}:initial:${next.detail}`
        })
        continue
      }
      if (previous?.ok && !next.ok) {
        pushNotification({
          title: `${next.source}状态异常`,
          detail: next.detail,
          level: "warning",
          source: next.source,
          dedupeKey: `health:${key}:down:${next.detail}`
        })
        continue
      }
      if (previous && !previous.ok && next.ok) {
        pushNotification({
          title: `${next.source}已恢复`,
          detail: next.detail,
          level: "success",
          source: next.source,
          dedupeKey: `health:${key}:recovered:${next.detail}`
        })
        continue
      }
      if (previous && !next.ok && previous.detail !== next.detail) {
        pushNotification({
          title: `${next.source}状态异常`,
          detail: next.detail,
          level: "warning",
          source: next.source,
          dedupeKey: `health:${key}:changed:${next.detail}`
        })
      }
    }
  }

  async function runGlobalHealthCheck({ manual = false } = {}) {
    if (state.globalHealthRunning) {
      if (manual) {
        showNotice("正在执行健康检查，请稍候。")
      }
      return
    }
    state.globalHealthRunning = true
    state.globalHealthSummary.checking = true
    try {
      state.snapshot = await loadSnapshot()
      state.installerState = await loadInstallerStatus()
      await ensureRuntimeDependenciesIfNeeded({ manual })
      state.snapshot = await loadSnapshot()
      state.installerState = await loadInstallerStatus()

      if (!state.snapshot?.backend?.running) {
        await startBackendAction({ notifySuccess: false, notifyFailure: false })
        state.snapshot = await loadSnapshot()
      } else if (!state.snapshot?.backend?.lastHealth?.ok) {
        state.snapshot.backend = await invoke("RestartBackendWithRequest", currentBackendRequest())
        const refreshed = await loadSnapshot()
        state.snapshot = mergeSnapshot(refreshed, state.snapshot.backend)
      }

      const developerSnapshot = await loadDeveloperEnvironmentSnapshot({ ensure: developerIsRunning() })
      const auxiliaryChecks = {}
      if (wizardConfigEntry("public")?.enabled && wizardConfigEntry("public").url.trim()) {
        const result = await invoke("CheckHealthWithRequest", buildBackendRequestForEntry("public"))
        auxiliaryChecks.publicEntry = {
          ok: Boolean(result?.ok),
          detail: result?.message || "公网入口未通过检查",
          source: "公网入口"
        }
      }
      if (wizardConfigEntry("tunnel")?.enabled && wizardConfigEntry("tunnel").redeemedDomain.trim()) {
        const result = await invoke("CheckHealthWithRequest", buildBackendRequestForEntry("tunnel"))
        auxiliaryChecks.tunnelEntry = {
          ok: Boolean(result?.ok),
          detail: result?.message || "托管隧道未通过检查",
          source: "托管隧道"
        }
      }

      const nextTargets = buildGlobalHealthTargets(live.value, developerSnapshot, auxiliaryChecks)
      applyHealthNotifications(nextTargets)
      state.globalHealthSummary = {
        checking: false,
        lastCheckedAt: new Date().toISOString(),
        targets: nextTargets
      }
      if (manual) {
        showNotice("已完成健康检查。")
      }
    } catch (error) {
      state.globalHealthSummary.checking = false
      pushNotification({
        title: "全局健康检查失败",
        detail: String(error),
        level: "error",
        source: "系统"
      })
      if (manual) {
        showNotice(`健康检查失败：${String(error)}`)
      }
    } finally {
      state.globalHealthRunning = false
      state.globalHealthSummary.checking = false
    }
  }

  function ensureGlobalHealthTicker() {
    if (state.globalHealthTicker) {
      return
    }
    state.globalHealthTicker = window.setInterval(() => {
      void runGlobalHealthCheck()
    }, GLOBAL_HEALTH_INTERVAL_MS)
  }

  function maybeShowTunnelExpiryNotice() {
    const tunnel = currentTunnel()
    if (!tunnel.tokenExpired) {
      state.tunnelExpiryNoticeKey = ""
      return
    }
    const noticeKey = `${tunnel.tunnelId}:${tunnel.tokenVersion}:${tunnel.message}`
    if (state.tunnelExpiryNoticeKey === noticeKey) {
      return
    }
    state.tunnelExpiryNoticeKey = noticeKey
    showNotice("远程访问隧道已失效，请联系管理员重新绑定设备。")
  }

  async function maybeAutoStartBackend() {
    if (state.backendAutoStartAttempted) {
      return
    }
    state.backendAutoStartAttempted = true
    if (state.snapshot?.backend?.running) {
      return
    }
    try {
      state.snapshot.backend = await invoke("StartBackend")
      const refreshed = await loadSnapshot()
      state.snapshot = mergeSnapshot(refreshed, state.snapshot.backend)
      hydrateStateFromSnapshot(state.snapshot)
      syncInstallWizardDraft(state.snapshot)
      state.backendLogs = await loadBackendLogs()
    } catch {
      // 启动失败时保留当前快照，由显式操作再给出错误提示。
    }
  }

  async function copyDiagnosticsAction(feedbackKey = "") {
    try {
      const payload = await invoke("CopyDiagnostics")
      await copyToClipboard(payload)
      state.copiedText = payload
      activateCopyFeedback(feedbackKey)
    } catch (error) {
      showNotice(`复制诊断信息失败：${String(error)}`)
    }
  }

  async function checkForUpdatesAction() {
    if (state.updateCheckRunning) {
      showNotice("正在检查更新，请稍候。")
      return
    }
    state.updateCheckRunning = true
    try {
      const currentWatchVersion = currentWatchVersionLabel()
      const result = await invoke("CheckForUpdates", currentWatchVersion === "未检测" ? "" : currentWatchVersion)
      state.updateCheckResult = result
      if (result?.desktopUpdateAvailable || result?.watchUpdateAvailable) {
        const targets = []
        if (result.desktopUpdateAvailable && result.latestDesktopVersion) {
          targets.push(`Desktop ${result.latestDesktopVersion}`)
        }
        if (result.watchUpdateAvailable && result.latestWatchVersion) {
          targets.push(`手表 ${result.latestWatchVersion}`)
        }
        showNotice(`发现可用更新：${targets.join("，") || "请查看更新页详情。"}。`)
      } else {
        showNotice("已检查更新，当前已是最新版本。")
      }
    } catch (error) {
      state.updateCheckResult = {
        checkedAt: new Date().toISOString(),
        error: String(error)
      }
      showNotice(`检查更新失败：${String(error)}`)
    } finally {
      state.updateCheckRunning = false
    }
  }

  async function loadDesktopUpdateStatus() {
    try {
      const status = await invoke("GetDesktopUpdateStatus")
      state.desktopUpdateStatus = status && status.phase ? status : null
    } catch {
      state.desktopUpdateStatus = null
    }
  }

  function handleDesktopUpdateProgress(progress) {
    state.desktopUpdateProgress = progress || null
    if (progress?.message) {
      state.desktopUpdateStatus = {
        phase: progress.phase || "downloading",
        message: progress.message,
        updatedAt: new Date().toISOString()
      }
    }
  }

  function ensureDesktopUpdateProgressListener() {
    if (desktopUpdateProgressOff) {
      return
    }
    desktopUpdateProgressOff = onRuntimeEvent("desktop-update-progress", handleDesktopUpdateProgress)
  }

  function ensureDesktopStateChangedListener() {
    if (desktopStateChangedOff) {
      return
    }
    desktopStateChangedOff = onRuntimeEvent("desktop-state-changed", async () => {
      const desktopSettings = await loadDesktopSettings()
      applyDesktopSettings(desktopSettings)
      applyDeveloperDesktopSettings(desktopSettings)
      await refreshState()
    })
  }

  async function installDesktopUpdateAction() {
    if (state.desktopUpdateRunning) {
      showNotice("正在下载 Desktop 更新，请稍候。")
      return
    }
    const result = state.updateCheckResult
    if (!result?.desktopUpdateAvailable) {
      showNotice("当前没有可安装的 Desktop 更新。")
      return
    }
    if (!result.desktopInstallable) {
      showNotice(result.desktopInstallMessage || "当前更新包不能自动安装。")
      return
    }
    state.desktopUpdateRunning = true
    state.desktopUpdateProgress = {
      phase: "starting",
      message: "正在准备 Desktop 更新",
      percent: 0,
      totalBytes: result.desktopSizeBytes || 0
    }
    try {
      const installResult = await invoke("InstallDesktopUpdate")
      state.desktopUpdateStatus = {
        phase: installResult.phase || "restarting",
        message: installResult.message || "更新程序已启动，Desktop 将自动重启",
        version: installResult.version || result.latestDesktopVersion || "",
        artifact: installResult.artifact || result.desktopArtifact || "",
        updatedAt: new Date().toISOString()
      }
      showNotice("更新程序已启动，Desktop 将自动重启。")
    } catch (error) {
      state.desktopUpdateStatus = {
        phase: "failed",
        message: String(error),
        version: result.latestDesktopVersion || "",
        artifact: result.desktopArtifact || "",
        updatedAt: new Date().toISOString()
      }
      showNotice(`安装 Desktop 更新失败：${String(error)}`)
    } finally {
      state.desktopUpdateRunning = false
    }
  }

  function desktopUpdateProgressLabel() {
    const progress = state.desktopUpdateProgress
    if (!progress) {
      return ""
    }
    const sizeLabel = progress.totalBytes && progress.downloadedBytes
      ? `${formatByteSize(progress.downloadedBytes)} / ${formatByteSize(progress.totalBytes)}`
      : ""
    return [progress.message, sizeLabel].filter(Boolean).join(" · ")
  }

  function wait(ms) {
    return new Promise((resolve) => window.setTimeout(resolve, ms))
  }

  async function toggleDeveloperEnvironmentAction() {
    if (state.developerAction.busy) {
      return
    }
    const nextEnabled = !developerIsRunning()
    state.developerAction = {
      busy: true,
      targetEnabled: nextEnabled,
      label: nextEnabled ? "正在执行启动脚本，请稍候。" : "正在停止当前开发环境。"
    }
    try {
      if (nextEnabled) {
        let snapshot = await invoke("EnsureDeveloperEnvironment", developerEnvironmentRequest({ enabled: true }))
        state.developerSnapshot = snapshot
        state.developerRepositories = Array.isArray(snapshot?.repositories) ? snapshot.repositories : []
        hydrateDeveloperStateFromSnapshot(snapshot)
        const deadlineAt = Date.now() + DEVELOPER_STARTUP_WAIT_MS
        while (!snapshot?.status?.lastHealth?.ok && Date.now() < deadlineAt) {
          await wait(800)
          snapshot = await invoke("GetDeveloperEnvironmentSnapshot", developerEnvironmentRequest({ enabled: true }))
          state.developerSnapshot = snapshot
          state.developerRepositories = Array.isArray(snapshot?.repositories) ? snapshot.repositories : []
          hydrateDeveloperStateFromSnapshot(snapshot)
        }
        const healthy = Boolean(snapshot?.status?.lastHealth?.ok)
        if (!healthy && snapshot?.status?.running) {
          persistDeveloperFormState()
          pushNotification({
            title: "开发环境仍在启动",
            detail: developerRecentLogMessage() || snapshot?.status?.message || "进程仍在运行，可先查看开发环境日志。",
            level: "warning",
            source: "开发环境"
          })
        } else if (!healthy) {
          persistDeveloperFormState()
          pushNotification({
            title: "开发环境启动失败",
            detail: developerRecentLogMessage() || snapshot?.status?.message || snapshot?.status?.lastHealth?.message || "尚未通过 /healthz",
            level: "error",
            source: "开发环境"
          })
        } else {
          persistDeveloperFormState()
          pushNotification({
            title: "开发环境已启动",
            detail: snapshot?.status?.externallyManaged ? "已接入当前正在运行的本机开发服务。" : (snapshot?.status?.message || "开发环境已经通过健康检查。"),
            level: "success",
            source: "开发环境"
          })
        }
      } else {
        state.developerSnapshot = await invoke("StopDeveloperEnvironment")
        persistDeveloperFormState()
        pushNotification({
          title: "开发环境已停止",
          detail: "当前开发环境已停止。",
          level: "info",
          source: "开发环境"
        })
      }
    } catch (error) {
      if (nextEnabled) {
        persistDeveloperFormState()
      }
      pushNotification({
        title: nextEnabled ? "开发环境启动失败" : "开发环境停止失败",
        detail: String(error),
        level: "error",
        source: "开发环境"
      })
    } finally {
      state.developerAction = {
        busy: false,
        targetEnabled: developerIsRunning(),
        label: ""
      }
    }
  }

  async function restartDeveloperEnvironmentAction() {
    if (state.developerAction.busy) {
      return
    }
    if (developerIsRunning()) {
      state.developerSnapshot = await invoke("StopDeveloperEnvironment")
      await wait(250)
    }
    await toggleDeveloperEnvironmentAction()
  }

  async function clearDeveloperLogsAction() {
    state.developerSnapshot = await invoke("ClearDeveloperEnvironmentLogs")
    showNotice("开发环境日志已清空。")
  }

  async function selectDeveloperRepoDirAction() {
    try {
      const selectedPath = await invoke("ChooseDeveloperRepositoryDir", state.developerForm.repoPath.trim())
      if (!selectedPath) {
        return
      }
      state.developerForm.repoPath = selectedPath
      if (state.developerForm.accessMode !== "custom") {
        updateDeveloperBaseUrlFromAccessMode(state.developerForm.accessMode)
      }
      persistDeveloperFormState()
      await loadDeveloperEnvironmentSnapshot({ ensure: false })
    } catch (error) {
      showNotice(`选择仓库目录失败：${String(error)}`)
    }
  }

  async function openFolderAction(method, successNotice, arg = undefined) {
    try {
      const errorMessage = arg === undefined ? await invoke(method) : await invoke(method, arg)
      if (errorMessage) {
        showNotice(errorMessage)
        return
      }
      showNotice(successNotice)
    } catch (error) {
      showNotice(`打开失败：${String(error)}`)
    }
  }

  async function refreshCodexHookStatusAction(options = {}) {
    state.codexHookState = await loadCodexHookStatus()
    if (options.notifySuccess) {
      showNotice("已刷新 Codex hooks 状态。")
    }
  }

  async function installCodexHooksAction() {
    if (state.codexHooksInstalling) {
      return
    }
    state.codexHooksInstalling = true
    try {
      const status = await invoke("InstallCodexHooks")
      state.codexHookState = status
      showNotice(status?.message || "已写入 OpenWatcher hooks，请在 Codex App 中审核。")
    } catch (error) {
      state.codexHookState = await loadCodexHookStatus()
      showNotice(`安装 Codex hooks 失败：${String(error)}`)
    } finally {
      state.codexHooksInstalling = false
    }
  }

  async function prepareWatchBootstrapAction(feedbackKey = "") {
    try {
      const payload = await invoke("PrepareWatchBootstrap", currentBackendRequest())
      state.generatedBootstrap = payload
      await copyText(payload.bootstrapUri, feedbackKey)
      state.installerState = await loadInstallerStatus()
      syncWizardWithInstallerState()
      showNotice(`已复制 bootstrap URI，token 指纹 ${payload.tokenFingerprint}。`)
    } catch (error) {
      showNotice(`生成手表配置失败：${String(error)}`)
    }
  }

  async function prepareDevWatchBootstrapAction(feedbackKey = "") {
    try {
      const baseURL = developerBaseUrlForCurrentForm() || suggestedDevBaseUrl()
      state.developerForm.devBaseUrl = baseURL
      persistDeveloperFormState()
      const payload = await invoke("PrepareDevWatchBootstrap", {
        baseURL,
        deviceName: state.developerForm.deviceName.trim(),
        repoPath: state.developerForm.repoPath.trim(),
        hostAlias: currentDeveloperHostAlias(),
        managedTunnelEnabled: Boolean(state.developerForm.managedTunnelEnabled)
      })
      state.generatedBootstrap = payload
      await copyText(payload.bootstrapUri, feedbackKey)
      showNotice(`已复制 dev bootstrap URI，token 指纹 ${payload.tokenFingerprint}。`)
    } catch (error) {
      showNotice(`生成开发环境配置失败：${String(error)}`)
    }
  }

  async function applyWatchBootstrapAction(requestOverride = null, options = {}) {
    const request = requestOverride || currentBackendRequest()
    const successNotice = options.successNotice || "已发送配置链接，请在手表上确认。"
    const failureStage = options.failureStage || ""
    try {
      let health = await invoke("CheckHealthWithRequest", request)
      if (!health?.ok && request.mode !== "public") {
        await startBackendAction({ notifySuccess: false, notifyFailure: false, requestOverride: request })
        health = await invoke("CheckHealthWithRequest", request)
      }
      if (!health?.ok) {
        throw new Error(health?.message || "当前地址还没有通过检查。")
      }
      const result = await invoke("BootstrapWatchOnDevice", request, state.installerState?.selectedSerial || "")
      state.installerState = result?.installer || state.installerState
      state.generatedBootstrap = result?.payload || state.generatedBootstrap
      syncWizardWithInstallerState()
      if (state.installerState?.phase === "troubleshooting") {
        throw new Error(state.installerState?.message || "写入失败。")
      }
      clearWizardStageError()
      showNotice(state.installerState?.message || successNotice)
      return true
    } catch (error) {
      if (failureStage) {
        setWizardStageError(failureStage, String(error))
      }
      showNotice(`写入设置失败：${String(error)}`)
      return false
    }
  }

  async function applyDevWatchBootstrapAction() {
    try {
      const baseURL = developerBaseUrlForCurrentForm() || suggestedDevBaseUrl()
      state.developerForm.devBaseUrl = baseURL
      persistDeveloperFormState()
      if (!developerIsHealthy()) {
        showNotice("请先启动开发环境。")
        return
      }
      if (!selectedInstallerDevice()) {
        showNotice("未检测到目标设备。")
        return
      }
      const result = await invoke("BootstrapDevWatchOnDevice", {
        baseURL,
        deviceName: state.developerForm.deviceName.trim(),
        repoPath: state.developerForm.repoPath.trim(),
        hostAlias: currentDeveloperHostAlias()
      }, state.installerState?.selectedSerial || "")
      state.installerState = result?.installer || state.installerState
      state.generatedBootstrap = result?.payload || state.generatedBootstrap
      syncWizardWithInstallerState()
      showNotice(state.installerState?.message || "已发送开发环境到手表，请在手表确认。")
    } catch (error) {
      showNotice(`发送开发环境失败：${String(error)}`)
    }
  }

  async function submitRemoteWatchBootstrapAction() {
    const form = state.remoteBootstrapForm
    const bootstrapCode = form.bootstrapCode.trim().toUpperCase().replace(/[\s-]/g, "")
    if (!bootstrapCode) {
      showNotice("请先填写手表上的临时配置码。")
      return
    }
    const apiBase = form.apiBase.trim() || remoteBootstrapDefaultApiBase()
    if (!apiBase && !form.tunnelCode.trim()) {
      showNotice("请填写 API 基址，或填写隧道配置码。")
      return
    }

    form.submitting = true
    form.result = null
    try {
      const result = await invoke("SubmitRemoteWatchBootstrap", {
        bootstrapCode,
        environment: form.environment === "dev" ? "dev" : "beta",
        apiBase,
        tunnelCode: form.tunnelCode.trim()
      })
      state.remoteBootstrapForm = {
        ...state.remoteBootstrapForm,
        bootstrapCode: "",
        tunnelCode: "",
        apiBase: result?.apiBase || apiBase,
        environment: result?.environment || form.environment,
        submitting: false,
        result
      }
      if (result?.tunnelRedeemed) {
        if (result.environment === "dev") {
          await loadDeveloperEnvironmentSnapshot({ ensure: developerIsRunning() })
        } else {
          state.snapshot = await loadSnapshot()
          hydrateStateFromSnapshot(state.snapshot)
        }
      }
      showNotice(result?.message || "已提交临时配置，等待手表获取。")
    } catch (error) {
      form.submitting = false
      showNotice(`发送临时配置失败：${String(error)}`)
    }
  }

  async function installWatchAppAction() {
    try {
      state.installerState = await invoke("InstallWatchApp", state.installerState?.selectedSerial || "")
      syncWizardWithInstallerState()
      showNotice(state.installerState?.message || "安装动作已执行。")
    } catch (error) {
      showNotice(`安装失败：${String(error)}`)
    }
  }

  async function launchWatchAppAction() {
    try {
      state.installerState = await invoke("LaunchWatchApp", state.installerState?.selectedSerial || "")
      syncWizardWithInstallerState()
      showNotice(state.installerState?.message || "应用已启动。")
    } catch (error) {
      showNotice(`启动失败：${String(error)}`)
    }
  }

  async function selectInstallerDevice(serial) {
    state.installerState = await invoke("SelectInstallerDevice", serial)
    syncWizardWithInstallerState()
    hydrateStateFromSnapshot(state.snapshot)
  }

  async function runConnectStageAction() {
    if (!pairingSkipReason()) {
      if (!state.installForm.pairIp.trim() || !state.installForm.pairPort.trim() || !state.installForm.connectPort.trim() || !state.installForm.pairingCode.trim()) {
        setWizardStageError("connect", "请先填写手表 IP、配对端口、连接端口和配对码。")
        showNotice("请先补全手表连接信息。")
        return false
      }
    }

    try {
      clearWizardStageError()
      state.installerState = await invoke("RunADBPairing", {
        pairIP: state.installForm.pairIp.trim(),
        pairPort: state.installForm.pairPort.trim(),
        pairingCode: state.installForm.pairingCode.trim(),
        connectIP: (state.installForm.useSeparateConnectIP ? state.installForm.connectIp : state.installForm.pairIp).trim(),
        connectPort: state.installForm.connectPort.trim(),
        selectedSerial: state.installerState?.selectedSerial || ""
      })
      syncWizardWithInstallerState()
      if (state.installerState?.phase === "troubleshooting") {
        setWizardStageError("connect", state.installerState?.message || "无法连接到手表。")
        showNotice(state.installerState?.message || "无法连接到手表。")
        return false
      }
      if (installerSelectionRequired()) {
        state.wizard.stageNotes.connect = "检测到多个设备，请先确认这次要安装的目标手表。"
        showNotice("请先选择目标设备。")
        return false
      }
      if (!wizardConnectReady()) {
        setWizardStageError("connect", "当前还没有确认目标设备。")
        showNotice("当前还没有确认目标设备。")
        return false
      }
      markWizardStageCompleted("connect", state.installerState?.message || "手表已连接。")
      state.wizard.stageNotes.connect = state.installerState?.message || "手表已连接。"
      clearWizardStageError()
      showNotice(state.installerState?.message || "手表已连接。")
      return true
    } catch (error) {
      setWizardStageError("connect", String(error))
      showNotice(`连接手表失败：${String(error)}`)
      return false
    }
  }

  async function runInstallStageAction() {
    if (!selectedInstallerDevice()) {
      setWizardStageError("install", "请先回到上一页确认目标设备。")
      showNotice("请先确认目标设备。")
      return false
    }
    try {
      clearWizardStageError()
      if (!state.installerState?.apk?.installed) {
        state.installerState = await invoke("InstallWatchApp", state.installerState?.selectedSerial || "")
        syncWizardWithInstallerState()
        if (state.installerState?.phase === "troubleshooting") {
          setWizardStageError("install", state.installerState?.message || "安装失败。")
          showNotice(state.installerState?.message || "安装失败。")
          return false
        }
      }
      state.installerState = await invoke("LaunchWatchApp", state.installerState?.selectedSerial || "")
      syncWizardWithInstallerState()
      if (state.installerState?.phase === "troubleshooting") {
        setWizardStageError("install", state.installerState?.message || "应用启动失败。")
        showNotice(state.installerState?.message || "应用启动失败。")
        return false
      }
      markWizardStageCompleted("install", state.installerState?.message || "应用安装并启动完成。")
      state.wizard.stageNotes.install = state.installerState?.message || "应用已安装并启动完成。"
      clearWizardStageError()
      showNotice(state.installerState?.message || "应用已安装并启动。")
      return true
    } catch (error) {
      setWizardStageError("install", String(error))
      showNotice(`安装应用失败：${String(error)}`)
      return false
    }
  }

  async function validateConfigEntryAction(entryId, options = {}) {
    const { quiet = false } = options
    const entry = wizardConfigEntry(entryId)
    if (!entry) {
      return false
    }

    try {
      if (entryId === "lan" || entryId === "public") {
        const raw = entry.url.trim()
        if (!raw) {
          throw new Error(entryId === "lan" ? "请先填写局域网地址" : "请先填写公网地址")
        }
        const parsed = new URL(raw)
        if (!/^https?:$/.test(parsed.protocol)) {
          throw new Error("地址只支持 http 或 https")
        }
        if (entryId === "lan" && parsed.protocol !== "http:") {
          throw new Error("局域网地址请使用 http")
        }
        entry.validation = "checking"
        entry.message = `正在检查 ${entryId === "lan" ? "局域网" : "公网"} /healthz`
        const health = await invoke("CheckHealthWithRequest", buildBackendRequestForEntry(entryId))
        if (!health?.ok) {
          entry.validation = "error"
          entry.message = health?.message || "地址检查未通过"
          if (!quiet) {
            showNotice(entry.message)
          }
          return false
        }
        entry.validation = "valid"
        entry.message = entryId === "lan" ? "同一网络内可用" : "已通过访问检查"
        entry.lastCheckedURL = raw
        if (!quiet) {
          showNotice(entry.message)
        }
        return true
      }

      if (!entry.redeemedDomain) {
        const code = entry.code.trim()
        if (!code) {
          entry.validation = "error"
          entry.message = "请先输入配置码"
          if (!quiet) {
            showNotice(entry.message)
          }
          return false
        }
        entry.validation = "checking"
        entry.message = "正在兑换配置码"
        state.snapshot = await invoke("RedeemManagedTunnelCode", code)
        state.backendLogs = await loadBackendLogs()
        hydrateStateFromSnapshot(state.snapshot)
        syncInstallWizardDraft(state.snapshot)
        entry.validation = "idle"
        entry.message = "已兑换，可继续校验"
      }

      const health = await checkTunnelHealthWithRetry(entry)
      if (!health?.ok) {
        entry.validation = "error"
        entry.message = health?.message || "托管隧道检查未通过"
        if (!quiet) {
          showNotice(entry.message)
        }
        return false
      }
      entry.validation = "valid"
      entry.message = "已通过访问检查"
      entry.lastCheckedURL = entry.redeemedDomain
      if (!quiet) {
        showNotice("托管隧道已通过检查。")
      }
      return true
    } catch (error) {
      entry.validation = "error"
      entry.message = String(error)
      if (!quiet) {
        showNotice(`检查失败：${String(error)}`)
      }
      return false
    }
  }

  async function checkTunnelHealthWithRetry(entry, maxWaitMs = 20000) {
    const startedAt = Date.now()
    let lastHealth = null
    while (Date.now() - startedAt <= maxWaitMs) {
      const elapsedSeconds = Math.max(0, Math.round((Date.now() - startedAt) / 1000))
      entry.validation = "checking"
      entry.message = elapsedSeconds > 0
        ? `正在等待托管隧道连接（${elapsedSeconds}s）`
        : "正在启动托管隧道并检查 /healthz"
      lastHealth = await invoke("CheckHealthWithRequest", buildBackendRequestForEntry("tunnel"))
      if (lastHealth?.ok) {
        return lastHealth
      }
      if (Date.now() - startedAt >= maxWaitMs) {
        break
      }
      await wait(1500)
    }
    return lastHealth
  }

  function resolveConfigWriteTarget() {
    const priorities = ["lan", "public", "tunnel"]
    return priorities.find((entryId) => wizardConfigEntry(entryId)?.enabled) || ""
  }

  function syncLegacyNetworkFormWithEntry(entryId) {
    if (entryId === "lan") {
      const target = new URL(wizardConfigEntry("lan").url)
      state.networkMode = "lan"
      state.networkForm.selectedIp = target.hostname
      state.networkForm.selectedInterface = `手动填写 · ${target.hostname}`
      state.networkForm.port = target.port || DEFAULT_SIDECAR_PORT
      return
    }
    if (entryId === "public") {
      state.networkMode = "public"
      state.networkForm.customUrl = wizardConfigEntry("public").url.trim()
      return
    }
    state.networkMode = "tunnel"
  }

  async function runConfigStageAction() {
    if (wizardConfigChecksInProgress()) {
      showNotice(`${wizardConfigCheckingLabel()} 正在检查，完成后才能写入。`)
      return false
    }
    if (!selectedInstallerDevice()) {
      setWizardStageError("config", "请先回到上一页确认目标设备。")
      showNotice("请先确认目标设备。")
      return false
    }
    const enabledEntries = wizardEnabledConfigEntries()
    if (enabledEntries.length === 0) {
      setWizardStageError("config", "请至少启用一个要写入的地址。")
      showNotice("请至少启用一个要写入的地址。")
      return false
    }

    for (const [entryId] of enabledEntries) {
      const ok = await validateConfigEntryAction(entryId, { quiet: true })
      if (!ok) {
        setWizardStageError("config", `${installConfigMeta[entryId].label} 还没有通过检查。`)
        showNotice(`${installConfigMeta[entryId].label} 还没有通过检查。`)
        return false
      }
    }

    const writeTarget = resolveConfigWriteTarget()
    if (!writeTarget) {
      setWizardStageError("config", "当前没有可写入的地址。")
      showNotice("当前没有可写入的地址。")
      return false
    }

    const bootstrapEndpoints = buildEnabledBootstrapEndpoints()
    syncLegacyNetworkFormWithEntry(writeTarget)
    const labels = enabledEntries.map(([entryId]) => installConfigMeta[entryId].label)
    const ok = await applyWatchBootstrapAction(buildBackendRequestForEntry(writeTarget, { endpoints: bootstrapEndpoints }), {
      successNotice: `已发送 ${labels.join("、")} 配置链接，请在手表上确认。`,
      failureStage: "config"
    })
    if (!ok) {
      return false
    }
    markWizardStageCompleted("config", `已发送 ${labels.join("、")} 配置链接，请在手表上确认。`)
    state.wizard.stageNotes.config = `已整理 ${labels.join("、")} 并发送到手表，等待手表确认。`
    return true
  }

  async function handleWizardPrimaryAction() {
    const stage = state.wizard.currentStage
    if (stage === "prepare") {
      if (!wizardPrepareReady(live.value)) {
        showNotice("请先完成自动检查和手动确认。")
        return
      }
      markWizardStageCompleted("prepare", "环境已确认。")
      setWizardStage("connect", "开始填写手表连接信息。")
      return
    }
    if (stage === "connect") {
      if (!wizardConnectReady()) {
        showNotice("请先点击开始连接，并确认目标设备。")
        return
      }
      markWizardStageCompleted("connect", state.wizard.stageNotes.connect || "手表已连接。")
      setWizardStage("install", "确认安装包后继续。")
      clearWizardStageError()
      return
    }
    if (stage === "install") {
      if (!wizardInstallReady()) {
        showNotice("请先安装应用或打开已安装应用。")
        return
      }
      markWizardStageCompleted("install", state.wizard.stageNotes.install || "应用已安装并启动完成。")
      setWizardStage("config", "开始准备写入配置。")
      clearWizardStageError()
      return
    }
    if (!wizardConfigReady()) {
      showNotice("请先写入配置。")
      return
    }
    state.currentPage = "watch"
    showNotice("配置已完成，已切换到手表设备页。")
  }

  async function handleWizardSecondaryAction() {
    const stage = state.wizard.currentStage
    if (stage === "prepare") {
      await refreshInstallerState()
      state.wizard.lastRefreshLabel = `已刷新 ${shortTimeLabel()}`
      showNotice("已刷新自动检查。")
      return
    }
    const backMap = {
      connect: "prepare",
      install: "connect",
      config: "install"
    }
    clearWizardStageError()
    setWizardStage(backMap[stage] || "prepare")
  }

  async function retryCurrentWizardStage() {
    clearWizardStageError()
    await handleWizardPrimaryAction()
  }

  async function exportDiagnosticsAction() {
    try {
      const path = await invoke("ExportDiagnosticsBundle")
      showNotice(`诊断包已导出到 ${path}`)
    } catch (error) {
      showNotice(`导出诊断包失败：${String(error)}`)
    }
  }

  async function clearBackendPairingAction(scope = "beta") {
    if (state.pairingAction.busy) {
      return
    }
    state.pairingAction = {
      busy: true,
      scope
    }
    try {
      clearWizardStageError()
      if (scope === "dev") {
        state.developerSnapshot = await invoke("ClearDeveloperPairing")
        state.developerRepositories = Array.isArray(state.developerSnapshot?.repositories) ? state.developerSnapshot.repositories : state.developerRepositories
        hydrateDeveloperStateFromSnapshot(state.developerSnapshot)
        state.generatedBootstrap = null
        showNotice("已重置开发环境 dev 配对。")
        return
      }

      await invoke("ClearBackendPairing")
      state.snapshot = await loadSnapshot()
      state.backendLogs = await loadBackendLogs()
      syncInstallWizardDraft(state.snapshot)
      state.healthCheckResult = null
      state.generatedBootstrap = null
      if (state.currentPage === "install") {
        state.wizard.stageNotes.config = "本机配对已清空，可以重新写入配置。"
      }
      showNotice("已清空本机服务 beta 配对。")
    } catch (error) {
      showNotice(`${scope === "dev" ? "重置开发环境配对" : "清空 beta 配对"}失败：${String(error)}`)
    } finally {
      state.pairingAction = {
        busy: false,
        scope: ""
      }
    }
  }

  function pairingActionBusy(scope = "") {
    if (!state.pairingAction.busy) {
      return false
    }
    return !scope || state.pairingAction.scope === scope
  }

  function pairingActionLabel(scope, idleLabel, busyLabel) {
    return pairingActionBusy(scope) ? busyLabel : idleLabel
  }

  function touchWizardBoundField(path) {
    if (path === "installForm.pairIp" && !state.installForm.useSeparateConnectIP) {
      state.installForm.connectIp = state.installForm.pairIp
    }
    if (!path.startsWith("wizard.configEntries.")) {
      return
    }
    const [, , entryId, field] = path.split(".")
    const entry = wizardConfigEntry(entryId)
    if (!entry) {
      return
    }
    if (field === "url") {
      entry.validation = "idle"
      entry.message = entryId === "lan" ? "请重新检测" : "请重新校验"
    }
    if (field === "code") {
      entry.validation = "pending"
      entry.message = "先输入配置码再兑换"
      entry.redeemedDomain = ""
    }
  }

  function setNetworkInterface(ip) {
    const selected = availableNetworkOptions().find((item) => item.ip === ip)
    state.networkForm.selectedIp = ip
    state.networkForm.selectedInterface = selected?.label || ip
    persistInstallNetworkState()
  }

  function setDeveloperAccessMode(value) {
    state.developerForm.accessMode = normalizeDeveloperAccessMode(value)
    updateDeveloperBaseUrlFromAccessMode(state.developerForm.accessMode)
    persistDeveloperFormState()
  }

  function toggleDeveloperTunnel() {
    state.developerForm.managedTunnelEnabled = !state.developerForm.managedTunnelEnabled
    persistDeveloperFormState()
    void loadDeveloperEnvironmentSnapshot({ ensure: developerIsRunning() })
    showNotice(state.developerForm.managedTunnelEnabled ? "已启用开发隧道保活与自动恢复。" : "已关闭开发隧道保活与自动恢复。")
  }

  async function redeemDeveloperTunnelAction() {
    const code = state.developerForm.tunnelCode.trim()
    if (!code) {
      showNotice("请先输入开发隧道配置码。")
      return
    }
    try {
      state.developerSnapshot = await invoke("RedeemDeveloperTunnelCode", code)
      state.developerForm.tunnelCode = ""
      persistDeveloperFormState()
      hydrateDeveloperStateFromSnapshot(state.developerSnapshot)
      if (developerIsRunning() && developerTunnelIsManaged()) {
        await loadDeveloperEnvironmentSnapshot({ ensure: true })
      }
      if (state.developerForm.accessMode === "tunnel") {
        updateDeveloperBaseUrlFromAccessMode("tunnel")
        persistDeveloperFormState()
      }
      pushNotification({
        title: "开发隧道已激活",
        detail: developerTunnelBaseUrl() || "后续可切换到开发隧道访问方式。",
        level: "success",
        source: "开发隧道"
      })
    } catch (error) {
      await loadDeveloperEnvironmentSnapshot({ ensure: developerIsRunning() })
      pushNotification({
        title: "开发隧道激活失败",
        detail: String(error),
        level: "error",
        source: "开发隧道"
      })
    }
  }

  async function bootstrap() {
    ensureDesktopUpdateProgressListener()
    ensureDesktopStateChangedListener()
    state.theme = loadThemeState()
    state.settingsPreferences = loadSettingsPreferencesState()
    const desktopSettings = await loadDesktopSettings()
    applyDesktopSettings(desktopSettings)
    const requestedPage = new URLSearchParams(window.location.search).get("page") || window.location.hash.replace(/^#/, "")
    if (navItems.some((item) => item.id === requestedPage)) {
      state.currentPage = requestedPage
    }
    const savedDeveloperForm = loadDeveloperFormState()
    if (savedDeveloperForm) {
      state.developerForm = {
        ...state.developerForm,
        ...savedDeveloperForm,
        deviceName: savedDeveloperForm.deviceName || state.developerForm.deviceName
      }
    }
    applyDeveloperDesktopSettings(desktopSettings)
    const savedInstallNetwork = loadInstallNetworkState()
    if (savedInstallNetwork) {
      if (savedInstallNetwork.networkMode) {
        state.networkMode = savedInstallNetwork.networkMode
      }
      if (savedInstallNetwork.networkForm?.customUrl) {
        state.networkForm.customUrl = savedInstallNetwork.networkForm.customUrl
      }
      if (savedInstallNetwork.networkForm?.port) {
        state.networkForm.port = savedInstallNetwork.networkForm.port
      }
      if (savedInstallNetwork.configEntries && typeof savedInstallNetwork.configEntries === "object") {
        state.wizard.configEntries = {
          ...state.wizard.configEntries,
          ...savedInstallNetwork.configEntries
        }
      }
    }
    state.snapshot = await loadSnapshot()
    hydrateStateFromSnapshot(state.snapshot)
    state.backendLogs = await loadBackendLogs()
    state.installerState = await loadInstallerStatus()
    state.codexHookState = await loadCodexHookStatus()
    await loadDesktopUpdateStatus()
    await loadDeveloperEnvironmentSnapshot({ ensure: false })
    syncWizardWithInstallerState()
    if (state.settingsPreferences.autoStartBackend) {
      await maybeAutoStartBackend()
    }
    ensureGlobalHealthTicker()
    await runGlobalHealthCheck({ manual: false })
  }

  return {
    state,
    live,
    topbarItems,
    selectedDevice,
    installerDevicesList,
    currentApi,
    watchApi,
    unreadNotificationCount,
    currentWizardStage,
    wizardFailure,
    rawLogLines,
    allRawLogLines,
    eventTimeline,
    watchSummaryCards,
    logsSummaryCards,
    constants: {
      navItems,
      settingsTabs,
      installWizardStages,
      installConfigMeta,
      prepareManualChecks
    },
    actions: {
      bootstrap,
      setPage: (page) => { state.currentPage = page },
      setSettingsTab: (tab) => { state.settingsTab = tab },
      setWatchTab: (tab) => { state.watchTab = tab },
      setLogsTab: (tab) => { state.logsTab = tab },
      toggleNotificationPanel,
      setTheme,
      toggleTheme,
      toggleSettingsPreference,
      closeNotificationPanel,
      runGlobalHealthCheck,
      copyText,
      showNotice,
      refreshInstallerState: () => refreshInstallerState({ notifySuccess: true }),
      startBackendAction,
      restartBackendAction,
      runHealthCheckAction,
      openFolderAction,
      refreshCodexHookStatusAction,
      installCodexHooksAction,
      prepareWatchBootstrapAction,
      applyWatchBootstrapAction,
      prepareDevWatchBootstrapAction,
      applyDevWatchBootstrapAction,
      submitRemoteWatchBootstrapAction,
      installWatchAppAction,
      launchWatchAppAction,
      selectInstallerDevice,
      runConnectStageAction,
      runInstallStageAction,
      runConfigStageAction,
      handleWizardPrimaryAction,
      handleWizardSecondaryAction,
      retryCurrentWizardStage,
      copyDiagnosticsAction,
      checkForUpdatesAction,
      installDesktopUpdateAction,
      exportDiagnosticsAction,
      clearBackendPairingAction,
      validateConfigEntryAction,
      clearDeveloperLogsAction,
      selectDeveloperRepoDirAction,
      toggleDeveloperEnvironmentAction,
      restartDeveloperEnvironmentAction,
      setDeveloperAccessMode,
      toggleDeveloperTunnel,
      redeemDeveloperTunnelAction,
      setNetworkInterface,
      persistDeveloperFormState,
      persistInstallNetworkState,
      updateDeveloperBaseUrlFromAccessMode,
      touchWizardBoundField,
      markWizardStageCompleted,
      setWizardStage,
      clearWizardStageError
    },
    selectors: {
      availableNetworkOptions,
      currentSelectedIP,
      currentListenPort,
      currentApiBase,
      currentWatchApiBase,
      currentAccessModeLabel,
      currentTunnel,
      installerDevices,
      installerSelectionRequired,
      selectedInstallerDevice,
      preferredBackendDevice,
      installerStatusTone,
      installerStatusLabel,
      installerStatusNote,
      installerSummaryNote,
      developerTunnelBaseUrl,
      developerHealthzUrl,
      developerEnvironmentRequest,
      developerStatus,
      developerTunnelStatus,
      developerTunnelIsManaged,
      developerTunnelStateLabel,
      developerLogs,
      developerLogFileLabel,
      developerLogEntries,
      developerLogLines,
      developerLogLevel,
      developerLogLevelLabel,
      formatDeveloperLogTime,
      developerStatusPhase,
      developerIsRunning,
      developerIsHealthy,
      developerTunnelIsHealthy,
      developerStateTone,
      developerStateLabel,
      developerStartedDurationLabel,
      developerStartCommand,
      developerCurrentRepoLabel,
      developerDeviceConnectionLabel,
      developerCanSendToWatch,
      developerSendDisabledReason,
      developerEnvFileLabel,
      wizardStageIndex,
      wizardStageMeta,
      wizardStageCompleted,
      wizardStageUnlocked,
      wizardCurrentStageError,
      pairingSkipReason,
      wizardAutoChecks,
      wizardManualChecksDone,
      toggleAllWizardManualChecks,
      wizardPrepareReady,
      wizardConnectReady,
      wizardConfigEntry,
      wizardEnabledConfigEntries,
      wizardConfigChecksInProgress,
      wizardConfigCheckingLabel,
      wizardInstallReady,
      wizardConfigReady,
      configWriteActionLabel,
      wizardFailureTips,
      buildEnabledBootstrapEndpoints,
      remoteBootstrapDefaultApiBase,
      remoteBootstrapHealthLabel,
      remoteBootstrapEnvironmentLabel,
      currentWatchVersionLabel,
      currentWatchInstalledVersionLabel,
      desktopUpdateProgressLabel,
      currentWatchPackageVersionLabel,
      deviceHistoryRows,
      copyFeedbackActive,
      notificationLevelTone,
      pairingActionBusy,
      pairingActionLabel,
      formatTimeAgo
    }
  }
}
