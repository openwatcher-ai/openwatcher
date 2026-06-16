import launcherIconUrl from "./assets/openwatcher_launcher.png"
import {
  backendStatusWithHealthResult,
  backendTargetFromHealthResult,
  isDraftField,
  markDraftField,
  setValueAtPath
} from "./interaction_state.mjs"
import { INSTALL_GUIDES } from "./install_guides"

const NAV_ITEMS = [
  { id: "install", label: "安装向导", icon: "wand" },
  { id: "watch", label: "手表设备", icon: "watch" },
  { id: "logs", label: "日志与诊断", icon: "document" },
  { id: "settings", label: "设置", icon: "gear" }
]

const SETTINGS_TABS = [
  { id: "general", label: "常规" },
  { id: "backend", label: "本机服务" },
  { id: "codex", label: "Codex 环境" },
  { id: "resources", label: "手表安装资源" },
  { id: "privacy", label: "隐私与安全" },
  { id: "updates", label: "更新" },
  { id: "developer", label: "开发者" },
  { id: "advanced", label: "高级" },
  { id: "danger", label: "危险操作", danger: true }
]

const fallbackSnapshot = {
  productVersion: "dev",
  system: {
    platform: "darwin",
    architecture: "arm64",
    goVersion: "go1.26.2",
    desktopConfigDir: "~/Library/Application Support/OpenWatcher"
  },
  codex: {
    homeLabel: "~/.codex",
    homeDetected: true,
    authDetected: true,
    sessionsDetected: true,
    readable: true,
    message: "已发现 ~/.codex"
  },
  backend: {
    state: "missing",
    message: "未找到 OpenWatcher 本机服务组件。请把二进制放到 bundled/openwatcher//，或在仓库根目录生成 bin/openwatcher。",
    resolvedBinary: "",
    binarySource: "bundled/openwatcher/",
    friendlyError: "未找到 OpenWatcher 本机服务组件。请把二进制放到 bundled/openwatcher//，或在仓库根目录生成 bin/openwatcher。",
    running: false,
    recentLogCount: 0,
    healthProbePath: "http://127.0.0.1:8787/healthz",
    configPathLabel: "~/.openwatcher/config.json",
    configuredListen: "127.0.0.1:8787",
    configuredPublicBaseUrl: "http://127.0.0.1:8787"
  },
  tunnel: {
    state: "unconfigured",
    message: "尚未兑换 OpenWatcher 托管隧道配置码。",
    configured: false,
    running: false,
    tokenExpired: false,
    publicBaseUrl: "",
    tunnelId: "",
    tokenVersion: 0,
    redeemedAt: "",
    tokenFingerprint: "",
    resolvedBinary: "",
    binarySource: "",
    startedAt: "",
    lastRedeemErrorCode: "",
    sharedProcessNotice: "",
    healthProbePath: "",
    recentLogCount: 0,
    lastHealth: null
  },
  accessMode: "本机模式",
  networkContext: {
    interfaces: [
      {
        name: "en0",
        label: "Wi-Fi (en0) — 192.168.31.12",
        ip: "192.168.31.12",
        recommended: true
      }
    ],
    recommendedIp: "192.168.31.12",
    recommendedTag: "Wi-Fi (en0) — 192.168.31.12"
  }
}

const fallbackInstallerState = {
  adb: {
    available: false,
    path: "",
    version: "",
    message: "未检测到 ADB"
  },
  devices: [],
  selectedSerial: "",
  selectedLabel: "",
  selectedPort: 0,
  apk: {
    available: false,
    path: "",
    label: "",
    versionName: "",
    versionCode: 0,
    packageName: "",
    sha256: "",
    debug: false,
    devFallback: false,
    message: "未检测到可安装 APK"
  },
  phase: "idle",
  message: "",
  logs: []
}

const PREPARE_MANUAL_CHECKS = [
  { id: "wifi", label: "手表已连接 Wi‑Fi，且与电脑同一局域网" },
  { id: "developer", label: "已启用开发者模式" },
  { id: "wireless", label: "已开启无线调试" },
  { id: "pairPage", label: "已打开“使用配对码配对设备”页面" }
]

const INSTALL_WIZARD_STAGES = [
  {
    id: "prepare",
    title: "环境准备",
    navSummary: "确认安装资源已就绪，并在手表上打开调试功能。",
    subtitle: "先确认安装需要的内容已经准备好。",
    tag: "阶段 1 / 4",
    primaryLabel: "已确认",
    secondaryLabel: "刷新检查"
  },
  {
    id: "connect",
    title: "连接手表",
    navSummary: "填写手表信息并完成连接。",
    subtitle: "根据手表上的页面信息完成连接，然后确认这次要安装的设备。",
    tag: "阶段 2 / 4",
    primaryLabel: "去安装应用",
    secondaryLabel: "上一步"
  },
  {
    id: "install",
    title: "安装应用",
    navSummary: "把 OpenWatcher 安装到手表并自动启动。",
    subtitle: "确认目标设备和安装包后，完成安装并自动启动应用。",
    tag: "阶段 3 / 4",
    primaryLabel: "去配置",
    secondaryLabel: "上一步"
  },
  {
    id: "config",
    title: "写入配置",
    navSummary: "启用可用地址并发送到手表确认。",
    subtitle: "启用要发送到手表的地址，确认保存后手表会按可用性和速度选择连接入口。",
    tag: "阶段 4 / 4",
    primaryLabel: "完成",
    secondaryLabel: "上一步"
  }
]

const INSTALL_CONFIG_META = {
  lan: {
    label: "局域网",
    icon: "🌐",
    inputLabel: "局域网地址",
    validateLabel: "重新检测",
    inactiveText: "启用后会一起发送到手表确认。"
  },
  public: {
    label: "公网",
    icon: "🔗",
    inputLabel: "公网地址",
    validateLabel: "重新校验",
    inactiveText: "启用后会一起发送到手表确认。"
  },
  tunnel: {
    label: "托管隧道",
    icon: "☁️",
    inputLabel: "配置码",
    validateLabel: "兑换配置码",
    inactiveText: "先兑换，再决定是否一起写入。"
  }
}

const DEFAULT_SIDECAR_PORT = "8787"
const DEFAULT_DEV_SIDECAR_PORT = "18787"

function suggestedLanURL(snapshot = fallbackSnapshot) {
  const ip = snapshot?.networkContext?.recommendedIp || fallbackSnapshot.networkContext.recommendedIp
  const listen = snapshot?.backend?.lastHealth?.config?.listen || snapshot?.backend?.configuredListen || `127.0.0.1:${DEFAULT_SIDECAR_PORT}`
  const port = listen.split(":").pop() || DEFAULT_SIDECAR_PORT
  return ip ? `http://${ip}:${port}` : ""
}

function suggestedDevBaseUrl() {
  const device = preferredBackendDevice()
  if (device?.isEmulator) {
    return `http://${device.hostAlias || "10.0.2.2"}:${DEFAULT_DEV_SIDECAR_PORT}`
  }
  const selectedIP = currentSelectedIP()
  return `http://${selectedIP}:${DEFAULT_DEV_SIDECAR_PORT}`
}

function createInstallConfigEntries(snapshot = fallbackSnapshot) {
  return {
    lan: {
      enabled: true,
      url: suggestedLanURL(snapshot),
      validation: "idle",
      message: "等待检测",
      lastCheckedURL: ""
    },
    public: {
      enabled: false,
      url: "https://openwatcher.example.com",
      validation: "idle",
      message: "等待校验",
      lastCheckedURL: ""
    },
    tunnel: {
      enabled: false,
      code: "",
      redeemedDomain: snapshot?.tunnel?.publicBaseUrl || "",
      validation: snapshot?.tunnel?.publicBaseUrl ? "idle" : "pending",
      message: snapshot?.tunnel?.publicBaseUrl ? "已兑换，可继续校验" : "先输入配置码再兑换",
      lastCheckedURL: snapshot?.tunnel?.publicBaseUrl || ""
    }
  }
}

const pageState = {
  currentPage: "install",
  theme: "dark",
  snapshot: fallbackSnapshot,
  installerState: fallbackInstallerState,
  developerRepositories: [],
  developerSnapshot: null,
  backendLogs: [],
  copiedText: "",
  tunnelExpiryNoticeKey: "",
  backendAutoStartAttempted: false,
  globalHealthTicker: null,
  globalHealthRunning: false,
  globalHealthSummary: {
    checking: false,
    lastCheckedAt: "",
    targets: {}
  },
  developerAction: {
    busy: false,
    targetEnabled: false,
    label: ""
  },
  pairingAction: {
    busy: false,
    scope: ""
  },
  developerAcknowledged: false,
  developerConfirmModalOpen: false,
  pendingSettingsTab: "",
  notificationCounter: 0,
  notifications: [],
  notificationPanelOpen: false,
  selectedLogSource: "all",
  networkMode: "lan",
  settingsTab: "general",
  healthCheckResult: null,
  generatedBootstrap: null,
  wizard: {
    currentStage: "prepare",
    completedStages: [],
    maxUnlockedIndex: 0,
    stageNotes: {},
    error: null,
    lastRefreshLabel: "尚未刷新",
    manualChecks: {
      wifi: false,
      developer: false,
      wireless: false,
      pairPage: false
    },
    guide: {
      selectedBrand: INSTALL_GUIDES[0]?.id || "xiaomi",
      modalOpen: false,
      stepIndex: 0
    },
    suggestedLanURL: suggestedLanURL(fallbackSnapshot),
    configEntries: createInstallConfigEntries(fallbackSnapshot)
  },
  installForm: {
    pairIp: "",
    pairPort: "",
    pairingCode: "",
    connectIp: "",
    connectPort: "",
    useSeparateConnectIP: false
  },
  watchForm: {
    deviceName: "watch"
  },
  remoteBootstrapForm: {
    bootstrapCode: "",
    environment: "beta",
    apiBase: "",
    tunnelCode: "",
    submitting: false,
    result: null
  },
  developerForm: {
    enabled: false,
    mode: "workspace",
    repoPath: "",
    hostAlias: "",
    devBaseUrl: "",
    deviceName: "watch",
    managedTunnelEnabled: false,
    tunnelCode: "",
    accessMode: "emulator"
  },
  networkForm: {
    selectedInterface: "Wi-Fi (en0) — 192.168.31.12",
    selectedIp: "192.168.31.12",
    bindAll: false,
    port: DEFAULT_SIDECAR_PORT,
    customUrl: "https://openwatcher.example.com",
    tunnelCode: ""
  },
  draftFields: {}
}

const DEV_ENV_STORAGE_KEY = "openwatcher-dev-environment"
const DEV_ENV_ACK_STORAGE_KEY = "openwatcher-dev-environment-ack"
const INSTALL_NETWORK_STORAGE_KEY = "openwatcher-install-network"
const GLOBAL_HEALTH_INTERVAL_MS = 15000
const DEVELOPER_STARTUP_WAIT_MS = 30000

const validPageIds = new Set(NAV_ITEMS.map((item) => item.id))

const sampleInstallLogs = [
  "[10:24:31] [OK] adb pair 192.168.31.88:37153",
  "[10:24:32] [OK] Successfully paired",
  "[10:24:33] [OK] adb connect 192.168.31.88:40221",
  "[10:24:33] [OK] connected to 192.168.31.88:40221",
  "[10:24:35] [RUN] adb install -r openwatcher-watch-release.apk",
  "[10:24:38] [OK] Success"
]

const sampleEventTimeline = [
  { time: "10:24:33.128", event: "ADB 配对成功: 192.168.31.88:37153", level: "INFO", source: "ADB 安装" },
  { time: "10:24:33.682", event: "已连接手表: watch (192.168.31.88)", level: "INFO", source: "手表 App" },
  { time: "10:24:34.116", event: "开始安装 APK: openwatcher-watch-release.apk", level: "INFO", source: "ADB 安装" },
  { time: "10:24:35.209", event: "APK 安装成功", level: "SUCCESS", source: "ADB 安装" },
  { time: "10:24:36.771", event: "已发送 bootstrap 配置链接，等待手表确认", level: "INFO", source: "本机服务" },
  { time: "10:24:37.215", event: "手表确认配置: 配置已保存", level: "INFO", source: "手表 App" },
  { time: "10:24:37.998", event: "手表请求 /api/status 成功", level: "INFO", source: "网络访问" }
]

const sampleRawLogs = [
  "[10:24:33.128] [INFO] [adb] Successfully paired with 192.168.31.88:37153",
  "[10:24:33.682] [INFO] [watch] Connected to watch (192.168.31.88)",
  "[10:24:34.116] [INFO] [adb] Installing APK: openwatcher-watch-release.apk",
  "[10:24:35.209] [SUCCESS] [adb] APK installed successfully",
  "[10:24:36.771] [INFO] [backend] Sent bootstrap configuration to watch",
  "[10:24:37.215] [INFO] [watch] Configuration acknowledged and saved",
  "[10:24:37.998] [INFO] [network] GET /api/status 200 OK (192.168.31.88)",
  "[10:24:40.512] [INFO] [backend] Health check 127.0.0.1:8787 OK"
]

const sampleDevices = [
  {
    name: "Xiaomi Watch",
    model: "M2233W1",
    android: "Android 14 (API 34)",
    lastSeen: "2025-05-20 10:24:35",
    adbState: "已连接",
    compatibility: "完全兼容",
    badge: "当前"
  },
  {
    name: "OPPO Watch 3",
    model: "OWW211",
    android: "Android 11 (API 30)",
    lastSeen: "2025-05-18 18:36:12",
    adbState: "未连接",
    compatibility: "部分兼容"
  },
  {
    name: "vivo WATCH 2",
    model: "WA2212",
    android: "Android 10 (API 29)",
    lastSeen: "2025-05-15 09:12:05",
    adbState: "未连接",
    compatibility: "部分兼容"
  }
]

async function invoke(method, ...args) {
  const target = window?.go?.main?.App?.[method]
  if (!target) {
    throw new Error(`Wails 绑定缺少 ${method}`)
  }
  return target(...args)
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

function persistDeveloperFormState() {
  try {
    window.localStorage.setItem(DEV_ENV_STORAGE_KEY, JSON.stringify(pageState.developerForm))
  } catch {
    // ignore local persistence failures
  }
}

function loadDeveloperAcknowledgedState() {
  try {
    return window.localStorage.getItem(DEV_ENV_ACK_STORAGE_KEY) === "1"
  } catch {
    return false
  }
}

function persistDeveloperAcknowledgedState() {
  try {
    window.localStorage.setItem(DEV_ENV_ACK_STORAGE_KEY, pageState.developerAcknowledged ? "1" : "0")
  } catch {
    // ignore local persistence failures
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

function persistInstallNetworkState() {
  try {
    window.localStorage.setItem(INSTALL_NETWORK_STORAGE_KEY, JSON.stringify({
      networkMode: pageState.networkMode,
      networkForm: {
        customUrl: pageState.networkForm.customUrl,
        port: pageState.networkForm.port
      },
      configEntries: pageState.wizard.configEntries
    }))
  } catch {
    // ignore local persistence failures
  }
}

function nextNotificationId() {
  pageState.notificationCounter += 1
  return `notice-${pageState.notificationCounter}`
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

function pushNotification({ title, detail = "", level = "info", source = "系统", dedupeKey = "" }) {
  const normalizedTitle = String(title || "").trim()
  const normalizedDetail = String(detail || "").trim()
  if (!normalizedTitle) {
    return
  }
  const now = new Date()
  const key = dedupeKey || `${source}|${normalizedTitle}|${normalizedDetail}`
  const existing = pageState.notifications.find((item) => item.key === key)
  if (existing) {
    existing.at = now.toISOString()
    existing.timeLabel = shortTimeLabel(now)
    existing.read = false
    existing.level = level
    existing.title = normalizedTitle
    existing.detail = normalizedDetail
    existing.source = source
  } else {
    pageState.notifications.unshift({
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
    if (pageState.notifications.length > 60) {
      pageState.notifications = pageState.notifications.slice(0, 60)
    }
  }
  renderApp({ preserveInteraction: true })
}

function unreadNotificationCount() {
  return pageState.notifications.filter((item) => !item.read).length
}

function markNotificationsRead() {
  for (const item of pageState.notifications) {
    item.read = true
  }
}

function toggleNotificationPanel() {
  pageState.notificationPanelOpen = !pageState.notificationPanelOpen
  if (pageState.notificationPanelOpen) {
    markNotificationsRead()
  }
  renderApp()
}

function closeNotificationPanel() {
  if (!pageState.notificationPanelOpen) {
    return
  }
  pageState.notificationPanelOpen = false
  renderApp()
}

function currentDeveloperHostAlias() {
  const device = preferredBackendDevice()
  return String(device?.hostAlias || pageState.developerForm.hostAlias || "10.0.2.2").trim()
}

function normalizeDeveloperAccessMode(value) {
  return ["emulator", "lan", "tunnel", "custom"].includes(value) ? value : "emulator"
}

function trimTrailingSlash(value) {
  return String(value || "").trim().replace(/\/+$/, "")
}

function developerTunnelBaseUrl() {
  return trimTrailingSlash(developerTunnelStatus()?.publicBaseUrl || "")
}

function developerSuggestedBaseUrl(accessMode = pageState.developerForm.accessMode) {
  const mode = normalizeDeveloperAccessMode(accessMode)
  const port = developerPortValue()
  const hostAlias = currentDeveloperHostAlias() || "10.0.2.2"
  if (mode === "lan") {
    return `http://${currentSelectedIP()}:${port}`
  }
  if (mode === "tunnel") {
    return developerTunnelBaseUrl() || pageState.developerForm.devBaseUrl.trim() || `http://${hostAlias}:${port}`
  }
  if (mode === "custom") {
    return pageState.developerForm.devBaseUrl.trim()
  }
  return `http://${hostAlias}:${port}`
}

function developerBaseUrlForCurrentForm() {
  const baseURL = trimTrailingSlash(pageState.developerForm.devBaseUrl)
  if (baseURL) {
    return baseURL
  }
  return trimTrailingSlash(developerSuggestedBaseUrl())
}

function developerPortValue(accessMode = pageState.developerForm.accessMode) {
  const mode = normalizeDeveloperAccessMode(accessMode)
  if (mode === "emulator" || mode === "lan" || mode === "tunnel") {
    return DEFAULT_DEV_SIDECAR_PORT
  }
  const currentBaseURL = trimTrailingSlash(pageState.developerForm.devBaseUrl || developerStatus()?.baseURL || "")
  if (currentBaseURL) {
    try {
      const parsed = new URL(currentBaseURL)
      if (parsed.port) {
        return parsed.port
      }
      return DEFAULT_DEV_SIDECAR_PORT
    } catch {
      // ignore malformed in-progress values
    }
  }
  return DEFAULT_DEV_SIDECAR_PORT
}

function updateDeveloperBaseUrlFromAccessMode(nextMode = pageState.developerForm.accessMode) {
  const normalizedMode = normalizeDeveloperAccessMode(nextMode)
  pageState.developerForm.accessMode = normalizedMode
  const nextBaseURL = developerSuggestedBaseUrl(normalizedMode)
  if (normalizedMode !== "custom") {
    pageState.developerForm.devBaseUrl = trimTrailingSlash(nextBaseURL)
  }
}

function developerAccessModeFromBaseUrl(baseURL = developerBaseUrlForCurrentForm()) {
  const normalized = trimTrailingSlash(baseURL)
  if (!normalized) {
    return pageState.developerForm.accessMode || "emulator"
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

function pairingActionBusy(scope = "") {
  if (!pageState.pairingAction.busy) {
    return false
  }
  if (!scope) {
    return true
  }
  return pageState.pairingAction.scope === scope
}

function pairingActionLabel(scope, idleLabel, busyLabel) {
  return pairingActionBusy(scope) ? busyLabel : idleLabel
}

function developerEnvironmentRequest(override = {}) {
  return {
    enabled: Object.prototype.hasOwnProperty.call(override, "enabled") ? Boolean(override.enabled) : Boolean(pageState.developerForm.enabled),
    mode: "workspace",
    repoPath: (override.repoPath ?? pageState.developerForm.repoPath).trim(),
    hostAlias: override.hostAlias || currentDeveloperHostAlias(),
    baseURL: trimTrailingSlash(override.baseURL || developerBaseUrlForCurrentForm()),
    deviceName: (override.deviceName ?? pageState.developerForm.deviceName).trim(),
    managedTunnelEnabled: Object.prototype.hasOwnProperty.call(override, "managedTunnelEnabled")
      ? Boolean(override.managedTunnelEnabled)
      : Boolean(pageState.developerForm.managedTunnelEnabled)
  }
}

async function loadDeveloperEnvironmentSnapshot({ ensure = false, preserveDrafts = false } = {}) {
  try {
    const method = ensure ? "EnsureDeveloperEnvironment" : "GetDeveloperEnvironmentSnapshot"
    const snapshot = await invoke(method, developerEnvironmentRequest())
    pageState.developerSnapshot = snapshot
    pageState.developerRepositories = Array.isArray(snapshot?.repositories) ? snapshot.repositories : []
    hydrateDeveloperStateFromSnapshot(snapshot, { preserveDrafts })
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

function shouldPreserveDraftField(path, options = {}) {
  return Boolean(options.preserveDrafts && isDraftField(pageState.draftFields, path))
}

function hydrateDeveloperStateFromSnapshot(snapshot, options = {}) {
  const repositories = Array.isArray(snapshot?.repositories) ? snapshot.repositories : []
  const status = snapshot?.status || null
  const preferredRepo = repositories.find((item) => item.autoDetected && item.valid) || repositories.find((item) => item.valid) || repositories[0]
  const preserveRepoPath = shouldPreserveDraftField("developerForm.repoPath", options)
  const preserveHostAlias = shouldPreserveDraftField("developerForm.hostAlias", options)
  const preserveDevBaseUrl = shouldPreserveDraftField("developerForm.devBaseUrl", options)
  const preserveEnabled = shouldPreserveDraftField("developerForm.enabled", options)

  if (!preserveRepoPath && status?.resolvedRepoPath) {
    pageState.developerForm.repoPath = status.resolvedRepoPath
  } else if (!preserveRepoPath && !pageState.developerForm.repoPath && preferredRepo?.path) {
    pageState.developerForm.repoPath = preferredRepo.path
  }
  if (!preserveHostAlias) {
    pageState.developerForm.hostAlias = currentDeveloperHostAlias() || "10.0.2.2"
  }
  if (!preserveDevBaseUrl) {
    if (status?.baseURL) {
      pageState.developerForm.devBaseUrl = trimTrailingSlash(status.baseURL)
    } else if (!pageState.developerForm.devBaseUrl) {
      pageState.developerForm.devBaseUrl = trimTrailingSlash(developerSuggestedBaseUrl(pageState.developerForm.accessMode))
    }
    pageState.developerForm.accessMode = developerAccessModeFromBaseUrl(pageState.developerForm.devBaseUrl)
    if (pageState.developerForm.accessMode !== "custom") {
      updateDeveloperBaseUrlFromAccessMode(pageState.developerForm.accessMode)
    }
  }
  if (!preserveEnabled) {
    pageState.developerForm.enabled = developerStatusPhase() === "running" || developerStatusPhase() === "starting"
  }
}

function developerStatus() {
  return pageState.developerSnapshot?.status || null
}

function developerTunnelStatus() {
  return pageState.developerSnapshot?.tunnel || null
}

function developerTunnelIsManaged() {
  return Boolean(pageState.developerForm.managedTunnelEnabled)
}

function developerTunnelStateTone() {
  const tunnel = developerTunnelStatus()
  if (!developerTunnelIsManaged()) {
    return "neutral"
  }
  if (tunnel?.running) {
    return "green"
  }
  if (tunnel?.configured) {
    return "amber"
  }
  return "amber"
}

function developerTunnelStateLabel() {
  const tunnel = developerTunnelStatus()
  if (!developerTunnelIsManaged()) {
    return "已关闭"
  }
  if (tunnel?.running) {
    return "运行中"
  }
  if (tunnel?.configured) {
    return "待启动"
  }
  return "未激活"
}

function developerLogs() {
  return Array.isArray(pageState.developerSnapshot?.logs) ? pageState.developerSnapshot.logs : []
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
  const status = developerStatus()
  if (pageState.developerAction.busy) {
    return pageState.developerAction.targetEnabled ? "starting" : "stopping"
  }
  if (!status) {
    return "stopped"
  }
  if (status.running && status.lastHealth?.ok) {
    return "running"
  }
  if (status.state === "recovering" || status.running) {
    return "starting"
  }
  if (status.state === "error") {
    return "failed"
  }
  return "stopped"
}

function developerIsRunning() {
  return developerStatusPhase() === "running" || developerStatusPhase() === "starting"
}

function developerIsHealthy() {
  const status = developerStatus()
  if (!status) {
    return false
  }
  return Boolean(status.lastHealth?.ok)
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

function developerStatusHeading() {
  switch (developerStatusPhase()) {
    case "running":
      return "开发环境运行中"
    case "starting":
      return "开发环境启动中"
    case "stopping":
      return "开发环境停止中"
    case "failed":
      return "开发环境启动失败"
    default:
      return "开发环境未启动"
  }
}

function developerStatusDetail() {
  const status = developerStatus()
  switch (developerStatusPhase()) {
    case "running":
      if (status?.externallyManaged) {
        return "已检测到当前仓库的本机开发服务，可直接发送到手表或由 Desktop 重新启动。"
      }
      return "环境已就绪并正常运行，可随时发送到手表进行调试。"
    case "starting":
      return pageState.developerAction.label || "正在执行启动脚本，请稍候。"
    case "stopping":
      return "正在停止当前开发环境。"
    case "failed":
      return status?.message || developerRecentLogMessage() || "启动脚本执行失败，请查看下方日志。"
    default:
      return "选择仓库和启动脚本后，可在右侧启动开发环境。"
  }
}

function developerPrimaryActionLabel() {
  switch (developerStatusPhase()) {
    case "running":
      return "停止环境"
    case "starting":
      return "启动中..."
    case "stopping":
      return "停止中..."
    case "failed":
      return "重新启动"
    default:
      return "启动环境"
  }
}

function developerLastCheckedLabel() {
  const checkedAt = developerStatus()?.lastCheckedAt
  if (!checkedAt) {
    return "尚未检测"
  }
  return formatTimeAgo(checkedAt)
}

function formatTimeAgo(isoText) {
  const target = new Date(isoText)
  if (Number.isNaN(target.getTime())) {
    return "刚刚"
  }
  const diffMs = Date.now() - target.getTime()
  if (diffMs < 60 * 1000) {
    return "刚刚"
  }
  const diffMinutes = Math.round(diffMs / (60 * 1000))
  if (diffMinutes < 60) {
    return `${diffMinutes} 分钟前`
  }
  const diffHours = Math.round(diffMinutes / 60)
  if (diffHours < 24) {
    return `${diffHours} 小时前`
  }
  const diffDays = Math.round(diffHours / 24)
  return `${diffDays} 天前`
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
  const diffMs = Math.max(0, Date.now() - started.getTime())
  const diffMinutes = Math.floor(diffMs / (60 * 1000))
  if (diffMinutes < 1) {
    return "已启动不到 1 分钟"
  }
  if (diffMinutes < 60) {
    return `已启动 ${diffMinutes} 分钟`
  }
  const hours = Math.floor(diffMinutes / 60)
  const minutes = diffMinutes % 60
  if (minutes === 0) {
    return `已启动 ${hours} 小时`
  }
  return `已启动 ${hours} 小时 ${minutes} 分钟`
}

function developerStartCommand() {
  return String(developerStatus()?.startCommand || developerStatus()?.resolvedScriptPath || "").trim()
}

function developerEnvironmentSummaryLines() {
  return {
    baseURL: developerBaseUrlForCurrentForm(),
    healthz: developerHealthzUrl(),
    bootstrap: pageState.generatedBootstrap?.bootstrapUri ? "已生成" : "未生成"
  }
}

function developerCurrentRepoLabel() {
  const status = developerStatus()
  const repoPath = status?.resolvedRepoPath || pageState.developerForm.repoPath
  if (!repoPath) {
    return "未选择仓库"
  }
  return repoPath.split(/[\\/]/).filter(Boolean).pop() || repoPath
}

function developerDeviceConnectionLabel() {
  const device = selectedInstallerDevice()
  if (device) {
    return `${device.displayName || pageState.developerForm.deviceName || "watch"}（已连接）`
  }
  return `${pageState.developerForm.deviceName || "watch"}（未连接）`
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

function formatDeveloperLogTime(raw) {
  const text = String(raw || "").trim()
  if (!text) {
    return "--:--:--"
  }
  const matched = text.match(/^(\d{4})-(\d{2})-(\d{2})T(\d{2}:\d{2}:\d{2})/)
  if (matched) {
    return `${matched[2]}-${matched[3]} ${matched[4]}`
  }
  return text
}

function renderDeveloperLogLines(limit = 0) {
  const entries = developerLogEntries(limit)
  if (entries.length === 0) {
    return `<div class="developer-log-entry is-empty"><span class="developer-log-time">--:--:--</span><span class="developer-log-level level-info">INFO</span><span class="developer-log-message">暂无开发环境日志</span></div>`
  }
  return entries.map((line) => {
    const level = developerLogLevel(line.message)
    return `
      <div class="developer-log-entry">
        <span class="developer-log-time">${escapeHtml(formatDeveloperLogTime(line.at))}</span>
        <span class="developer-log-level level-${level}">${developerLogLevelLabel(line.message)}</span>
        <span class="developer-log-message">${escapeHtml(line.message || "")}</span>
      </div>
    `
  }).join("")
}

function openDeveloperConfirmModal() {
  pageState.developerConfirmModalOpen = true
  renderApp()
}

function closeDeveloperConfirmModal() {
  pageState.developerConfirmModalOpen = false
  renderApp()
}

async function ensureRuntimeDependenciesIfNeeded({ manual = false } = {}) {
  const missingInstaller = !pageState.installerState?.adb?.available || !pageState.installerState?.apk?.available
  const missingTunnel = !pageState.snapshot?.tunnel?.resolvedBinary
  if (!manual && !missingInstaller && !missingTunnel) {
    return
  }
  pageState.snapshot = await invoke("EnsureRuntimeDependencies")
  pageState.installerState = await loadInstallerStatus()
}

function buildGlobalHealthTargets(live, developerSnapshot, auxiliaryChecks = {}) {
  const installerHealthy = Boolean(pageState.installerState?.adb?.available && pageState.installerState?.apk?.available)
  const developerStatus = developerSnapshot?.status || null
  const developerTunnel = developerSnapshot?.tunnel || null
  const developerHealthy = developerStatus
    ? Boolean(developerStatus.running && developerStatus.lastHealth?.ok)
    : null
  const developerTunnelHealthy = developerTunnelIsManaged()
    ? Boolean(developerTunnel?.running || developerTunnel?.lastHealth?.ok)
    : null
  return {
    codex: { ok: Boolean(live.codexHealthy), detail: live.codexStatusNote || "Codex 未就绪", source: "Codex" },
    backend: { ok: Boolean(live.backendHealthy), detail: live.backendStatusNote || "本机服务未就绪", source: "本机服务" },
    resources: { ok: installerHealthy, detail: installerHealthy ? "运行时依赖齐全" : "存在缺失的运行时依赖", source: "安装资源" },
    developer: developerHealthy == null ? null : {
      ok: developerHealthy,
      detail: developerStatus?.message || "开发环境未就绪",
      source: "开发环境"
    },
    developerTunnel: developerTunnelHealthy == null ? null : {
      ok: developerTunnelHealthy,
      detail: developerTunnel?.message || "开发隧道未就绪",
      source: "开发隧道"
    },
    publicEntry: auxiliaryChecks.publicEntry || null,
    tunnelEntry: auxiliaryChecks.tunnelEntry || null
  }
}

function applyHealthNotifications(nextTargets) {
  const previousTargets = pageState.globalHealthSummary.targets || {}
  let eventCount = 0
  const emit = (payload) => {
    eventCount += 1
    pushNotification(payload)
  }
  for (const [key, next] of Object.entries(nextTargets)) {
    if (!next) {
      continue
    }
    const previous = previousTargets[key]
    if (!previous) {
      if (!next.ok) {
        emit({
          title: `${next.source}状态异常`,
          detail: next.detail,
          level: "warning",
          source: next.source,
          dedupeKey: `health:${key}:initial:${next.detail}`
        })
      }
      continue
    }

    if (previous.ok && !next.ok) {
      emit({
        title: `${next.source}状态异常`,
        detail: next.detail,
        level: "warning",
        source: next.source,
        dedupeKey: `health:${key}:down:${next.detail}`
      })
      continue
    }

    if (!previous.ok && next.ok) {
      emit({
        title: `${next.source}已恢复`,
        detail: next.detail,
        level: "success",
        source: next.source,
        dedupeKey: `health:${key}:recovered:${next.detail}`
      })
      continue
    }

    if (!next.ok && previous.detail !== next.detail) {
      emit({
        title: `${next.source}状态异常`,
        detail: next.detail,
        level: "warning",
        source: next.source,
        dedupeKey: `health:${key}:changed:${next.detail}`
      })
    }
  }
  return eventCount
}

async function runGlobalHealthCheck({ manual = false } = {}) {
  if (pageState.globalHealthRunning) {
    return
  }
  pageState.globalHealthRunning = true
  pageState.globalHealthSummary.checking = true
  if (manual) {
    renderApp({ preserveInteraction: true })
  }
  try {
    pageState.snapshot = await loadSnapshot()
    pageState.installerState = await loadInstallerStatus()
    await ensureRuntimeDependenciesIfNeeded({ manual })
    pageState.snapshot = await loadSnapshot()
    pageState.installerState = await loadInstallerStatus()

    if (!pageState.snapshot?.backend?.running) {
      await startBackendAction({
        notifySuccess: false,
        notifyFailure: false,
        preserveInteraction: true,
        preserveDrafts: true
      })
      pageState.snapshot = await loadSnapshot()
    } else if (!pageState.snapshot?.backend?.lastHealth?.ok) {
      pageState.snapshot.backend = await invoke("RestartBackendWithRequest", currentBackendRequest())
      const refreshed = await loadSnapshot()
      pageState.snapshot = mergeSnapshot(refreshed, pageState.snapshot.backend)
    }

    const developerSnapshot = await loadDeveloperEnvironmentSnapshot({ ensure: developerIsRunning(), preserveDrafts: true })
    const live = deriveLiveState(pageState.snapshot)
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

    const nextTargets = buildGlobalHealthTargets(live, developerSnapshot, auxiliaryChecks)
    applyHealthNotifications(nextTargets)
    pageState.globalHealthSummary = {
      checking: false,
      lastCheckedAt: new Date().toISOString(),
      targets: nextTargets
    }
    renderApp({ preserveInteraction: true })
  } catch (error) {
    pageState.globalHealthSummary.checking = false
    pushNotification({
      title: "全局健康检查失败",
      detail: String(error),
      level: "error",
      source: "系统"
    })
  } finally {
    pageState.globalHealthRunning = false
    pageState.globalHealthSummary.checking = false
  }
}

function ensureGlobalHealthTicker() {
  if (pageState.globalHealthTicker) {
    return
  }
  pageState.globalHealthTicker = window.setInterval(() => {
    void runGlobalHealthCheck()
  }, GLOBAL_HEALTH_INTERVAL_MS)
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

function normalizeLogLines(lines) {
  if (!Array.isArray(lines) || lines.length === 0) {
    return sampleRawLogs
  }
  return lines.map((line) => `[${line.at || "--:--:--"}] ${line.message || ""}`)
}

function installerLogLines() {
  const lines = pageState.installerState?.logs
  if (!Array.isArray(lines) || lines.length === 0) {
    return sampleRawLogs
  }
  return lines.map((line) => `[${line.at || "--:--:--"}] [${line.source || "adb"}] ${line.message || ""}`)
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
  const codexHealth = health.codex || {}
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
    watchStatusLabel: backendHealthy ? "未安装" : "等待连接",
    watchStatusNote: backendHealthy ? "等待无线 ADB 配对" : "尚未连接到手表",
    accessStatusLabel: accessMode,
    accessStatusNote: publicBaseUrl,
    remoteExposureLabel: managedTunnelSelected
      ? (tunnel.running ? "已连接" : (tunnel.configured ? "待启动" : "未配置"))
      : (publicBaseUrl.startsWith("https://") ? "已启用" : "未启用"),
    networkHealthy: backendHealthy,
    healthConfig,
    codexHealth
  }
}

function topbarStatusItems(snapshot, live) {
  const targets = pageState.globalHealthSummary?.targets || {}
  const items = [
    {
      id: "codex",
      icon: "terminal",
      label: "Codex",
      ok: Boolean(targets.codex?.ok ?? live.codexHealthy)
    },
    {
      id: "backend",
      icon: "server",
      label: "本机服务",
      ok: Boolean(targets.backend?.ok ?? live.backendHealthy)
    },
    {
      id: "resources",
      icon: "document",
      label: "安装资源",
      ok: Boolean(targets.resources?.ok ?? (pageState.installerState?.adb?.available && pageState.installerState?.apk?.available))
    }
  ]

  if (targets.developer) {
    items.push({
      id: "developer",
      icon: "wand",
      label: "开发环境",
      ok: Boolean(targets.developer?.ok)
    })
  }
  if (developerTunnelIsManaged()) {
    items.push({
      id: "developerTunnel",
      icon: "cloud",
      label: "开发隧道",
      ok: Boolean(targets.developerTunnel?.ok ?? developerTunnelStatus()?.running)
    })
  }
  if (targets.publicEntry) {
    items.push({
      id: "public",
      icon: "globe",
      label: "公网入口",
      ok: Boolean(targets.publicEntry.ok)
    })
  }
  if (targets.tunnelEntry) {
    items.push({
      id: "tunnel",
      icon: "cloud",
      label: "托管隧道",
      ok: Boolean(targets.tunnelEntry.ok)
    })
  }
  return items
}

function availableNetworkOptions(snapshot = pageState.snapshot) {
  const context = snapshot.networkContext || fallbackSnapshot.networkContext
  return Array.isArray(context.interfaces) && context.interfaces.length > 0
    ? context.interfaces
    : fallbackSnapshot.networkContext.interfaces
}

function hydrateStateFromSnapshot(snapshot, options = {}) {
  const networkOptions = availableNetworkOptions(snapshot)
  const selectedExists = networkOptions.some((item) => item.ip === pageState.networkForm.selectedIp)
  const usingFallbackIP = pageState.networkForm.selectedIp === fallbackSnapshot.networkContext.recommendedIp
  const preserveSelectedIp = shouldPreserveDraftField("networkForm.selectedIp", options)
  const preserveDevBaseUrl = shouldPreserveDraftField("developerForm.devBaseUrl", options)
  const preserveHostAlias = shouldPreserveDraftField("developerForm.hostAlias", options)

  if (!preserveSelectedIp && (!selectedExists || usingFallbackIP) && networkOptions.length > 0) {
    const preferred = networkOptions.find((item) => item.recommended) || networkOptions[0]
    pageState.networkForm.selectedIp = preferred.ip
    pageState.networkForm.selectedInterface = preferred.label
  } else if (!preserveSelectedIp) {
    const selected = networkOptions.find((item) => item.ip === pageState.networkForm.selectedIp)
    if (selected) {
      pageState.networkForm.selectedInterface = selected.label
    }
  }
  const recommendedDevBaseUrl = suggestedDevBaseUrl()
  if (!preserveDevBaseUrl && (
    !pageState.developerForm.devBaseUrl
    || (
      preferredBackendDevice()?.isEmulator
      && /10\.0\.2\.2:18787$/.test(pageState.developerForm.devBaseUrl.trim())
    )
  )) {
    pageState.developerForm.devBaseUrl = recommendedDevBaseUrl
  }
  if (!preserveHostAlias) {
    pageState.developerForm.hostAlias = currentDeveloperHostAlias()
  }
  if (!preserveDevBaseUrl) {
    pageState.developerForm.accessMode = developerAccessModeFromBaseUrl(pageState.developerForm.devBaseUrl)
    if (pageState.developerForm.accessMode !== "custom") {
      updateDeveloperBaseUrlFromAccessMode(pageState.developerForm.accessMode)
    }
  }
  if (!pageState.developerForm.deviceName) {
    pageState.developerForm.deviceName = pageState.watchForm.deviceName
  }
}

function currentSelectedIP(snapshot = pageState.snapshot) {
  return pageState.networkForm.selectedIp
    || snapshot.networkContext?.recommendedIp
    || fallbackSnapshot.networkContext.recommendedIp
}

function currentListenPort(live) {
  const listen = live?.listen || pageState.snapshot?.backend?.lastHealth?.config?.listen || pageState.snapshot?.backend?.configuredListen || ""
  const resolved = listen.split(":").pop()
  return resolved || pageState.networkForm.port || DEFAULT_SIDECAR_PORT
}

function currentApiBase(live) {
  const port = currentListenPort(live)
  const selectedIP = currentSelectedIP()
  if (pageState.networkMode === "public") {
    return pageState.networkForm.customUrl.trim() || "https://openwatcher.example.com"
  }
  if (pageState.networkMode === "tunnel") {
    return pageState.snapshot?.tunnel?.publicBaseUrl || "https://等待兑换.openwatcher.ai"
  }
  return `http://${selectedIP}:${port}`
}

function currentWatchApiBase(live) {
  const port = currentListenPort(live)
  const device = preferredBackendDevice()
  if (device?.isEmulator && pageState.networkMode === "lan") {
    return `http://${device.hostAlias || "10.0.2.2"}:${port}`
  }
  return currentApiBase(live)
}

function remoteBootstrapDefaultApiBase(live) {
  if (pageState.remoteBootstrapForm.environment === "dev") {
    return developerBaseUrlForCurrentForm() || suggestedDevBaseUrl()
  }
  return currentWatchApiBase(live)
}

function remoteBootstrapHealthLabel(result = pageState.remoteBootstrapForm.result) {
  if (!result) {
    return "尚未提交"
  }
  if (result.health?.ok) {
    return "已通过 /healthz"
  }
  return result.health?.message || "未通过 /healthz"
}

function remoteBootstrapEnvironmentLabel(value = pageState.remoteBootstrapForm.environment) {
  return value === "dev" ? "dev" : "beta"
}

function currentAccessModeLabel() {
  if (pageState.networkMode === "public") {
    return "自定义公网 URL"
  }
  if (pageState.networkMode === "tunnel") {
    return "OpenWatcher 托管隧道"
  }
  return "局域网模式"
}

function currentTunnel(snapshot = pageState.snapshot) {
  return snapshot?.tunnel || fallbackSnapshot.tunnel
}

function tunnelStatusBadge(tunnel = currentTunnel()) {
  if (tunnel.tokenExpired) {
    return { label: "已失效", tone: "red" }
  }
  if (tunnel.running) {
    return { label: "运行中", tone: "green" }
  }
  if (tunnel.configured) {
    return { label: "已绑定", tone: "blue" }
  }
  if (tunnel.lastRedeemErrorCode) {
    return { label: "兑换失败", tone: "red" }
  }
  return { label: "待兑换", tone: "amber" }
}

function setBoundValue(path, value) {
  setValueAtPath(pageState, path, value)
}

function inputValueForBinding(target) {
  if (target?.type === "checkbox") {
    return Boolean(target.checked)
  }
  return target?.value ?? ""
}

function updateBoundDraft(path, target) {
  if (!path) {
    return
  }
  markDraftField(pageState.draftFields, path)
  setBoundValue(path, inputValueForBinding(target))
}

function installerDevices() {
  return Array.isArray(pageState.installerState?.devices) ? pageState.installerState.devices : []
}

function connectedEmulatorDevices() {
  return installerDevices().filter((device) => device.isEmulator && device.state === "device")
}

function singleConnectedEmulator() {
  const devices = connectedEmulatorDevices()
  return devices.length === 1 ? devices[0] : null
}

function installerSelectionRequired() {
  return installerDevices().length > 1 && !pageState.installerState?.selectedSerial
}

function selectedInstallerDevice() {
  const devices = installerDevices()
  const selectedSerial = pageState.installerState?.selectedSerial
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

function wizardStageIndex(stageId) {
  return INSTALL_WIZARD_STAGES.findIndex((stage) => stage.id === stageId)
}

function wizardStageMeta(stageId = pageState.wizard.currentStage) {
  return INSTALL_WIZARD_STAGES.find((stage) => stage.id === stageId) || INSTALL_WIZARD_STAGES[0]
}

function setWizardStage(stageId, note = "") {
  pageState.wizard.currentStage = stageId
  pageState.wizard.maxUnlockedIndex = Math.max(pageState.wizard.maxUnlockedIndex, wizardStageIndex(stageId))
  if (note) {
    pageState.wizard.stageNotes[stageId] = note
  }
}

function markWizardStageCompleted(stageId, note = "") {
  if (!pageState.wizard.completedStages.includes(stageId)) {
    pageState.wizard.completedStages.push(stageId)
  }
  if (note) {
    pageState.wizard.stageNotes[stageId] = note
  }
}

function wizardStageCompleted(stageId) {
  return pageState.wizard.completedStages.includes(stageId)
}

function wizardStageUnlocked(stageId) {
  return wizardStageIndex(stageId) <= pageState.wizard.maxUnlockedIndex
}

function wizardCurrentStageError() {
  return pageState.wizard.error?.stage === pageState.wizard.currentStage ? pageState.wizard.error : null
}

function clearWizardStageError() {
  pageState.wizard.error = null
}

function setWizardStageError(stageId, message) {
  pageState.wizard.error = {
    stage: stageId,
    message: message || "当前步骤执行失败。"
  }
  if (message) {
    pageState.wizard.stageNotes[stageId] = message
  }
}

function pairingSkipReason() {
  const emulator = singleConnectedEmulator()
  if (!emulator) {
    return ""
  }
  return `检测到已连接模拟器 ${emulator.serial}，已跳过无线配对。`
}

function syncInstallWizardDraft(snapshot = pageState.snapshot) {
  const nextSuggestedLanURL = suggestedLanURL(snapshot)
  const lanEntry = pageState.wizard.configEntries.lan
  if (!lanEntry.url || lanEntry.url === pageState.wizard.suggestedLanURL) {
    lanEntry.url = nextSuggestedLanURL
  }
  pageState.wizard.suggestedLanURL = nextSuggestedLanURL

  const tunnelDomain = snapshot?.tunnel?.publicBaseUrl || ""
  if (tunnelDomain) {
    pageState.wizard.configEntries.tunnel.redeemedDomain = tunnelDomain
    if (pageState.wizard.configEntries.tunnel.validation === "pending") {
      pageState.wizard.configEntries.tunnel.validation = "idle"
    }
    if (!pageState.wizard.configEntries.tunnel.message || pageState.wizard.configEntries.tunnel.message === "先输入配置码再兑换") {
      pageState.wizard.configEntries.tunnel.message = "已兑换，可继续校验"
    }
  }

  if (!pageState.installForm.useSeparateConnectIP) {
    pageState.installForm.connectIp = pageState.installForm.pairIp
  }
}

function syncWizardWithInstallerState() {
  syncInstallWizardDraft(pageState.snapshot)
  if (pageState.installerState?.phase === "troubleshooting" && pageState.wizard.currentStage !== "prepare") {
    setWizardStageError(pageState.wizard.currentStage, pageState.installerState?.message || "当前步骤执行失败。")
  } else if (pageState.wizard.error && pageState.installerState?.phase !== "troubleshooting") {
    clearWizardStageError()
  }

  if (pairingSkipReason() && pageState.wizard.currentStage === "connect" && selectedInstallerDevice()) {
    pageState.wizard.stageNotes.connect = pairingSkipReason()
  }

  if (selectedInstallerDevice()) {
    pageState.wizard.stageNotes.connect = pageState.installerState?.message || `${selectedInstallerDevice().displayName || selectedInstallerDevice().serial} 已准备好继续安装。`
  }

  if (pageState.installerState?.apk?.installed && !wizardStageCompleted("install")) {
    markWizardStageCompleted("connect", pageState.wizard.stageNotes.connect || "设备已连接")
  }
}

function wizardAutoChecks(live = deriveLiveState(pageState.snapshot)) {
  const installerState = pageState.installerState || fallbackInstallerState
  const apkName = installerState.apk?.path
    ? installerState.apk.path.split("/").slice(-1)[0]
    : (installerState.apk?.label || "未检测到安装包")

  return [
    {
      id: "tool",
      label: "安装工具可用",
      ok: Boolean(installerState.adb?.available),
      tag: installerState.adb?.available ? "已就绪" : "未就绪",
      detail: installerState.adb?.available
        ? ["已缓存到本机", `版本 ${installerState.adb?.version || "已找到"}`]
        : [installerState.adb?.message || "未检测到安装工具"]
    },
    {
      id: "package",
      label: "手表安装包可用",
      ok: Boolean(installerState.apk?.available),
      tag: installerState.apk?.available ? "已就绪" : "未就绪",
      detail: installerState.apk?.available
        ? [
          `文件 ${apkName}`,
          `版本 ${installerState.apk?.versionName || "未知版本"} · ${installerState.apk?.debug ? "debug" : "release"}`
        ]
        : [installerState.apk?.message || "未检测到安装包"]
    }
  ]
}

function wizardManualChecksDone() {
  return PREPARE_MANUAL_CHECKS.every((item) => Boolean(pageState.wizard.manualChecks[item.id]))
}

function toggleAllWizardManualChecks() {
  const nextValue = !wizardManualChecksDone()
  for (const item of PREPARE_MANUAL_CHECKS) {
    pageState.wizard.manualChecks[item.id] = nextValue
  }
}

function wizardPrepareReady(live = deriveLiveState(pageState.snapshot)) {
  return wizardAutoChecks(live).every((item) => item.ok) && wizardManualChecksDone()
}

function wizardConnectReady() {
  return Boolean(selectedInstallerDevice()) && !installerSelectionRequired()
}

function wizardSelectedGuide() {
  return INSTALL_GUIDES.find((guide) => guide.id === pageState.wizard.guide.selectedBrand) || INSTALL_GUIDES[0]
}

function wizardGuideStep() {
  const guide = wizardSelectedGuide()
  return guide.steps[pageState.wizard.guide.stepIndex] || guide.steps[0]
}

function wizardConfigEntry(entryId) {
  return pageState.wizard.configEntries[entryId]
}

function wizardEnabledConfigEntries() {
  return Object.entries(pageState.wizard.configEntries).filter(([, entry]) => entry.enabled)
}

function wizardConfigChecksInProgress() {
  return wizardEnabledConfigEntries().some(([, entry]) => entry.validation === "checking")
}

function wizardConfigCheckingLabel() {
  const active = wizardEnabledConfigEntries().find(([, entry]) => entry.validation === "checking")
  return active ? INSTALL_CONFIG_META[active[0]].label : ""
}

function wizardInstallReady() {
  return Boolean(wizardStageCompleted("install") || pageState.installerState?.apk?.installed)
}

function configWriteActionLabel() {
  return pageState.generatedBootstrap?.apiBase ? "重新写入" : "写入配置"
}

function wizardConfigReady() {
  return Boolean(wizardStageCompleted("config") || pageState.generatedBootstrap?.apiBase)
}

function buildEnabledBootstrapEndpoints() {
  const selectedDevice = preferredBackendDevice()
  const isEmulator = Boolean(selectedDevice?.isEmulator)
  const emulatorHostAlias = selectedDevice?.hostAlias || "10.0.2.2"
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
  const message = String(pageState.installerState?.message || "").toLowerCase()
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
  return [
    "请重试当前步骤。",
    "如果问题重复出现，导出诊断包后再继续排查。"
  ]
}

function currentBackendRequest() {
  const selectedDevice = preferredBackendDevice()
  const isEmulator = Boolean(selectedDevice?.isEmulator)
  const emulatorHostAlias = selectedDevice?.hostAlias || "10.0.2.2"
  const port = pageState.networkForm.port
  return {
    mode: pageState.networkMode,
    selectedIP: isEmulator && pageState.networkMode === "lan" ? "127.0.0.1" : currentSelectedIP(),
    bindAll: pageState.networkForm.bindAll,
    port,
    customURL: pageState.networkForm.customUrl,
    tunnelCode: pageState.networkForm.tunnelCode,
    deviceName: pageState.watchForm.deviceName,
    publicBaseURL: isEmulator && pageState.networkMode === "lan" ? `http://${emulatorHostAlias}:${port || "8787"}` : "",
    endpoints: buildEnabledBootstrapEndpoints()
  }
}

function installerStatusTone() {
  if (!pageState.installerState?.adb?.available) {
    return "amber"
  }
  return selectedInstallerDevice() || installerSelectionRequired() ? "green" : "amber"
}

function installerStatusLabel() {
  const device = selectedInstallerDevice()
  if (device) {
    return pageState.installerState.apk?.available ? "已连接 / 可安装" : "已连接"
  }
  return pageState.installerState?.adb?.available ? "等待连接" : "ADB 不可用"
}

function installerStatusNote() {
  const state = pageState.installerState || fallbackInstallerState
  const device = selectedInstallerDevice()
  if (!state.adb?.available) {
    return state.adb?.message || "未检测到 ADB"
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
  return state.message || "等待无线 ADB 配对或现有模拟器设备"
}

function installerSummaryNote() {
  const state = pageState.installerState || fallbackInstallerState
  if (state.message) {
    return state.message
  }
  if (state.apk?.devFallback) {
    return "当前仅检测到 debug APK。它只用于开发或模拟器验证，真实设备与公开发布请改用 release 包。"
  }
  if (state.apk?.available) {
    return `已检测安装包：${state.apk.versionName || "未知版本"}`
  }
  return state.apk?.message || "尚未找到可安装的手表 APK。"
}

function renderBootstrapStatus() {
  if (!pageState.generatedBootstrap) {
    return ""
  }
  return `
    <p class="support-note">
      最近一次已生成配置链接：${escapeHtml(pageState.generatedBootstrap.deviceName)} · token 指纹 ${escapeHtml(pageState.generatedBootstrap.tokenFingerprint)} · ${escapeHtml(pageState.generatedBootstrap.apiBase)}
    </p>
  `
}

function renderRemoteWatchBootstrapPanel(live) {
  const form = pageState.remoteBootstrapForm
  const result = form.result
  const defaultApiBase = remoteBootstrapDefaultApiBase(live)
  const busy = Boolean(form.submitting)
  return `
    <article class="panel-card">
      <h3>远程初始化</h3>
      <p class="support-note">手表和电脑不在同一网络时，在手表上读取临时配置码，然后在这里发送 API 基址。device token 仍只在手表侧生成。</p>
      <div class="install-form-grid">
        <label class="field">
          <span>临时配置码</span>
          <input value="${escapeHtml(form.bootstrapCode)}" data-bind="remoteBootstrapForm.bootstrapCode" placeholder="例如 AB12CD34" ${busy ? "disabled" : ""} />
        </label>
        <label class="field">
          <span>环境类型</span>
          <select data-bind="remoteBootstrapForm.environment" ${busy ? "disabled" : ""}>
            <option value="beta" ${form.environment === "beta" ? "selected" : ""}>beta</option>
            <option value="dev" ${form.environment === "dev" ? "selected" : ""}>dev</option>
          </select>
        </label>
        <label class="field">
          <span>API 基址</span>
          <input value="${escapeHtml(form.apiBase)}" data-bind="remoteBootstrapForm.apiBase" placeholder="${escapeHtml(defaultApiBase)}" ${busy ? "disabled" : ""} />
        </label>
        <label class="field">
          <span>隧道配置码（可选）</span>
          <input value="${escapeHtml(form.tunnelCode)}" data-bind="remoteBootstrapForm.tunnelCode" placeholder="留空则使用上面的 API 基址" ${busy ? "disabled" : ""} />
        </label>
      </div>
      <div class="button-row">
        <button class="primary-btn" data-command="submit-remote-watch-bootstrap" ${busy ? "disabled" : ""}>${busy ? `<span class="install-spinner" aria-hidden="true"></span>` : icon("link")}发送到手表临时配置</button>
        <button class="secondary-btn" data-command="fill-remote-watch-api-base" ${busy ? "disabled" : ""}>${icon("refresh")}填入当前建议地址</button>
      </div>
      <div class="info-table">
        ${[
          ["环境", result ? remoteBootstrapEnvironmentLabel(result.environment) : remoteBootstrapEnvironmentLabel(form.environment)],
          ["API 基址", result?.apiBase || form.apiBase || defaultApiBase],
          ["健康检查", remoteBootstrapHealthLabel(result)],
          ["提交时间", result?.submittedAt || "尚未提交"]
        ].map(([label, value]) => `<div class="info-row"><span>${label}</span><strong>${escapeHtml(value)}</strong></div>`).join("")}
      </div>
      ${result?.message ? `<p class="support-note">${escapeHtml(result.message)}</p>` : ""}
    </article>
  `
}

function filteredTimeline() {
  const dynamicTimeline = Array.isArray(pageState.installerState?.logs) && pageState.installerState.logs.length > 0
    ? pageState.installerState.logs.slice(-8).map((line) => ({
      time: String(line.at || "").slice(11, 19) || "--:--:--",
      event: line.message || "",
      level: /fail|error|异常|失败/i.test(line.message || "") ? "ERROR" : "INFO",
      source: line.source === "adb" ? "ADB 安装" : line.source
    }))
    : sampleEventTimeline
  if (pageState.selectedLogSource === "all") {
    return dynamicTimeline
  }
  return dynamicTimeline.filter((item) => item.source === pageState.selectedLogSource)
}

function filteredRawLogs() {
  const rawLogs = [...installerLogLines(), ...developerLogLines(), ...normalizeLogLines(pageState.backendLogs)]
  if (pageState.selectedLogSource === "all") {
    return rawLogs
  }
  const sourceMap = {
    "Desktop": "[desktop]",
    "本机服务": "[backend]",
    "开发环境": "[developer]",
    "ADB 安装": "[adb]",
    "手表 App": "[watch]",
    "网络访问": "[network]",
    "托管隧道": "[tunnel]",
    "更新": "[update]",
    "安全": "[security]"
  }
  const token = sourceMap[pageState.selectedLogSource]
  if (!token) {
    return rawLogs
  }
  const matched = rawLogs.filter((line) => line.toLowerCase().includes(token))
  return matched.length > 0 ? matched : rawLogs
}

function icon(name) {
  const map = {
    wand: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 19.5 13.5 10m-6 9.5L5 17l8.5-8.5 2.5 2.5L7.5 19.5Zm9-12L18 4m0 0 1.4 3.1L22.5 8.5 19.4 9.9 18 13l-1.4-3.1L13.5 8.5l3.1-1.4L18 4Z"/></svg>`,
    watch: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M9 2h6l1 3h1.5A2.5 2.5 0 0 1 20 7.5v9A2.5 2.5 0 0 1 17.5 19H16l-1 3H9l-1-3H6.5A2.5 2.5 0 0 1 4 16.5v-9A2.5 2.5 0 0 1 6.5 5H8l1-3Zm-1 6v8h8V8H8Z"/></svg>`,
    globe: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18Zm0 0c2.2 2.1 3.5 5.6 3.5 9S14.2 18.9 12 21m0-18C9.8 5.1 8.5 8.6 8.5 12S9.8 18.9 12 21m-8-9h16M4.7 8h14.6M4.7 16h14.6"/></svg>`,
    document: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 3h7l5 5v13H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2Zm7 1.5V9h4.5M9 12h6M9 16h6"/></svg>`,
    gear: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m12 3 1.3 2.7 3 .4.9 2.8 2.7 1.3-.4 3 1.8 2.4-1.8 2.4.4 3-2.7 1.3-.9 2.8-3 .4L12 21l-1.3-2.7-3-.4-.9-2.8-2.7-1.3.4-3L2.7 12l1.8-2.4-.4-3L6.8 5.3l.9-2.8 3-.4L12 3Zm0 5a4 4 0 1 0 0 8 4 4 0 0 0 0-8Z"/></svg>`,
    help: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 21a9 9 0 1 1 0-18 9 9 0 0 1 0 18Zm0-5h.01M9.5 9.5a2.5 2.5 0 1 1 4 2c-.8.6-1.5 1.1-1.5 2.5"/></svg>`,
    bell: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 4a4 4 0 0 0-4 4v2.2c0 .9-.3 1.7-.9 2.4L5 15h14l-2.1-2.4a3.6 3.6 0 0 1-.9-2.4V8a4 4 0 0 0-4-4Zm0 16a2.5 2.5 0 0 1-2.4-2h4.8A2.5 2.5 0 0 1 12 20Z"/></svg>`,
    mode: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3a9 9 0 1 0 9 9 7 7 0 1 1-9-9Z"/></svg>`,
    folder: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 7.5A2.5 2.5 0 0 1 5.5 5H10l2 2h6.5A2.5 2.5 0 0 1 21 9.5v7A2.5 2.5 0 0 1 18.5 19h-13A2.5 2.5 0 0 1 3 16.5v-9Z"/></svg>`,
    server: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 6.5A2.5 2.5 0 0 1 6.5 4h11A2.5 2.5 0 0 1 20 6.5v2A2.5 2.5 0 0 1 17.5 11h-11A2.5 2.5 0 0 1 4 8.5v-2Zm0 9A2.5 2.5 0 0 1 6.5 13h11A2.5 2.5 0 0 1 20 15.5v2a2.5 2.5 0 0 1-2.5 2.5h-11A2.5 2.5 0 0 1 4 17.5v-2ZM8 7.5h.01M8 16.5h.01"/></svg>`,
    shield: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 5 6v5c0 4.6 2.8 7.8 7 10 4.2-2.2 7-5.4 7-10V6l-7-3Z"/></svg>`,
    cloud: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 18a4 4 0 0 1-.3-8A5.5 5.5 0 0 1 18 9.5 3.5 3.5 0 1 1 18.5 18H8Zm4-8v6m0 0-2.5-2.5M12 16l2.5-2.5"/></svg>`,
    terminal: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 8 4 4-4 4m6 0h6M4.5 5h15A1.5 1.5 0 0 1 21 6.5v11a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 17.5v-11A1.5 1.5 0 0 1 4.5 5Z"/></svg>`,
    refresh: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6v5h-5m4.2 4a8 8 0 1 1 0-6"/></svg>`,
    play: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m8 6 10 6-10 6z"/></svg>`,
    check: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m5 12 4.2 4.2L19 6.5"/></svg>`,
    close: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 6 18 18M18 6 6 18"/></svg>`,
    warning: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 4 3.5 19h17L12 4Zm0 5v4m0 3h.01"/></svg>`,
    sun: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 4v2m0 12v2m8-8h-2M6 12H4m12.66 5.66-1.41-1.41M8.75 8.75 7.34 7.34m9.32 0-1.41 1.41M8.75 15.25l-1.41 1.41M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Z"/></svg>`,
    trash: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 7h14m-9-3h4m-7 3 1 12h8l1-12M10 11v4m4-4v4"/></svg>`,
    copy: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M9 9h10v11H9zM5 5h10v2H7v10H5z"/></svg>`,
    link: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M10 14 8 16a3 3 0 1 1-4-4l3-3a3 3 0 0 1 4 0m3 1 2-2a3 3 0 1 0-4-4l-1 1m-1 4 6-6"/></svg>`
  }
  return `<span class="icon icon-${name}">${map[name] || map.document}</span>`
}

function badge(text, tone = "neutral") {
  return `<span class="badge badge-${tone}">${text}</span>`
}

function statusDot(text, tone = "neutral") {
  return `<span class="status-dot status-dot-${tone}"><span class="status-dot-core"></span>${text}</span>`
}

function topStatusCard(card) {
  return `
    <article class="top-card">
      <div class="top-card-icon tone-${card.tone}">${icon(card.icon)}</div>
      <div class="top-card-body">
        <p class="top-card-title">${card.title}</p>
        <h3 class="top-card-value tone-text-${card.tone}">${card.value}</h3>
        <p class="top-card-meta">${card.meta}</p>
      </div>
    </article>
  `
}

function appLogo() {
  return `
    <div class="brand">
      <span class="brand-mark">
        <img src="${launcherIconUrl}" alt="OpenWatcher" />
      </span>
      <span class="brand-text">OpenWatcher</span>
    </div>
  `
}

function renderTopBar(snapshot, live) {
  const statuses = topbarStatusItems(snapshot, live)
  const themeTitle = pageState.theme === "dark" ? "切换浅色" : "切换深色"
  const themeIcon = pageState.theme === "dark" ? "sun" : "mode"
  const unreadCount = unreadNotificationCount()
  return `
    <header class="topbar" data-topbar>
      <div class="topbar-left">
        ${appLogo()}
        ${badge(snapshot.productVersion || "dev", "ghost")}
      </div>
      <div class="topbar-center">
        ${statuses.map((item) => `
          <div class="topbar-status-chip ${item.ok ? "is-ok" : "is-idle"}">
            <span class="topbar-status-icon">${icon(item.icon)}</span>
            <span class="topbar-status-label">${item.label}</span>
          </div>
        `).join("")}
      </div>
      <div class="topbar-right">
        <button class="ghost-chip icon-only" data-command="run-global-health-check" title="立即刷新健康检查" aria-label="立即刷新健康检查">${icon("refresh")}</button>
        <button class="ghost-chip icon-only" data-command="toast" data-value="帮助 暂未接入。" title="帮助" aria-label="帮助">${icon("help")}</button>
        <button class="ghost-chip icon-only has-badge" data-command="toggle-notification-panel" title="通知" aria-label="通知">
          ${icon("bell")}
          ${unreadCount > 0 ? `<span class="icon-badge">${unreadCount > 99 ? "99+" : unreadCount}</span>` : ""}
        </button>
        <button class="mode-chip icon-only" data-command="toggle-theme" title="${themeTitle}" aria-label="${themeTitle}">${icon(themeIcon)}</button>
      </div>
    </header>
    ${renderNotificationPanel()}
  `
}

function renderNotificationPanel() {
  if (!pageState.notificationPanelOpen) {
    return ""
  }
  return `
    <div class="notification-panel">
      <div class="notification-panel-head">
        <div>
          <strong>通知中心</strong>
          <p>统一显示健康检查、自动修复和用户操作结果。</p>
        </div>
        <button class="ghost-chip icon-only" data-command="close-notification-panel" title="关闭通知" aria-label="关闭通知">✕</button>
      </div>
      <div class="notification-panel-body">
        ${pageState.notifications.length === 0 ? `
          <div class="notification-empty">当前还没有通知事件。</div>
        ` : pageState.notifications.map((item) => `
          <article class="notification-item tone-${notificationLevelTone(item.level)} ${item.read ? "" : "is-unread"}">
            <div class="notification-item-head">
              <strong>${escapeHtml(item.title)}</strong>
              <span>${escapeHtml(item.timeLabel)}</span>
            </div>
            <p>${escapeHtml(item.detail || item.source)}</p>
            <small>${escapeHtml(item.source)}</small>
          </article>
        `).join("")}
      </div>
    </div>
  `
}

function renderDeveloperConfirmModal() {
  if (!pageState.developerConfirmModalOpen) {
    return ""
  }
  return `
    <div class="install-guide-modal" data-command="developer-confirm-cancel">
      <div class="install-guide-dialog developer-confirm-dialog" data-stop-click="true">
        <div class="install-guide-head">
          <div>
            <h3>开发环境使用说明</h3>
            <p>这个页面只管理当前仓库的本机开发服务、开发隧道和发送到手表的调试入口。</p>
          </div>
          <button class="install-guide-close" data-command="developer-confirm-cancel">✕</button>
        </div>
        <div class="developer-confirm-body">
          <article class="developer-confirm-card">
            <strong>这个页面能做什么</strong>
            <ul>
              <li>选择当前仓库目录，查看服务地址、Healthz 地址和启动脚本。</li>
              <li>启动、停止或重新启动当前仓库的本机开发环境。</li>
              <li>激活开发隧道，并把当前开发环境发送到手表确认。</li>
            </ul>
          </article>
          <article class="developer-confirm-card">
            <strong>状态是怎么判断的</strong>
            <ul>
              <li>如果 Desktop 自己启动了服务，会记录启动时间并持续刷新日志。</li>
              <li>如果你已经手动启动了本机开发服务，这里也会直接识别为“运行中”。</li>
              <li>服务异常或启动失败时，可直接在页面查看最近日志并重新启动。</li>
            </ul>
          </article>
          <article class="developer-confirm-card tone-warn">
            <strong>需要注意</strong>
            <ul>
              <li>不要把未准备好的地址发送到手表。</li>
              <li>如果当前端口上已有手动启动的服务，停止环境会一并结束该端口对应的进程。</li>
              <li>激活开发隧道后，选择“开发隧道”访问方式才会把它作为当前 Base URL。</li>
            </ul>
          </article>
        </div>
        <div class="install-guide-foot">
          <button class="primary-btn" data-command="developer-confirm-cancel">我知道了</button>
        </div>
      </div>
    </div>
  `
}

function renderSidebar() {
  return `
    <aside class="sidebar">
      <nav class="nav-list">
        ${NAV_ITEMS.map((item) => `
          <button class="nav-item ${pageState.currentPage === item.id ? "is-active" : ""}" data-nav="${item.id}">
            ${icon(item.icon)}
            <span>${item.label}</span>
          </button>
        `).join("")}
      </nav>
    </aside>
  `
}

function renderWizardFailureNotice() {
  const failure = wizardCurrentStageError()
  if (!failure) {
    return ""
  }
  const pairedConflict = String(failure.message || "").includes("已存在配对信息")
  return `
    <div class="install-callout install-callout-error">
      <strong>${escapeHtml(failure.message)}</strong>
      <p>${escapeHtml(wizardFailureTips()[0])}</p>
      ${pairedConflict ? `
        <div class="button-row compact-right">
          <button class="secondary-btn small" data-command="go-clear-pairing">${icon("trash")}去清空</button>
        </div>
      ` : ""}
    </div>
  `
}

function renderInstallGuideModal() {
  const guide = wizardSelectedGuide()
  const step = wizardGuideStep()
  if (!pageState.wizard.guide.modalOpen || !guide || !step) {
    return ""
  }
  return `
    <div class="install-guide-modal" data-command="wizard-close-guide">
      <div class="install-guide-dialog" data-stop-click="true">
        <div class="install-guide-head">
          <div>
            <h3>${escapeHtml(guide.title)}</h3>
            <p>${escapeHtml(guide.subtitle)}</p>
          </div>
          <button class="install-guide-close" data-command="wizard-close-guide">✕</button>
        </div>
        <div class="install-guide-body">
          <div class="install-guide-visual">
            <div class="install-guide-frame">
              <div class="install-guide-window">
                <div class="install-guide-window-top"><span></span><span></span><span></span></div>
                <div class="install-guide-window-body">${escapeHtml(step.visual)}</div>
              </div>
            </div>
            <div class="install-guide-progress">
              <span>步骤 ${pageState.wizard.guide.stepIndex + 1} / ${guide.steps.length}</span>
              <div class="install-guide-dots">
                ${guide.steps.map((_, index) => `<span class="install-guide-dot ${index === pageState.wizard.guide.stepIndex ? "is-active" : ""}"></span>`).join("")}
              </div>
            </div>
          </div>
          <div class="install-guide-copy">
            <strong>${escapeHtml(step.title)}</strong>
            <p>${escapeHtml(step.body)}</p>
            <div class="install-guide-bullets">
              ${step.bullets.map((item, index) => `
                <div class="install-guide-bullet">
                  <span class="install-guide-bullet-index">${index + 1}</span>
                  <span>${escapeHtml(item)}</span>
                </div>
              `).join("")}
            </div>
          </div>
        </div>
        <div class="install-guide-foot">
          <button class="secondary-btn" data-command="wizard-guide-prev" ${pageState.wizard.guide.stepIndex === 0 ? "disabled" : ""}>上一步</button>
          <div class="install-guide-foot-actions">
            <button class="secondary-btn" data-command="wizard-guide-next" ${pageState.wizard.guide.stepIndex >= guide.steps.length - 1 ? "disabled" : ""}>下一步</button>
            <button class="primary-btn" data-command="wizard-close-guide">我知道怎么设置了</button>
          </div>
        </div>
      </div>
    </div>
  `
}

function renderInstallStageTabs() {
  return `
    <div class="install-stage-tabs">
      ${INSTALL_WIZARD_STAGES.map((stage, index) => {
        const current = stage.id === pageState.wizard.currentStage
        const done = wizardStageCompleted(stage.id)
        return `
          <button
            class="install-stage-tab ${current ? "is-current" : ""} ${done ? "is-done" : ""}"
            data-command="wizard-go-stage"
            data-value="${stage.id}"
            ${wizardStageUnlocked(stage.id) ? "" : "disabled"}
          >
            <span class="install-stage-index">${index + 1}</span>
            <span class="install-stage-copy">
              <strong>${escapeHtml(stage.title)}</strong>
              <span>${escapeHtml(stage.navSummary)}</span>
            </span>
          </button>
        `
      }).join("")}
    </div>
  `
}

function renderPrepareStage(live) {
  const autoChecks = wizardAutoChecks(live)
  const guide = wizardSelectedGuide()
  return `
    <div class="install-stage-layout prepare">
      <article class="install-card">
        <div class="install-card-head">
          <div>
            <h3>准备清单</h3>
            <p>自动项只会刷新检测结果，手动勾选会保留。</p>
          </div>
          <span class="install-status-chip soft">${escapeHtml(pageState.wizard.lastRefreshLabel)}</span>
        </div>
        ${renderWizardFailureNotice()}
        <div class="install-check-section">
          <div class="install-section-caption">自动检查</div>
          <div class="install-check-grid">
            ${autoChecks.map((item) => `
              <div class="install-check-item">
                <div class="install-check-top">
                  <span class="install-check-icon ${item.ok ? "ok" : "warn"}">${item.ok ? "✓" : "!"}</span>
                  <strong>${escapeHtml(item.label)}</strong>
                  <span class="install-status-chip ${item.ok ? "ok" : "warn"}">${escapeHtml(item.tag)}</span>
                </div>
                <div class="install-check-notes">
                  ${item.detail.map((detail) => `<span>${escapeHtml(detail)}</span>`).join("")}
                </div>
              </div>
            `).join("")}
          </div>
        </div>
        <div class="install-check-section">
          <div class="install-section-head">
            <div class="install-section-caption">手动确认</div>
            <button class="install-check-all ${wizardManualChecksDone() ? "is-checked" : ""}" data-command="wizard-toggle-all-manual-checks">
              <span class="install-check-all-box">${wizardManualChecksDone() ? "✓" : ""}</span>
              <span>全选</span>
            </button>
          </div>
          <div class="install-check-grid">
            ${PREPARE_MANUAL_CHECKS.map((item) => {
              const checked = Boolean(pageState.wizard.manualChecks[item.id])
              return `
                <button class="install-check-item install-check-action ${checked ? "is-checked" : ""}" data-command="wizard-toggle-manual-check" data-value="${item.id}">
                  <div class="install-check-top">
                    <span class="install-check-icon ${checked ? "ok" : "todo"}">${checked ? "✓" : "☐"}</span>
                    <strong>${escapeHtml(item.label)}</strong>
                    <span class="install-status-chip ${checked ? "ok" : "warn"}">${checked ? "已确认" : "待确认"}</span>
                  </div>
                </button>
              `
            }).join("")}
          </div>
        </div>
      </article>
      <article class="install-card">
        <div class="install-card-head">
          <div>
            <h3>品牌 / 型号</h3>
            <p>先选你的设备，再查看对应的图文教程。</p>
          </div>
        </div>
        <div class="install-guide-grid">
          ${INSTALL_GUIDES.map((item) => `
            <button class="install-guide-pill ${item.id === guide.id ? "is-active" : ""}" data-command="wizard-open-guide" data-value="${item.id}">
              <span class="install-guide-pill-icon">${item.icon}</span>
              <span class="install-guide-pill-copy">
                <strong>${escapeHtml(item.label)}</strong>
                <small>${escapeHtml(item.models)}</small>
              </span>
              <span class="install-guide-pill-link">查看教程</span>
            </button>
          `).join("")}
        </div>
      </article>
    </div>
  `
}

function renderConnectStage() {
  const devices = installerDevices()
  const selectedDevice = selectedInstallerDevice()
  const separateIP = pageState.installForm.useSeparateConnectIP
  const connectionSummary = selectedDevice
    ? `${selectedDevice.displayName || selectedDevice.serial} 已连接`
    : (pageState.installerState?.message || "填写手表上的信息后继续")

  return `
    <div class="install-stage-layout connect">
      <article class="install-card">
        <div class="install-card-head">
          <div>
            <h3>填写手表信息</h3>
            <p>把手表页面上的地址、端口和配对码填进来。</p>
          </div>
          <span class="install-status-chip ${wizardConnectReady() ? "ok" : "soft"}">${wizardConnectReady() ? "已就绪" : "等待连接"}</span>
        </div>
        ${renderWizardFailureNotice()}
        ${pairingSkipReason() ? `
          <div class="install-callout">
            <strong>已检测到可用设备</strong>
            <p>${escapeHtml(pairingSkipReason())}</p>
          </div>
        ` : ""}
        <div class="install-form-grid">
          <label class="field">
            <span>手表 IP</span>
            <input value="${escapeHtml(pageState.installForm.pairIp)}" placeholder="例如 192.168.31.88" data-bind="installForm.pairIp" />
          </label>
          <label class="field">
            <span>配对端口</span>
            <input value="${escapeHtml(pageState.installForm.pairPort)}" placeholder="例如 37153" data-bind="installForm.pairPort" />
          </label>
          <label class="field">
            <span>连接端口</span>
            <input value="${escapeHtml(pageState.installForm.connectPort)}" placeholder="例如 40221" data-bind="installForm.connectPort" />
          </label>
          <label class="field">
            <span>配对码</span>
            <input value="${escapeHtml(pageState.installForm.pairingCode)}" placeholder="六位配对码" data-bind="installForm.pairingCode" />
          </label>
        </div>
        <div class="install-inline-actions">
          <button class="secondary-btn small" data-command="wizard-toggle-advanced-connect">${separateIP ? "收起高级选项" : "高级选项"}</button>
        </div>
        ${separateIP ? `
          <div class="install-advanced-grid">
            <label class="field">
              <span>单独的连接地址</span>
              <input value="${escapeHtml(pageState.installForm.connectIp)}" placeholder="只有少数设备需要填写" data-bind="installForm.connectIp" />
            </label>
          </div>
        ` : ""}
        <div class="install-callout">
          <strong>默认只用一个地址</strong>
          <p>大多数设备只需要填写一个手表 IP。只有少数设备需要单独填写连接地址。</p>
        </div>
      </article>
      <article class="install-card">
        <div class="install-card-head">
          <div>
            <h3>连接进展</h3>
            <p>只显示这一步需要你确认的内容。</p>
          </div>
        </div>
        <div class="install-result-list">
          <div class="install-result-item">
            <strong>当前状态 <span class="install-status-chip ${wizardConnectReady() ? "ok" : "soft"}">${wizardConnectReady() ? "可继续" : "进行中"}</span></strong>
            <span>${escapeHtml(connectionSummary)}</span>
          </div>
          <div class="install-result-item">
            <strong>目标设备 <span class="install-status-chip ${selectedDevice ? "ok" : "warn"}">${selectedDevice ? "已确认" : "待确认"}</span></strong>
            <span>${escapeHtml(selectedDevice ? `${selectedDevice.displayName || selectedDevice.serial} · ${selectedDevice.serial}` : (installerSelectionRequired() ? "检测到多个设备，请在下面点选。" : "连接成功后会自动确认。"))}</span>
          </div>
          <div class="install-result-item">
            <strong>最近结果 <span class="install-status-chip soft">已脱敏</span></strong>
            <span>${escapeHtml((pageState.installerState?.logs || []).slice(-1)[0]?.message || pageState.installerState?.message || "还没有新的结果。")}</span>
          </div>
        </div>
        ${devices.length > 0 ? `
          <div class="install-device-grid">
            ${devices.map((device) => `
              <button class="install-device-card ${device.serial === pageState.installerState?.selectedSerial || (!pageState.installerState?.selectedSerial && selectedDevice?.serial === device.serial) ? "is-active" : ""}" data-command="wizard-select-device" data-value="${device.serial}">
                <strong>${escapeHtml(device.displayName || device.serial)}</strong>
                <span>${escapeHtml(device.serial)}</span>
                <em>${escapeHtml(device.isEmulator ? "模拟器" : (device.isWatch ? "手表" : "其他设备"))} · ${escapeHtml(device.state)}</em>
              </button>
            `).join("")}
          </div>
        ` : `
          <div class="install-callout">
            <strong>等待设备出现</strong>
            <p>完成连接后，这里会显示这次可安装的设备。</p>
          </div>
        `}
        <div class="button-row compact-right">
          <button class="primary-btn" data-command="wizard-run-connect">${icon("watch")}开始连接</button>
        </div>
      </article>
    </div>
  `
}

function renderInstallStage() {
  const installerState = pageState.installerState || fallbackInstallerState
  const selectedDevice = selectedInstallerDevice()
  const installActionLabel = installerState.apk?.installed ? "打开应用" : "开始安装"
  const packageSource = installerState.apk?.path
    ? installerState.apk.path.split("/").slice(-1)[0]
    : "未检测到安装包"

  return `
    <div class="install-stage-layout install">
      <article class="install-card">
        <div class="install-card-head">
          <div>
            <h3>安装目标</h3>
            <p>确认目标设备和安装包后，手动执行安装或打开应用。</p>
          </div>
        </div>
        ${renderWizardFailureNotice()}
        <div class="install-summary-grid">
          <div class="install-summary-box">
            <h4>目标设备</h4>
            <div class="install-summary-list">
              <div class="install-summary-row"><span>设备名称</span><strong>${escapeHtml(selectedDevice?.displayName || "未选择")}</strong></div>
              <div class="install-summary-row"><span>设备标识</span><strong>${escapeHtml(selectedDevice?.serial || "未连接")}</strong></div>
              <div class="install-summary-row"><span>设备类型</span><strong>${escapeHtml(selectedDevice ? (selectedDevice.isEmulator ? "模拟器" : (selectedDevice.isWatch ? "真实手表" : "其他设备")) : "等待连接")}</strong></div>
            </div>
          </div>
          <div class="install-summary-box">
            <h4>安装包</h4>
            <div class="install-summary-list">
              <div class="install-summary-row"><span>版本</span><strong>${escapeHtml(installerState.apk?.versionName || "未检测")}</strong></div>
              <div class="install-summary-row"><span>签名</span><strong>${escapeHtml(installerState.apk?.debug ? "debug" : "release")}</strong></div>
              <div class="install-summary-row"><span>来源</span><strong>${escapeHtml(packageSource)}</strong></div>
            </div>
          </div>
        </div>
        <div class="install-callout">
          <strong>安装后会自动启动</strong>
          <p>应用安装完成后会自动在手表上打开，方便你继续下一步。</p>
        </div>
      </article>
      <article class="install-card">
        <div class="install-card-head">
          <div>
            <h3>安装结果</h3>
            <p>这里只保留目标设备、安装包和结果摘要。</p>
          </div>
        </div>
        <div class="install-result-list">
          <div class="install-result-item">
            <strong>安装包检查 <span class="install-status-chip ${installerState.apk?.available ? "ok" : "warn"}">${installerState.apk?.available ? "可安装" : "未就绪"}</span></strong>
            <span>${escapeHtml(installerState.apk?.message || installerSummaryNote())}</span>
          </div>
          <div class="install-result-item">
            <strong>安装状态 <span class="install-status-chip ${installerState.apk?.installed ? "ok" : "soft"}">${installerState.apk?.installed ? "已安装" : "等待执行"}</span></strong>
            <span>${escapeHtml(pageState.wizard.stageNotes.install || "点击开始安装后执行安装流程。")}</span>
          </div>
          <div class="install-result-item">
            <strong>启动状态 <span class="install-status-chip ${wizardStageCompleted("install") ? "ok" : "soft"}">${wizardStageCompleted("install") ? "已完成" : "等待执行"}</span></strong>
            <span>${escapeHtml(wizardStageCompleted("install") ? "手表应用已启动，可以继续写入配置。" : "安装完成后会自动启动应用。")}</span>
          </div>
        </div>
        <div class="button-row compact-right">
          <button class="primary-btn" data-command="wizard-run-install">${icon("watch")}${installActionLabel}</button>
        </div>
      </article>
    </div>
  `
}

function renderConfigEntryCard(entryId) {
  const entry = wizardConfigEntry(entryId)
  const meta = INSTALL_CONFIG_META[entryId]
  const enabled = Boolean(entry.enabled)
  const checking = entry.validation === "checking"
  let statusTone = "soft"
  let statusLabel = enabled ? "待检查" : "未启用"
  if (checking) {
    statusTone = "loading"
    statusLabel = entryId === "tunnel" ? "连接中" : "检查中"
  } else if (entry.validation === "valid") {
    statusTone = "ok"
    statusLabel = "已通过"
  } else if (entry.validation === "error") {
    statusTone = "warn"
    statusLabel = "未通过"
  } else if (entryId === "tunnel" && entry.redeemedDomain) {
    statusTone = enabled ? "ok" : "soft"
    statusLabel = enabled ? "已启用" : "可启用"
  } else if (enabled) {
    statusTone = "soft"
    statusLabel = entryId === "tunnel" ? "待兑换" : "待检查"
  }
  const statusChip = `<span class="install-status-chip ${statusTone}">${checking ? `<span class="install-spinner" aria-hidden="true"></span>` : ""}${statusLabel}</span>`
  const checkButtonLabel = checking ? "检查中" : meta.validateLabel
  const checkMessage = entry.message || statusLabel
  const checkMessageContent = checking
    ? `<span class="install-inline-loading"><span class="install-spinner" aria-hidden="true"></span>${escapeHtml(checkMessage)}</span>`
    : escapeHtml(checkMessage)

  return `
    <article class="install-config-card ${enabled ? "is-enabled" : ""} ${checking ? "is-checking" : ""} ${entry.validation === "valid" ? "is-valid" : ""} ${entryId === "tunnel" ? "is-wide" : ""}">
      <div class="install-config-card-head">
        <button class="install-config-toggle" data-command="wizard-toggle-config-entry" data-value="${entryId}" ${checking ? "disabled" : ""}>
          <span class="install-config-switch ${enabled ? "is-on" : ""}"></span>
          <strong>${meta.icon} ${meta.label}</strong>
        </button>
        ${statusChip}
      </div>
      <p>${meta.inactiveText}</p>
      ${entryId === "tunnel" ? `
        <div class="install-config-fields">
          <label class="field">
            <span>${meta.inputLabel}</span>
            <input value="${escapeHtml(entry.code)}" placeholder="例如 OW-ABCD-1234" data-bind="wizard.configEntries.tunnel.code" ${checking ? "disabled" : ""} />
          </label>
          <div class="install-inline-actions">
            <button class="secondary-btn small" data-command="wizard-validate-entry" data-value="tunnel" ${checking ? "disabled" : ""}>${checking ? `<span class="install-spinner" aria-hidden="true"></span>` : ""}${checkButtonLabel}</button>
            <span class="install-status-chip ${entry.redeemedDomain ? "ok" : "warn"}">${entry.redeemedDomain ? "已兑换" : "等待兑换"}</span>
          </div>
          ${entry.redeemedDomain ? `
            <div class="install-domain-row">
              <span>域名</span>
              <strong>${escapeHtml(entry.redeemedDomain)}</strong>
            </div>
          ` : ""}
          <div class="install-domain-row install-check-row">
            <span>/healthz</span>
            <strong>${checkMessageContent}</strong>
          </div>
        </div>
      ` : `
        <div class="install-config-fields">
          <label class="field">
            <span>${meta.inputLabel}</span>
            <input value="${escapeHtml(entry.url)}" data-bind="wizard.configEntries.${entryId}.url" ${checking ? "disabled" : ""} />
          </label>
          <div class="install-inline-actions">
            <button class="secondary-btn small" data-command="wizard-validate-entry" data-value="${entryId}" ${checking ? "disabled" : ""}>${checking ? `<span class="install-spinner" aria-hidden="true"></span>` : ""}${checkButtonLabel}</button>
            <span class="install-status-chip ${statusTone}">${escapeHtml(entry.message || statusLabel)}</span>
          </div>
        </div>
      `}
    </article>
  `
}

function renderConfigStage() {
  const enabledEntries = wizardEnabledConfigEntries()
  const selectedDevice = selectedInstallerDevice()
  return `
    <div class="install-stage-layout config">
      <article class="install-card">
        <div class="install-card-head">
          <div>
            <h3>启用要发送到手表的地址</h3>
            <p>只有你显式启用的地址，才会一起发送到手表确认。</p>
          </div>
        </div>
        ${renderWizardFailureNotice()}
        <div class="install-config-grid">
          ${renderConfigEntryCard("lan")}
          ${renderConfigEntryCard("public")}
          ${renderConfigEntryCard("tunnel")}
        </div>
      </article>
      <article class="install-card">
        <div class="install-card-head">
          <div>
            <h3>写入前检查</h3>
            <p>会一起整理所有已启用的地址，再发送到当前手表等待确认。</p>
          </div>
        </div>
        <div class="install-result-list">
          <div class="install-result-item">
            <strong>将一起发送的地址 <span class="install-status-chip soft">${enabledEntries.length} 项</span></strong>
            <span>${enabledEntries.length > 0 ? enabledEntries.map(([entryId]) => INSTALL_CONFIG_META[entryId].label).join("、") : "还没有启用任何地址。"}</span>
          </div>
          <div class="install-result-item">
            <strong>目标手表 <span class="install-status-chip ${selectedDevice ? "ok" : "warn"}">${selectedDevice ? "已确认" : "未确认"}</span></strong>
            <span>${escapeHtml(selectedDevice ? `${selectedDevice.displayName || selectedDevice.serial} 将接收配置链接，并需要在手表上确认。` : "请先回到上一页确认目标设备。")}</span>
          </div>
          <div class="install-result-item">
            <strong>最近配置链接 <span class="install-status-chip ${pageState.generatedBootstrap?.apiBase ? "ok" : "soft"}">${pageState.generatedBootstrap?.apiBase ? "已生成" : "未生成"}</span></strong>
            <span>${escapeHtml(pageState.generatedBootstrap?.apiBase || "生成后，这里会显示最近一次发送的地址。")}</span>
          </div>
        </div>
        <div class="install-callout">
          <strong>会一起发送多个已启用地址</strong>
          <p>手表收到配置链接后，需要先确认保存，再根据后续能力选择当前可用的连接入口。</p>
        </div>
        <div class="button-row compact-right">
          <button class="primary-btn" data-command="wizard-run-config">${icon("refresh")}${configWriteActionLabel()}</button>
        </div>
      </article>
    </div>
  `
}

function renderInstallStageContent(snapshot, live) {
  const stage = pageState.wizard.currentStage
  if (stage === "prepare") {
    return renderPrepareStage(live)
  }
  if (stage === "connect") {
    return renderConnectStage(snapshot, live)
  }
  if (stage === "install") {
    return renderInstallStage(snapshot, live)
  }
  return renderConfigStage(snapshot, live)
}

function renderInstallFooter() {
  const meta = wizardStageMeta()
  const failure = wizardCurrentStageError()
  const blockedByCheck = meta.id === "config" && wizardConfigChecksInProgress()
  const blockedLabel = blockedByCheck ? `${wizardConfigCheckingLabel()} 正在检查，完成后才能写入。` : ""
  if (failure) {
    return `
      <div class="install-footer">
        <div></div>
        <div class="install-footer-actions">
          <button class="secondary-btn" data-command="wizard-export-diagnostics">${icon("document")}导出诊断包</button>
          <button class="primary-btn" data-command="wizard-retry">重试</button>
        </div>
      </div>
    `
  }
  const showSecondary = meta.id !== "prepare" || meta.secondaryLabel
  return `
    <div class="install-footer">
      <div class="install-footer-note">${escapeHtml(blockedLabel || pageState.wizard.stageNotes[meta.id] || "")}</div>
      <div class="install-footer-actions">
        ${showSecondary ? `<button class="secondary-btn" data-command="wizard-secondary">${meta.secondaryLabel}</button>` : ""}
        <button class="primary-btn" data-command="wizard-primary" ${blockedByCheck ? "disabled" : ""}>${blockedByCheck ? `<span class="install-spinner" aria-hidden="true"></span>` : ""}${meta.primaryLabel}</button>
      </div>
    </div>
  `
}

function renderInstallPage(snapshot, live) {
  return `
    <section class="page-stack install-page" data-scroll-key="page-install">
      <section class="install-wizard-shell">
        ${renderInstallStageTabs()}
        <section class="install-stage-panel">
          <div class="install-stage-body" data-scroll-key="install-stage-${escapeHtml(pageState.wizard.currentStage)}">
            ${renderInstallStageContent(snapshot, live)}
          </div>
          ${renderInstallFooter()}
        </section>
      </section>
      ${renderInstallGuideModal()}
    </section>
  `
}

function renderWatchPage(snapshot, live) {
  const apiBase = currentApiBase(live)
  const installerState = pageState.installerState || fallbackInstallerState
  const selectedDevice = selectedInstallerDevice()
  const topCards = [
    { icon: "watch", title: "手表连接", value: selectedDevice ? "已连接" : "未连接", meta: installerStatusNote(), tone: selectedDevice ? "green" : "amber" },
    { icon: "watch", title: "当前设备", value: selectedDevice?.displayName || "未选择设备", meta: selectedDevice?.isWatch ? "Wear / Watch 类设备" : "等待连接", tone: selectedDevice ? "blue" : "amber" },
    { icon: "cloud", title: "OpenWatcher Watch 版本", value: installerState.apk?.versionName || "未检测", meta: installerState.apk?.debug ? "debug 本地验证" : "release 缓存就绪", tone: installerState.apk?.available ? "purple" : "amber" },
    { icon: "globe", title: "本机服务访问地址", value: apiBase, meta: "可通过当前模式访问", tone: "cyan" }
  ]

  return `
    <section class="page-stack" data-scroll-key="page-watch">
      <section class="card-surface status-strip">
        ${topCards.map(topStatusCard).join("")}
      </section>

      <section class="device-grid">
        <article class="panel-card">
          <h3>设备信息</h3>
          <div class="info-table">
            ${[
              ["设备名", selectedDevice?.displayName || "未检测"],
              ["制造商", selectedDevice?.product || "未检测"],
              ["型号", selectedDevice?.model || "未检测"],
              ["Android 版本", selectedDevice?.isEmulator ? "模拟器" : "待补充"],
              ["API level", "待检测"],
              ["ABI", selectedDevice?.device || "待检测"],
              ["ADB serial", selectedDevice?.serial || "未检测"],
              ["设备类型", selectedDevice?.isEmulator ? "Wear OS 模拟器" : (selectedDevice?.isWatch ? "手表设备" : "未知设备")]
            ].map(([label, value]) => `<div class="info-row"><span>${label}</span><strong>${value}</strong></div>`).join("")}
          </div>
        </article>

        <article class="panel-card">
          <h3>应用状态</h3>
          <div class="info-table">
            ${[
              ["安装状态", installerState.apk?.installed ? "已安装" : (installerState.apk?.available ? "可安装" : "未找到安装包")],
              ["版本", installerState.apk?.installedVersionName || installerState.apk?.versionName || "未检测"],
              ["versionCode", installerState.apk?.installedVersionCode || installerState.apk?.versionCode || "未检测"],
              ["包名", installerState.apk?.packageName || "未检测"],
              ["签名", installerState.apk?.debug ? "debug（仅本地验证）" : "release"],
              ["APK 来源", installerState.apk?.label || "未检测"],
              ["SHA-256", installerState.apk?.sha256 ? installerState.apk.sha256.slice(0, 12) + "..." : "未检测"]
            ].map(([label, value]) => `<div class="info-row"><span>${label}</span><strong>${value}</strong></div>`).join("")}
          </div>
          <div class="button-row">
            <button class="secondary-btn" data-command="install-watch-app">${icon("watch")}安装或覆盖安装</button>
            <button class="secondary-btn" data-command="launch-watch-app">${icon("refresh")}启动手表 App</button>
          </div>
        </article>

        <article class="panel-card">
          <h3>配置状态</h3>
          <div class="info-table">
            ${[
              ["API 基址", apiBase],
              ["设备名", pageState.watchForm.deviceName || "watch"],
              ["Token 指纹", pageState.generatedBootstrap?.tokenFingerprint || "尚未生成"],
              ["配置来源", "Desktop bootstrap"],
              ["配置时间", pageState.generatedBootstrap?.createdAt || "尚未写入"]
            ].map(([label, value]) => `<div class="info-row"><span>${label}</span><strong>${value}</strong></div>`).join("")}
          </div>
          <div class="button-row">
            <button class="primary-btn" data-command="apply-watch-bootstrap">${icon("refresh")}重新写入配置</button>
            <button class="secondary-btn" data-command="open-page" data-value="install">${icon("link")}调整写入地址</button>
            <button class="secondary-btn" data-command="prepare-watch-bootstrap">${icon("refresh")}仅生成 bootstrap URI</button>
            <button class="danger-btn" data-command="go-clear-pairing">${icon("trash")}重置配对</button>
          </div>
          ${renderBootstrapStatus()}
        </article>

        ${renderRemoteWatchBootstrapPanel(live)}

        <article class="panel-card">
          <h3>连接管理</h3>
          <div class="info-table">
            ${[
              ["无线 ADB 状态", selectedDevice ? "已连接" : "未连接"],
              ["连接 IP", selectedDevice?.serial?.includes(":") ? selectedDevice.serial.split(":")[0] : "未检测"],
              ["端口", pageState.installerState?.selectedPort || "未检测"]
            ].map(([label, value]) => `<div class="info-row"><span>${label}</span><strong>${value}</strong></div>`).join("")}
          </div>
          <div class="button-row">
            <button class="secondary-btn" data-command="refresh-installer-status">${icon("refresh")}刷新设备列表</button>
            <button class="secondary-btn" data-command="open-page" data-value="install">${icon("refresh")}重新配对</button>
            <button class="danger-btn" data-command="go-clear-pairing">${icon("trash")}忘记设备</button>
          </div>
          <p class="support-note">ADB 未连接不影响日常使用，仅在安装和维护时需要。</p>
        </article>
      </section>

      <section class="card-surface panel-card">
        <div class="section-mini-head">
          <h3>最近设备 / 兼容性记录</h3>
        </div>
        <table class="device-table">
          <thead>
            <tr>
              <th>设备名</th>
              <th>型号</th>
              <th>Android 版本</th>
              <th>最近连接时间</th>
              <th>ADB 状态</th>
              <th>兼容性</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            ${(installerDevices().length > 0 ? installerDevices().map((device) => ({
              name: device.displayName || device.serial,
              model: device.model || device.product || "未检测",
              android: device.isEmulator ? "Wear OS Emulator" : "Android / Wear OS",
              lastSeen: device.serial,
              adbState: device.state === "device" ? "已连接" : device.state,
              compatibility: device.isWatch ? "完全兼容" : "待验证",
              badge: device.serial === installerState.selectedSerial ? "当前" : ""
            })) : sampleDevices).map((device) => `
              <tr>
                <td>${device.name} ${device.badge ? badge(device.badge, "primary") : ""}</td>
                <td>${device.model}</td>
                <td>${device.android}</td>
                <td>${device.lastSeen}</td>
                <td>${device.adbState}</td>
                <td>${device.compatibility}</td>
                <td class="table-actions">${icon("link")}${icon("trash")}</td>
              </tr>
            `).join("")}
          </tbody>
        </table>
      </section>
    </section>
  `
}

function renderLogsPage(snapshot, live) {
  const rawLogs = filteredRawLogs()
  const timeline = filteredTimeline()
  const topCards = [
    { icon: "folder", title: "Desktop", value: "正常", meta: snapshot.productVersion || "dev", tone: "green" },
    { icon: "server", title: "本机服务", value: live.backendHealthy ? "正常" : "异常", meta: live.listen, tone: live.backendHealthy ? "green" : "amber" },
    { icon: "watch", title: "ADB", value: pageState.installerState?.selectedSerial ? "已连接" : "未连接", meta: installerStatusNote(), tone: pageState.installerState?.selectedSerial ? "green" : "amber" },
    { icon: "watch", title: "手表 App", value: pageState.installerState?.apk?.available ? "可安装" : "未检测", meta: pageState.installerState?.apk?.versionName || "待下载", tone: pageState.installerState?.apk?.available ? "blue" : "amber" },
    { icon: "globe", title: "网络访问", value: currentAccessModeLabel(), meta: `${currentSelectedIP(snapshot)} 可访问`, tone: "blue" }
  ]

  return `
    <section class="page-stack" data-scroll-key="page-logs">
      <section class="card-surface status-strip status-strip-five">
        ${topCards.map(topStatusCard).join("")}
      </section>

      <section class="logs-layout">
        <article class="panel-card narrow-list">
          <h3>日志来源</h3>
          ${[
            ["全部", "127"],
            ["Desktop", "34"],
            ["本机服务", "28"],
            ["开发环境", String(developerLogs().length || 0)],
            ["ADB 安装", "16"],
            ["手表 App", "14"],
            ["网络访问", "12"],
            ["托管隧道", "8"],
            ["更新", "7"],
            ["安全", "8"]
          ].map(([label, count], index) => `
            <button class="source-item ${pageState.selectedLogSource === (index === 0 ? "all" : label) ? "is-active" : ""}" data-log-source="${index === 0 ? "all" : label}">
              <span>${label}</span>
              <strong>${count}</strong>
            </button>
          `).join("")}
        </article>

        <article class="panel-card timeline-card">
          <h3>事件时间线</h3>
          <div class="timeline-table">
            ${timeline.map((item) => `
              <div class="timeline-row">
                <span class="timeline-time">${item.time}</span>
                <span class="timeline-event">${item.event}</span>
                <span class="level-tag level-${item.level.toLowerCase()}">${item.level}</span>
                <span class="timeline-source">${item.source}</span>
              </div>
            `).join("")}
          </div>
        </article>

        <article class="panel-card diagnosis-card">
          <h3>自动诊断建议</h3>
          <div class="warning-box">
            <strong>发现 1 个可能问题</strong>
            <p>手表无法访问本机服务</p>
            <button class="primary-btn small" data-command="open-page" data-value="install">查看详情</button>
          </div>
          <div class="diagnosis-copy">
            <h4>可能原因</h4>
            <ul>
              <li>本机服务可能仅监听在 127.0.0.1，手表无法访问</li>
              <li>网络防火墙阻止了局域网访问</li>
              <li>设备所在网络不在同一子网</li>
            </ul>
            <h4>推荐操作</h4>
            <button class="secondary-btn full-width" data-command="open-page" data-value="install">${icon("globe")}返回安装向导调整地址</button>
            <button class="secondary-btn full-width" data-command="open-page" data-value="install">${icon("shield")}检查写入配置</button>
            <button class="secondary-btn full-width" data-command="copy-diagnostics">${icon("copy")}复制诊断信息</button>
          </div>
        </article>
      </section>

      <section class="logs-layout bottom-layout">
        <article class="panel-card raw-log-card">
          <div class="section-mini-head">
            <h3>原始日志查看器</h3>
            <div class="mini-filters">
              <span class="mini-chip">搜索日志内容...</span>
              <span class="mini-chip">所有级别</span>
              <span class="mini-chip">2025-05-18</span>
              <span class="mini-chip">10:20 - 10:25</span>
            </div>
          </div>
          <div class="console-window">
            ${rawLogs.map((line) => `<div>${escapeHtml(line)}</div>`).join("")}
          </div>
        </article>

        <article class="panel-card export-card">
          <h3>导出脱敏诊断包</h3>
          <p>将收集必要的日志与环境信息，并自动排除敏感内容。</p>
          <button class="primary-btn full-width" data-command="copy-diagnostics">${icon("document")}导出脱敏诊断包</button>
          <div class="export-grid">
            <div>
              <strong>包含以下信息</strong>
              <ul>
                <li>Desktop 日志</li>
                <li>本机服务日志</li>
                <li>开发环境日志</li>
                <li>ADB 操作日志</li>
                <li>设备信息</li>
              </ul>
            </div>
            <div>
              <strong>不包含以下信息</strong>
              <ul>
                <li>Codex access token</li>
                <li>完整 device token</li>
                <li>tunnel token</li>
              </ul>
            </div>
          </div>
        </article>
      </section>
    </section>
  `
}

function renderSettingsPage(snapshot, live) {
  const tab = pageState.settingsTab
  const developerSummary = developerEnvironmentSummaryLines()
  const developerAccessOptions = [
    { id: "emulator", label: "模拟器" },
    { id: "lan", label: "局域网" },
    { id: "tunnel", label: "开发隧道" },
    { id: "custom", label: "自定义地址" }
  ]
  const contentMap = {
    general: `
      <article class="panel-card">
        <h3>常规设置</h3>
        <div class="toggle-list">
          ${[
            ["开机启动 OpenWatcher", true],
            ["启动后自动启动本机服务", true],
            ["关闭窗口后最小化到托盘", true]
          ].map(([label, on]) => `
            <div class="toggle-row">
              <span>${label}</span>
              <span class="toggle ${on ? "is-on" : ""}"></span>
            </div>
          `).join("")}
        </div>
        <div class="field-grid two-col">
          <label class="field">
            <span>语言</span>
            <input value="简体中文" readonly />
          </label>
          <label class="field">
            <span>主题</span>
            <input value="深色（默认）" readonly />
          </label>
        </div>
      </article>
    `,
    backend: `
      <article class="panel-card">
        <h3>本机服务</h3>
        <div class="info-table">
          ${[
            ["本机服务二进制版本", live.backendBuildVersion],
            ["配置文件路径", snapshot.backend?.configPathLabel || "~/.openwatcher/config.json"],
            ["默认监听地址", live.listen],
            ["截图目录", "~/.openwatcher/screenshots"],
            ["诊断目录", "~/.openwatcher/diagnostics"]
          ].map(([label, value]) => `<div class="info-row"><span>${label}</span><strong>${value}</strong></div>`).join("")}
        </div>
        <div class="button-row">
          <button class="secondary-btn" data-command="open-backend-config-dir">${icon("folder")}打开配置目录</button>
          <button class="secondary-btn" data-command="restart-backend">${icon("refresh")}重启本机服务</button>
          <button class="secondary-btn" data-command="test-connection">${icon("link")}健康检查</button>
        </div>
      </article>
    `,
    codex: `
      <article class="panel-card">
        <h3>Codex 环境</h3>
        <div class="field-grid">
          <label class="field field-inline">
            <span>Codex Home</span>
            <input value="${live.codexHomeLabel}" readonly />
          </label>
        </div>
        <div class="info-table">
          <div class="info-row"><span>auth.json</span><strong>${live.codex.authDetected ? "已检测" : "未检测"}</strong></div>
          <div class="info-row"><span>sessions</span><strong>${live.codex.sessionsDetected ? "已检测" : "未检测"}</strong></div>
        </div>
        <div class="button-row">
          <button class="secondary-btn" data-command="open-codex-home">${icon("folder")}打开 Codex 目录</button>
          <button class="secondary-btn" data-command="reload-snapshot">${icon("refresh")}重新检测</button>
        </div>
      </article>
    `,
    resources: `
      <article class="panel-card">
        <h3>手表安装资源</h3>
        <div class="info-table">
          ${[
            ["ADB 版本", pageState.installerState?.adb?.version || "未检测"],
            ["手表 APK 版本", pageState.installerState?.apk?.versionName || "未检测"],
            ["APK 发布策略", pageState.installerState?.apk?.debug ? "仅开发 / 模拟器可用" : "release 优先"],
            ["SHA-256", pageState.installerState?.apk?.sha256 ? pageState.installerState.apk.sha256.slice(0, 16) + "..." : "未检测"],
            ["cloudflared 状态", currentTunnel(snapshot).running ? "运行中" : (currentTunnel(snapshot).resolvedBinary ? "已检测" : "未检测")]
          ].map(([label, value]) => `<div class="info-row"><span>${label}</span><strong>${value}</strong></div>`).join("")}
        </div>
        <div class="button-row">
          <button class="secondary-btn" data-command="refresh-installer-status">${icon("refresh")}重新检测资源</button>
          <button class="secondary-btn" data-command="open-page" data-value="install">${icon("watch")}返回安装向导</button>
        </div>
      </article>
    `,
    privacy: `
      <article class="panel-card">
        <h3>隐私与安全</h3>
        <div class="toggle-list">
          ${[
            ["诊断包默认脱敏", true],
            ["允许匿名兼容性上报", true],
            ["允许检查更新", true],
            ["自动清理配对码日志", true]
          ].map(([label, on]) => `
            <div class="toggle-row">
              <span>${label}</span>
              <span class="toggle ${on ? "is-on" : ""}"></span>
            </div>
          `).join("")}
        </div>
        <p class="support-note">不会在未经您同意的情况下上传任何个人信息。</p>
      </article>
    `,
    updates: `
      <article class="panel-card">
        <h3>更新</h3>
        <div class="info-table">
          ${[
            ["Desktop 当前版本", `${snapshot.productVersion || "dev"} (Technical Preview)`],
            ["手表 App 当前版本", pageState.installerState?.apk?.installedVersionName || "未检测"]
          ].map(([label, value]) => `<div class="info-row"><span>${label}</span><strong>${value}</strong></div>`).join("")}
        </div>
        <button class="secondary-btn" data-command="toast" data-value="更新检查将接入真实版本元数据。">${icon("refresh")}检查更新</button>
      </article>
    `,
    developer: `
      <section class="developer-page-layout">
        <div class="developer-main-column">
          <section class="section-head developer-page-head">
            <div>
              <h2>开发环境</h2>
              <p>在 Desktop 中启动和管理仓库的开发环境，并发送到手表进行调试。</p>
            </div>
            <div class="button-row developer-page-head-actions">
              <button class="danger-btn" data-command="clear-developer-pairing" ${pairingActionBusy() ? "disabled" : ""}>${icon("trash")}${pairingActionLabel("dev", "重置开发配对", "正在重置...")}</button>
              <button class="secondary-btn" data-command="developer-refresh-environment" ${pageState.developerAction.busy ? "disabled" : ""}>${icon("refresh")}重新检测</button>
              <button class="secondary-btn" data-command="open-developer-usage">${icon("document")}使用说明</button>
            </div>
          </section>

          <article class="panel-card developer-config-card">
            <h3>开发环境配置</h3>
            <div class="developer-config-stack">
              <div class="developer-setting-row">
                <div class="developer-setting-title">仓库目录</div>
                <div class="developer-setting-body developer-setting-body-inline">
                  <input data-bind="developerForm.repoPath" value="${escapeHtml(pageState.developerForm.repoPath)}" placeholder="/path/to/repo" />
                  <button class="secondary-btn developer-inline-btn" data-command="select-developer-repo-dir">${icon("folder")}选择文件夹</button>
                </div>
              </div>

              <div class="developer-setting-row">
                <div class="developer-setting-title">启动脚本（从仓库启动配置检测）</div>
                <div class="developer-setting-body">
                  <select disabled>
                    <option>${escapeHtml(developerStartCommand() || "scripts/start-local.sh")}</option>
                  </select>
                  <small class="developer-field-note">实际执行：${escapeHtml(developerStartCommand() || "scripts/start-local.sh")}</small>
                </div>
              </div>

              <div class="developer-setting-row">
                <div class="developer-setting-title">访问方式</div>
                <div class="developer-setting-body">
                <div class="developer-radio-grid">
                  ${developerAccessOptions.map((option) => `
                    <button class="radio-line ${pageState.developerForm.accessMode === option.id ? "is-selected" : ""}" data-developer-access-mode="${option.id}">
                      <span class="radio-dot"></span>
                      <span>${escapeHtml(option.label)}</span>
                    </button>
                  `).join("")}
                </div>
                </div>
              </div>

              <div class="developer-setting-row">
                <div class="developer-setting-title">服务地址</div>
                <div class="developer-setting-body developer-setting-body-inline">
                  <input data-bind="developerForm.devBaseUrl" value="${escapeHtml(developerBaseUrlForCurrentForm())}" ${pageState.developerForm.accessMode === "custom" ? "" : "readonly"} placeholder="http://10.0.2.2:18787" />
                  <button class="secondary-btn developer-inline-btn" data-command="copy-developer-base-url">${icon("copy")}复制</button>
                </div>
              </div>

              <div class="developer-setting-row">
                <div class="developer-setting-title">Healthz 地址</div>
                <div class="developer-setting-body developer-setting-body-inline">
                  <input value="${escapeHtml(developerHealthzUrl())}" readonly />
                  <button class="secondary-btn developer-inline-btn" data-command="copy-developer-healthz-url">${icon("copy")}复制</button>
                </div>
              </div>

              <div class="developer-setting-row">
                <div class="developer-setting-title">环境变量</div>
                <div class="developer-setting-body developer-setting-body-inline">
                  <input value="${escapeHtml(developerEnvFileLabel())}" readonly />
                  <button class="secondary-btn developer-inline-btn" data-command="open-developer-env-file" ${developerStatus()?.envFilePresent ? "" : "disabled"}>${icon("folder")}查看</button>
                </div>
              </div>

            </div>
          </article>

          <article class="panel-card developer-log-card">
            <div class="developer-log-head">
              <div>
                <h3>启动日志（实时）</h3>
                <p>${developerLogFileLabel() ? `日志文件：${escapeHtml(developerLogFileLabel())}` : "日志会写入 Desktop 配置目录下的 logs 文件夹。"}</p>
              </div>
              <div class="button-row developer-log-actions">
                <button class="secondary-btn" data-command="clear-developer-logs">${icon("trash")}清空日志</button>
                <button class="secondary-btn" data-command="open-developer-log-file" ${developerLogFileLabel() ? "" : "disabled"}>${icon("folder")}打开日志文件</button>
              </div>
            </div>
            <div class="console-window developer-console developer-console-rich">
              ${renderDeveloperLogLines(24)}
            </div>
          </article>
        </div>

        <aside class="developer-side-column">
          <article class="panel-card developer-side-card">
            <div class="developer-side-title">
              <h3>${icon("server")}环境控制 <small>当前仓库</small></h3>
            </div>
            <strong class="developer-side-repo">${escapeHtml(developerCurrentRepoLabel())}</strong>
            <div class="developer-runtime-pill tone-${developerStateTone()} developer-runtime-pill-with-actions">
              <div class="developer-runtime-copy">
                <strong>${developerStateLabel()}</strong>
                <span>${escapeHtml(developerStartedDurationLabel())}</span>
              </div>
              <div class="developer-runtime-actions">
                ${developerStatusPhase() !== "running" ? `
                  <button class="secondary-btn icon-only developer-runtime-icon-btn" data-command="developer-toggle-environment" title="启动环境" aria-label="启动环境" ${developerStatusPhase() === "starting" || developerStatusPhase() === "stopping" ? "disabled" : ""}>
                    ${icon("play")}
                  </button>
                ` : `
                  <button class="danger-btn icon-only developer-runtime-icon-btn" data-command="developer-toggle-environment" title="停止环境" aria-label="停止环境" ${developerStatusPhase() === "starting" || developerStatusPhase() === "stopping" ? "disabled" : ""}>
                    ${icon("close")}
                  </button>
                `}
                <button class="secondary-btn icon-only developer-runtime-icon-btn" data-command="restart-developer-environment" title="重启环境" aria-label="重启环境" ${pageState.developerAction.busy ? "disabled" : ""}>
                  ${icon("refresh")}
                </button>
              </div>
            </div>
          </article>

          <article class="panel-card developer-side-card">
            <div class="developer-side-title">
              <h3>${icon("cloud")}开发隧道</h3>
              <button class="toggle ${developerTunnelIsManaged() ? "is-on" : ""}" data-command="toggle-developer-tunnel" title="${developerTunnelIsManaged() ? "关闭开发隧道保活" : "启用开发隧道保活"}" aria-label="${developerTunnelIsManaged() ? "关闭开发隧道保活" : "启用开发隧道保活"}"></button>
            </div>
            <p class="developer-side-copy">当手表无法访问本机时，可使用开发隧道。</p>
            <div class="developer-tunnel-inline">
              <input data-bind="developerForm.tunnelCode" value="${escapeHtml(pageState.developerForm.tunnelCode)}" placeholder="输入隧道配置码" />
              <button class="secondary-btn" data-command="redeem-developer-tunnel">${icon("cloud")}激活隧道</button>
            </div>
            ${developerTunnelBaseUrl() ? `
              <div class="developer-tunnel-summary">
                <div class="developer-tunnel-summary-head">
                  <span>隧道地址</span>
                  <button class="secondary-btn small icon-only developer-tunnel-copy-btn" data-command="copy-developer-tunnel-url" title="复制地址" aria-label="复制地址">${icon("copy")}</button>
                </div>
                <strong>${escapeHtml(developerTunnelBaseUrl())}</strong>
                ${pageState.developerForm.accessMode === "tunnel" ? `<span class="developer-inline-state">当前使用中</span>` : ""}
              </div>
            ` : ""}
            <button class="text-link developer-text-link" data-command="toast" data-value="开发隧道用于在手表无法直接访问本机时，通过已兑换的公开地址访问当前开发服务。">什么是开发隧道？</button>
          </article>

          <article class="panel-card developer-side-card">
            <div class="developer-side-title">
              <h3>${icon("watch")}发送到手表 <small>目标设备</small></h3>
              <button class="secondary-btn small" data-command="refresh-installer-status">${icon("refresh")}</button>
            </div>
            <div class="developer-device-row">
              <strong>${escapeHtml(developerDeviceConnectionLabel())}</strong>
              <span class="status-dot ${selectedInstallerDevice() ? "status-dot-green" : "status-dot-slate"}"><span class="status-dot-core"></span></span>
            </div>
            <div class="developer-summary-box">
              <div class="info-row compact-info-row"><span>Base URL:</span><strong>${escapeHtml(developerSummary.baseURL || "未设置")}</strong></div>
              <div class="info-row compact-info-row"><span>Healthz:</span><strong>${escapeHtml(developerSummary.healthz ? developerSummary.healthz.replace(developerSummary.baseURL, "") || "/healthz" : "未设置")}</strong></div>
              <div class="info-row compact-info-row"><span>Bootstrap:</span><strong>${escapeHtml(developerSummary.bootstrap)}</strong></div>
            </div>
            <button class="primary-btn full-width large" data-command="apply-dev-watch-bootstrap" ${developerCanSendToWatch() ? "" : "disabled"}>${icon("watch")}发送开发环境到手表</button>
            ${developerSendDisabledReason() ? `<p class="developer-side-note">${escapeHtml(developerSendDisabledReason())}</p>` : ""}
            <div class="button-row developer-side-actions">
              <button class="secondary-btn full-width" data-command="prepare-dev-watch-bootstrap">${icon("copy")}生成并复制 Bootstrap URI</button>
              <button class="secondary-btn full-width" data-command="copy-developer-base-url">${icon("copy")}复制 Base URL</button>
            </div>
          </article>
        </aside>
      </section>
    `,
    advanced: `
      <article class="panel-card">
        <h3>高级</h3>
        <div class="toggle-list">
          ${[
            ["使用系统 PATH 中的 adb", true],
            ["ADB 服务端端口", "5037"],
            ["启用开发者日志", true],
            ["显示原始命令输出", false],
            ["实验性托管隧道", true]
          ].map((item) => `
            <div class="toggle-row">
              <span>${item[0]}</span>
              ${typeof item[1] === "string" ? `<strong>${item[1]}</strong>` : `<span class="toggle ${item[1] ? "is-on" : ""}"></span>`}
            </div>
          `).join("")}
        </div>
        <div class="button-row">
          <button class="secondary-btn" data-command="open-desktop-config-dir">${icon("folder")}打开 Desktop 目录</button>
        </div>
      </article>
    `,
    danger: `
      <section class="danger-zone">
        <div class="danger-zone-head">
          <h3>危险操作</h3>
          <p>这些操作不可逆，请谨慎操作。建议在执行前先备份相关配置与数据。</p>
        </div>
        <div class="danger-grid">
          ${[
            ["重置 Desktop 配置", "将 Desktop 恢复到默认状态，保留日志与诊断数据。", "重置", "toast", "重置 Desktop 配置 动作尚未接入。"],
            ["重置本机服务配置", "清除本机服务配置文件，恢复默认配置。不会停止本机服务。", "重置", "toast", "重置本机服务配置 动作尚未接入。"],
            ["清空 beta 配对", "只清除本机服务 beta 槽位中的配对信息，不影响开发环境 dev 配对。", pairingActionLabel("beta", "清空", "正在清空..."), "clear-backend-pairing", "", pairingActionBusy()],
            ["撤销托管隧道", "立即撤销所有托管隧道与相关凭证，将影响当前网络访问。", "撤销", "toast", "撤销托管隧道 动作尚未接入。"]
          ].map(([title, desc, action, command, value, disabled]) => `
            <article class="danger-card">
              <h4>${title}</h4>
              <p>${desc}</p>
              <button class="danger-btn full-width" data-command="${command}" ${value ? `data-value="${value}"` : ""} ${disabled ? "disabled" : ""}>${action}</button>
            </article>
          `).join("")}
        </div>
      </section>
    `
  }

  return `
    <section class="page-stack ${tab === "developer" ? "developer-page-stack" : ""}" data-scroll-key="page-settings-${escapeHtml(tab)}">
      <section class="settings-tabs">
        ${SETTINGS_TABS.map((item) => `
          <button class="settings-tab ${tab === item.id ? "is-active" : ""} ${item.danger ? "is-danger" : ""}" data-settings-tab="${item.id}">${item.label}</button>
        `).join("")}
      </section>
      ${tab === "danger"
        ? contentMap[tab]
        : tab === "developer"
          ? contentMap[tab]
          : `<section class="settings-grid">${contentMap[tab]}</section>`}
    </section>
  `
}

function renderPage(snapshot) {
  const live = deriveLiveState(snapshot)
  switch (pageState.currentPage) {
    case "watch":
      return renderWatchPage(snapshot, live)
    case "logs":
      return renderLogsPage(snapshot, live)
    case "settings":
      return renderSettingsPage(snapshot, live)
    case "install":
    default:
      return renderInstallPage(snapshot, live)
  }
}

function scrollKeyForElement(element, index) {
  const explicitKey = element?.getAttribute?.("data-scroll-key")
  if (explicitKey) {
    return `scroll:${explicitKey}`
  }
  const id = element?.id
  if (id) {
    return `id:${id}`
  }
  const tag = String(element?.tagName || "node").toLowerCase()
  const className = String(element?.className || "")
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 4)
    .join(".")
  return `${tag}:${className}:${index}`
}

function isScrollableElement(element) {
  if (!element || element === document.body || element === document.documentElement) {
    return false
  }
  if (element.getAttribute?.("data-scroll-key")) {
    return true
  }
  const style = typeof window.getComputedStyle === "function" ? window.getComputedStyle(element) : null
  const canScrollY = /auto|scroll|overlay/.test(style?.overflowY || "")
  const canScrollX = /auto|scroll|overlay/.test(style?.overflowX || "")
  return (canScrollY && element.scrollHeight > element.clientHeight)
    || (canScrollX && element.scrollWidth > element.clientWidth)
}

function collectScrollContainers(app) {
  const root = app || document.getElementById("app")
  if (!root) {
    return []
  }
  const candidates = [
    ...root.querySelectorAll("[data-scroll-key], .page-stack, .install-stage-body, .notification-panel-body, .console-window, .sidebar")
  ]
  return candidates.filter((element, index, list) => list.indexOf(element) === index && isScrollableElement(element))
}

function captureScrollPositions(app) {
  const positions = {}
  for (const [index, element] of collectScrollContainers(app).entries()) {
    const key = scrollKeyForElement(element, index)
    positions[key] = {
      top: element.scrollTop || 0,
      left: element.scrollLeft || 0
    }
  }
  return positions
}

function restoreScrollPositions(positions) {
  if (!positions) {
    return
  }
  const containers = collectScrollContainers(document.getElementById("app"))
  for (const [index, element] of containers.entries()) {
    const position = positions[scrollKeyForElement(element, index)]
    if (!position) {
      continue
    }
    element.scrollTop = position.top || 0
    element.scrollLeft = position.left || 0
  }
}

function captureInteractionState(app) {
  const active = document.activeElement
  const activePath = active?.getAttribute?.("data-bind") || ""
  const contentArea = app?.querySelector?.(".content-area")
  const scrollingElement = document.scrollingElement || document.documentElement
  return {
    page: pageState.currentPage,
    windowX: window.scrollX || 0,
    windowY: window.scrollY || 0,
    documentScrollTop: scrollingElement?.scrollTop || 0,
    contentScrollTop: contentArea?.scrollTop || 0,
    scrollPositions: captureScrollPositions(app),
    activePath,
    activeValue: activePath && "value" in active ? active.value : "",
    selectionStart: activePath && typeof active.selectionStart === "number" ? active.selectionStart : null,
    selectionEnd: activePath && typeof active.selectionEnd === "number" ? active.selectionEnd : null
  }
}

function findBoundElement(path) {
  if (!path) {
    return null
  }
  for (const node of document.querySelectorAll("[data-bind]")) {
    if (node.getAttribute("data-bind") === path) {
      return node
    }
  }
  return null
}

function restoreInteractionState(state) {
  if (!state || state.page !== pageState.currentPage) {
    return
  }
  const restore = () => {
    const contentArea = document.querySelector(".content-area")
    if (contentArea) {
      contentArea.scrollTop = state.contentScrollTop
    }
    const scrollingElement = document.scrollingElement || document.documentElement
    if (scrollingElement) {
      scrollingElement.scrollTop = state.documentScrollTop
    }
    if (typeof window.scrollTo === "function") {
      window.scrollTo(state.windowX, state.windowY)
    }
    restoreScrollPositions(state.scrollPositions)
    const active = findBoundElement(state.activePath)
    if (!active) {
      return
    }
    if (isDraftField(pageState.draftFields, state.activePath) && "value" in active) {
      active.value = state.activeValue
    }
    if (typeof active.focus === "function") {
      active.focus({ preventScroll: true })
    }
    if (
      typeof active.setSelectionRange === "function"
      && state.selectionStart != null
      && state.selectionEnd != null
    ) {
      active.setSelectionRange(state.selectionStart, state.selectionEnd)
    }
  }
  restore()
  if (typeof window.requestAnimationFrame === "function") {
    window.requestAnimationFrame(restore)
    window.requestAnimationFrame(() => window.requestAnimationFrame(restore))
  }
  if (typeof window.setTimeout === "function") {
    window.setTimeout(restore, 80)
  }
}

function renderApp(options = {}) {
  const app = document.getElementById("app")
  const snapshot = pageState.snapshot
  const live = deriveLiveState(snapshot)
  const interactionState = options.preserveInteraction ? captureInteractionState(app) : null
  document.documentElement.dataset.theme = pageState.theme
  document.documentElement.dataset.platform = snapshot?.system?.platform || ""

  app.innerHTML = `
    <div class="desktop-shell">
      ${renderTopBar(snapshot, live)}
      <div class="desktop-main">
        ${renderSidebar()}
        <main class="content-area ${pageState.currentPage === "install" ? "is-install-page" : ""}">
          ${renderPage(snapshot)}
        </main>
      </div>
      ${renderDeveloperConfirmModal()}
    </div>
  `

  bindInteractions()
  restoreInteractionState(interactionState)
}

function bindInteractions() {
  for (const node of document.querySelectorAll("[data-topbar]")) {
    node.addEventListener("dblclick", async (event) => {
      if (event.target.closest("button, input, select, a")) {
        return
      }
      if (!window?.runtime?.WindowToggleMaximise) {
        return
      }
      event.preventDefault()
      await window.runtime.WindowToggleMaximise()
    })
  }

  for (const node of document.querySelectorAll("[data-nav]")) {
    node.addEventListener("click", () => {
      pageState.currentPage = node.getAttribute("data-nav")
      renderApp()
    })
  }

  for (const node of document.querySelectorAll("[data-command]")) {
    node.addEventListener("click", async () => {
      const command = node.getAttribute("data-command")
      const value = node.getAttribute("data-value") || ""
      if (command === "toast") {
        showNotice(value)
        return
      }
      if (command === "toggle-notification-panel") {
        toggleNotificationPanel()
        return
      }
      if (command === "open-developer-usage") {
        openDeveloperConfirmModal()
        return
      }
      if (command === "developer-confirm-cancel") {
        closeDeveloperConfirmModal()
        return
      }
      if (command === "close-notification-panel") {
        closeNotificationPanel()
        return
      }
      if (command === "run-global-health-check") {
        await runGlobalHealthCheck({ manual: true })
        return
      }
      if (command === "open-page") {
        pageState.currentPage = value
        renderApp()
        return
      }
      if (command === "toggle-theme") {
        pageState.theme = pageState.theme === "dark" ? "light" : "dark"
        renderApp()
        return
      }
      if (command === "copy-text") {
        await copyText(value || node.closest(".inline-copy")?.querySelector("input")?.value || "")
        return
      }
      if (command === "copy-api-base") {
        const live = deriveLiveState(pageState.snapshot)
        await copyText(currentApiBase(live))
        return
      }
      if (command === "redeem-managed-tunnel") {
        await redeemManagedTunnelAction()
        return
      }
      if (command === "prepare-watch-bootstrap") {
        await prepareWatchBootstrapAction()
        return
      }
      if (command === "apply-watch-bootstrap") {
        await applyWatchBootstrapAction()
        return
      }
      if (command === "fill-remote-watch-api-base") {
        const live = deriveLiveState(pageState.snapshot)
        pageState.remoteBootstrapForm.apiBase = remoteBootstrapDefaultApiBase(live)
        renderApp()
        return
      }
      if (command === "submit-remote-watch-bootstrap") {
        await submitRemoteWatchBootstrapAction()
        return
      }
      if (command === "prepare-dev-watch-bootstrap") {
        await prepareDevWatchBootstrapAction()
        return
      }
      if (command === "apply-dev-watch-bootstrap") {
        await applyDevWatchBootstrapAction()
        return
      }
      if (command === "copy-diagnostics") {
        try {
          const payload = await invoke("CopyDiagnostics")
          await copyText(payload)
        } catch {
          await copyText("当前未接入 Wails 运行时诊断导出。")
        }
        return
      }
      if (command === "start-backend") {
        await startBackendAction()
        return
      }
      if (command === "restart-backend") {
        await restartBackendAction()
        return
      }
      if (command === "reload-snapshot") {
        await runGlobalHealthCheck({ manual: true })
        return
      }
      if (command === "open-desktop-config-dir") {
        await openFolderAction("OpenDesktopConfigDir", "已打开 Desktop 配置目录。")
        return
      }
      if (command === "open-backend-config-dir") {
        await openFolderAction("OpenBackendConfigDir", "已打开本机服务配置目录。")
        return
      }
      if (command === "open-codex-home") {
        await openFolderAction("OpenCodexHome", "已打开 Codex 目录。")
        return
      }
      if (command === "test-connection") {
        await runHealthCheckAction()
        return
      }
      if (command === "refresh-installer-status") {
        await refreshInstallerState()
        showNotice("ADB 设备状态已刷新。")
        return
      }
      if (command === "developer-toggle-environment") {
        await toggleDeveloperEnvironmentAction()
        return
      }
      if (command === "developer-refresh-environment") {
        await loadDeveloperEnvironmentSnapshot({ ensure: developerIsRunning() })
        renderApp()
        return
      }
      if (command === "select-developer-repo-dir") {
        try {
          const selectedPath = await invoke("ChooseDeveloperRepositoryDir", pageState.developerForm.repoPath.trim())
          if (!selectedPath) {
            return
          }
          pageState.developerForm.repoPath = selectedPath
          if (pageState.developerForm.accessMode !== "custom") {
            updateDeveloperBaseUrlFromAccessMode(pageState.developerForm.accessMode)
          }
          persistDeveloperFormState()
          await loadDeveloperEnvironmentSnapshot({ ensure: false })
          renderApp()
        } catch (error) {
          showNotice(`选择仓库目录失败：${String(error)}`)
        }
        return
      }
      if (command === "restart-developer-environment") {
        await restartDeveloperEnvironmentAction()
        return
      }
      if (command === "clear-developer-logs") {
        pageState.developerSnapshot = await invoke("ClearDeveloperEnvironmentLogs")
        renderApp()
        showNotice("开发环境日志已清空。")
        return
      }
      if (command === "open-developer-log-file") {
        await openFolderAction("OpenDeveloperLogFile", "已打开开发环境日志文件。")
        return
      }
      if (command === "open-developer-env-file") {
        try {
          const errorMessage = await invoke("OpenDeveloperEnvFile", pageState.developerForm.repoPath.trim())
          if (errorMessage) {
            showNotice(errorMessage)
            return
          }
          showNotice("已打开 .env.development。")
        } catch (error) {
          showNotice(`打开环境变量文件失败：${String(error)}`)
        }
        return
      }
      if (command === "copy-developer-base-url") {
        await copyText(developerBaseUrlForCurrentForm())
        return
      }
      if (command === "copy-developer-healthz-url") {
        await copyText(developerHealthzUrl())
        return
      }
      if (command === "copy-developer-start-command") {
        await copyText(developerStartCommand())
        return
      }
      if (command === "copy-developer-tunnel-url") {
        await copyText(developerTunnelBaseUrl())
        return
      }
      if (command === "toggle-developer-tunnel") {
        pageState.developerForm.managedTunnelEnabled = !pageState.developerForm.managedTunnelEnabled
        persistDeveloperFormState()
        if (developerIsRunning()) {
          await loadDeveloperEnvironmentSnapshot({ ensure: true })
        } else {
          await loadDeveloperEnvironmentSnapshot({ ensure: false })
        }
        showNotice(pageState.developerForm.managedTunnelEnabled ? "已启用开发隧道保活与自动恢复。" : "已关闭开发隧道保活与自动恢复。")
        renderApp()
        return
      }
      if (command === "redeem-developer-tunnel") {
        await redeemDeveloperTunnelAction()
        return
      }
      if (command === "wizard-primary") {
        await handleWizardPrimaryAction()
        return
      }
      if (command === "wizard-secondary") {
        handleWizardSecondaryAction()
        return
      }
      if (command === "wizard-run-connect") {
        await runConnectStageAction()
        return
      }
      if (command === "wizard-run-install") {
        await runInstallStageAction()
        return
      }
      if (command === "wizard-run-config") {
        await runConfigStageAction()
        return
      }
      if (command === "wizard-go-stage") {
        if (!wizardStageUnlocked(value)) {
          showNotice("请先完成当前步骤。")
          return
        }
        clearWizardStageError()
        setWizardStage(value || "prepare")
        renderApp()
        return
      }
      if (command === "wizard-retry") {
        await retryCurrentWizardStage()
        return
      }
      if (command === "wizard-export-diagnostics") {
        await exportDiagnosticsAction()
        return
      }
      if (command === "go-clear-pairing") {
        pageState.currentPage = "settings"
        pageState.settingsTab = "danger"
        renderApp()
        return
      }
      if (command === "wizard-open-guide") {
        pageState.wizard.guide.selectedBrand = value
        pageState.wizard.guide.stepIndex = 0
        pageState.wizard.guide.modalOpen = true
        renderApp()
        return
      }
      if (command === "wizard-close-guide") {
        pageState.wizard.guide.modalOpen = false
        renderApp()
        return
      }
      if (command === "wizard-guide-prev") {
        pageState.wizard.guide.stepIndex = Math.max(0, pageState.wizard.guide.stepIndex - 1)
        renderApp()
        return
      }
      if (command === "wizard-guide-next") {
        const lastIndex = wizardSelectedGuide().steps.length - 1
        pageState.wizard.guide.stepIndex = Math.min(lastIndex, pageState.wizard.guide.stepIndex + 1)
        renderApp()
        return
      }
      if (command === "wizard-toggle-manual-check") {
        pageState.wizard.manualChecks[value] = !pageState.wizard.manualChecks[value]
        renderApp()
        return
      }
      if (command === "wizard-toggle-all-manual-checks") {
        toggleAllWizardManualChecks()
        renderApp()
        return
      }
      if (command === "wizard-toggle-advanced-connect") {
        pageState.installForm.useSeparateConnectIP = !pageState.installForm.useSeparateConnectIP
        if (!pageState.installForm.useSeparateConnectIP) {
          pageState.installForm.connectIp = pageState.installForm.pairIp
        }
        renderApp()
        return
      }
      if (command === "wizard-select-device") {
        pageState.installerState = await invoke("SelectInstallerDevice", value)
        syncWizardWithInstallerState()
        hydrateStateFromSnapshot(pageState.snapshot)
        renderApp()
        return
      }
      if (command === "wizard-toggle-config-entry") {
        const entry = wizardConfigEntry(value)
        if (!entry) {
          return
        }
        entry.enabled = !entry.enabled
        persistInstallNetworkState()
        renderApp()
        return
      }
      if (command === "wizard-validate-entry") {
        await validateConfigEntryAction(value)
        return
      }
      if (command === "clear-backend-pairing") {
        await clearBackendPairingAction("beta")
        return
      }
      if (command === "clear-developer-pairing") {
        await clearBackendPairingAction("dev")
        return
      }
      if (command === "set-bind-scope") {
        pageState.networkForm.bindAll = value === "all"
        persistInstallNetworkState()
        renderApp()
        return
      }
      if (command === "install-watch-app") {
        await installWatchAppAction()
        return
      }
      if (command === "launch-watch-app") {
        await launchWatchAppAction()
        return
      }
    })
  }

  for (const node of document.querySelectorAll("[data-stop-click]")) {
    node.addEventListener("click", (event) => {
      event.stopPropagation()
    })
  }

  for (const node of document.querySelectorAll("[data-select-mode]")) {
    node.addEventListener("click", () => {
      pageState.networkMode = node.getAttribute("data-select-mode")
      renderApp()
    })
  }

  for (const node of document.querySelectorAll("[data-log-source]")) {
    node.addEventListener("click", () => {
      pageState.selectedLogSource = node.getAttribute("data-log-source")
      renderApp()
    })
  }

  for (const node of document.querySelectorAll("[data-settings-tab]")) {
    node.addEventListener("click", () => {
      const nextTab = node.getAttribute("data-settings-tab")
      pageState.settingsTab = nextTab
      renderApp()
    })
  }

  for (const node of document.querySelectorAll("[data-developer-access-mode]")) {
    node.addEventListener("click", () => {
      pageState.developerForm.accessMode = normalizeDeveloperAccessMode(node.getAttribute("data-developer-access-mode"))
      updateDeveloperBaseUrlFromAccessMode(pageState.developerForm.accessMode)
      persistDeveloperFormState()
      renderApp()
    })
  }

  for (const node of document.querySelectorAll("[data-network-interface]")) {
    node.addEventListener("change", (event) => {
      const selectedIP = event.target.value
      const selected = availableNetworkOptions().find((item) => item.ip === selectedIP)
      pageState.networkForm.selectedIp = selectedIP
      pageState.networkForm.selectedInterface = selected?.label || selectedIP
      renderApp()
    })
  }

  for (const node of document.querySelectorAll("[data-installer-device]")) {
    node.addEventListener("change", async (event) => {
      pageState.installerState = await invoke("SelectInstallerDevice", event.target.value)
      syncWizardWithInstallerState()
      hydrateStateFromSnapshot(pageState.snapshot)
      renderApp()
    })
  }

  for (const node of document.querySelectorAll("[data-bind]")) {
    node.addEventListener("input", (event) => {
      updateBoundDraft(node.getAttribute("data-bind"), event.target)
    })
    node.addEventListener("change", (event) => {
      const path = node.getAttribute("data-bind")
      updateBoundDraft(path, event.target)
      if (path.startsWith("developerForm.")) {
        if (path === "developerForm.hostAlias" && pageState.developerForm.accessMode === "emulator") {
          updateDeveloperBaseUrlFromAccessMode("emulator")
        }
        if (path === "developerForm.devBaseUrl") {
          pageState.developerForm.accessMode = developerAccessModeFromBaseUrl(event.target.value)
        }
        persistDeveloperFormState()
      }
      if (path.startsWith("wizard.configEntries.") || path.startsWith("networkForm.")) {
        persistInstallNetworkState()
      }
      touchWizardBoundField(path)
      renderApp()
    })
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

async function copyText(text) {
  try {
    if (window?.runtime?.ClipboardSetText) {
      await window.runtime.ClipboardSetText(text)
    } else {
      await navigator.clipboard.writeText(text)
    }
    showNotice("已复制到剪贴板。")
  } catch {
    showNotice("复制失败，请手动复制。")
  }
}

async function refreshState() {
  pageState.snapshot = await loadSnapshot()
  hydrateStateFromSnapshot(pageState.snapshot)
  pageState.backendLogs = await loadBackendLogs()
  pageState.installerState = await loadInstallerStatus()
  await loadDeveloperEnvironmentSnapshot({ ensure: developerIsRunning() })
  syncWizardWithInstallerState()
  renderApp()
  maybeShowTunnelExpiryNotice()
}

async function startBackendAction(options = {}) {
  const notifySuccess = options.notifySuccess !== false
  const notifyFailure = options.notifyFailure !== false
  const requestOverride = options.requestOverride || null
  const renderOptions = { preserveInteraction: options.preserveInteraction === true }
  const hydrateOptions = { preserveDrafts: options.preserveDrafts === true }
  try {
    const backendStatus = await invoke("StartBackendWithRequest", requestOverride || currentBackendRequest())
    pageState.snapshot.backend = backendStatus
    const refreshed = await loadSnapshot()
    pageState.snapshot = mergeSnapshot(refreshed, pageState.snapshot.backend)
    hydrateStateFromSnapshot(pageState.snapshot, hydrateOptions)
    syncInstallWizardDraft(pageState.snapshot)
    pageState.backendLogs = await loadBackendLogs()
    renderApp(renderOptions)
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
    pageState.snapshot.backend = await invoke("RestartBackendWithRequest", currentBackendRequest())
    const refreshed = await loadSnapshot()
    pageState.snapshot = mergeSnapshot(refreshed, pageState.snapshot.backend)
    hydrateStateFromSnapshot(pageState.snapshot)
    syncInstallWizardDraft(pageState.snapshot)
    pageState.backendLogs = await loadBackendLogs()
    renderApp()
    maybeShowTunnelExpiryNotice()
    showNotice("本机服务已重启。")
  } catch (error) {
    showNotice(`重启本机服务失败：${String(error)}`)
  }
}

async function runHealthCheckAction() {
  try {
    const result = await invoke("CheckHealthWithRequest", currentBackendRequest())
    pageState.healthCheckResult = {
      ok: Boolean(result?.ok),
      result: result?.message || "已完成"
    }
    const refreshed = await loadSnapshot()
    const backendStatus = backendStatusWithHealthResult(refreshed.backend, result)
    pageState.snapshot = mergeSnapshot(refreshed, backendStatus)
    hydrateStateFromSnapshot(pageState.snapshot)
    syncInstallWizardDraft(pageState.snapshot)
    pageState.backendLogs = await loadBackendLogs()
    const live = deriveLiveState(pageState.snapshot)
    pageState.globalHealthSummary = {
      ...pageState.globalHealthSummary,
      checking: false,
      lastCheckedAt: new Date().toISOString(),
      targets: {
        ...(pageState.globalHealthSummary.targets || {}),
        backend: backendTargetFromHealthResult(result, live.backendStatusNote)
      }
    }
    renderApp({ preserveInteraction: true })
    maybeShowTunnelExpiryNotice()
    showNotice(result?.ok ? "健康检查通过。" : `健康检查完成：${result?.message || "服务不可达"}`)
  } catch (error) {
    const result = {
      ok: false,
      message: `健康检查失败：${String(error)}`
    }
    pageState.healthCheckResult = {
      ok: false,
      result: "检查失败"
    }
    pageState.snapshot = mergeSnapshot(pageState.snapshot, backendStatusWithHealthResult(pageState.snapshot.backend, result))
    pageState.globalHealthSummary = {
      ...pageState.globalHealthSummary,
      checking: false,
      lastCheckedAt: new Date().toISOString(),
      targets: {
        ...(pageState.globalHealthSummary.targets || {}),
        backend: backendTargetFromHealthResult(result)
      }
    }
    renderApp({ preserveInteraction: true })
    showNotice(result.message)
  }
}

async function redeemManagedTunnelAction() {
  const code = pageState.networkForm.tunnelCode.trim()
  if (!code) {
    showNotice("请先输入配置码。")
    return
  }
  try {
    pageState.snapshot = await invoke("RedeemManagedTunnelCode", code)
    pageState.networkForm.tunnelCode = ""
    hydrateStateFromSnapshot(pageState.snapshot)
    syncInstallWizardDraft(pageState.snapshot)
    pageState.backendLogs = await loadBackendLogs()
    renderApp()
    showNotice("配置码兑换成功，已保存托管隧道绑定。")
  } catch (error) {
    await refreshState()
    showNotice(`配置码兑换失败：${String(error)}`)
  }
}

async function redeemDeveloperTunnelAction() {
  const code = pageState.developerForm.tunnelCode.trim()
  if (!code) {
    showNotice("请先输入开发隧道配置码。")
    return
  }
  try {
    pageState.developerSnapshot = await invoke("RedeemDeveloperTunnelCode", code)
    pageState.developerForm.tunnelCode = ""
    persistDeveloperFormState()
    hydrateDeveloperStateFromSnapshot(pageState.developerSnapshot)
    if (developerIsRunning() && developerTunnelIsManaged()) {
      await loadDeveloperEnvironmentSnapshot({ ensure: true })
    }
    if (pageState.developerForm.accessMode === "tunnel") {
      updateDeveloperBaseUrlFromAccessMode("tunnel")
      persistDeveloperFormState()
    }
    renderApp()
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

async function toggleDeveloperEnvironmentAction() {
  if (pageState.developerAction.busy) {
    return
  }
  const nextEnabled = !developerIsRunning()
  pageState.developerAction = {
    busy: true,
    targetEnabled: nextEnabled,
    label: nextEnabled ? "正在执行启动脚本，请稍候。" : "正在停止当前开发环境。"
  }
  renderApp()
  try {
    if (nextEnabled) {
      let snapshot = await invoke("EnsureDeveloperEnvironment", developerEnvironmentRequest({ enabled: true }))
      pageState.developerSnapshot = snapshot
      pageState.developerRepositories = Array.isArray(snapshot?.repositories) ? snapshot.repositories : []
      hydrateDeveloperStateFromSnapshot(snapshot)
      const deadlineAt = Date.now() + DEVELOPER_STARTUP_WAIT_MS
      while (!snapshot?.status?.lastHealth?.ok && Date.now() < deadlineAt) {
        await wait(800)
        snapshot = await invoke("GetDeveloperEnvironmentSnapshot", developerEnvironmentRequest({ enabled: true }))
        pageState.developerSnapshot = snapshot
        pageState.developerRepositories = Array.isArray(snapshot?.repositories) ? snapshot.repositories : []
        hydrateDeveloperStateFromSnapshot(snapshot)
        renderApp()
      }
      const healthy = Boolean(snapshot?.status?.lastHealth?.ok)
      const stillStarting = Boolean(snapshot?.status?.running) && !healthy
      if (!healthy && stillStarting) {
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
      pageState.developerSnapshot = await invoke("StopDeveloperEnvironment")
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
    pageState.developerAction = {
      busy: false,
      targetEnabled: developerIsRunning(),
      label: ""
    }
    renderApp()
  }
}

async function restartDeveloperEnvironmentAction() {
  if (pageState.developerAction.busy) {
    return
  }
  if (developerIsRunning()) {
    pageState.developerSnapshot = await invoke("StopDeveloperEnvironment")
    await wait(250)
  }
  await toggleDeveloperEnvironmentAction()
}

async function prepareWatchBootstrapAction() {
  try {
    const payload = await invoke("PrepareWatchBootstrap", currentBackendRequest())
    pageState.generatedBootstrap = payload
    await copyText(payload.bootstrapUri)
    pageState.installerState = await loadInstallerStatus()
    syncWizardWithInstallerState()
    showNotice(`已复制 bootstrap URI，token 指纹 ${payload.tokenFingerprint}。`)
  } catch (error) {
    showNotice(`生成手表配置失败：${String(error)}`)
  }
}

async function prepareDevWatchBootstrapAction() {
  try {
    const baseURL = developerBaseUrlForCurrentForm() || suggestedDevBaseUrl()
    pageState.developerForm.devBaseUrl = baseURL
    persistDeveloperFormState()
    const payload = await invoke("PrepareDevWatchBootstrap", {
      baseURL,
      deviceName: pageState.developerForm.deviceName.trim(),
      repoPath: pageState.developerForm.repoPath.trim(),
      hostAlias: currentDeveloperHostAlias(),
      managedTunnelEnabled: Boolean(pageState.developerForm.managedTunnelEnabled)
    })
    pageState.generatedBootstrap = payload
    await copyText(payload.bootstrapUri)
    showNotice(`已复制 dev bootstrap URI，token 指纹 ${payload.tokenFingerprint}。`)
  } catch (error) {
    showNotice(`生成开发环境配置失败：${String(error)}`)
  }
}

async function submitRemoteWatchBootstrapAction() {
  const live = deriveLiveState(pageState.snapshot)
  const form = pageState.remoteBootstrapForm
  const bootstrapCode = form.bootstrapCode.trim().toUpperCase().replace(/[\s-]/g, "")
  if (!bootstrapCode) {
    showNotice("请先填写手表上的临时配置码。")
    return
  }
  const apiBase = form.apiBase.trim() || remoteBootstrapDefaultApiBase(live)
  if (!apiBase && !form.tunnelCode.trim()) {
    showNotice("请填写 API 基址，或填写隧道配置码。")
    return
  }

  form.submitting = true
  form.result = null
  renderApp()
  try {
    const result = await invoke("SubmitRemoteWatchBootstrap", {
      bootstrapCode,
      environment: form.environment === "dev" ? "dev" : "beta",
      apiBase,
      tunnelCode: form.tunnelCode.trim()
    })
    pageState.remoteBootstrapForm = {
      ...pageState.remoteBootstrapForm,
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
        pageState.snapshot = await loadSnapshot()
        hydrateStateFromSnapshot(pageState.snapshot)
      }
    }
    renderApp()
    showNotice(result?.message || "已提交临时配置，等待手表获取。")
  } catch (error) {
    form.submitting = false
    renderApp()
    showNotice(`发送临时配置失败：${String(error)}`)
  }
}

async function refreshInstallerState() {
  pageState.installerState = await loadInstallerStatus()
  syncWizardWithInstallerState()
  hydrateStateFromSnapshot(pageState.snapshot)
  renderApp()
}

function shortTimeLabel(date = new Date()) {
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit"
  }).format(date)
}

function wait(ms) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function touchWizardBoundField(path) {
  if (path === "installForm.pairIp" && !pageState.installForm.useSeparateConnectIP) {
    pageState.installForm.connectIp = pageState.installForm.pairIp
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

function buildBackendRequestForEntry(entryId, options = {}) {
  const selectedDevice = preferredBackendDevice()
  const isEmulator = Boolean(selectedDevice?.isEmulator)
  const emulatorHostAlias = selectedDevice?.hostAlias || "10.0.2.2"
  const endpoints = options.endpoints || []
  if (entryId === "lan") {
    const target = new URL(wizardConfigEntry("lan").url)
    const effectivePort = target.port || DEFAULT_SIDECAR_PORT
    return {
      mode: "lan",
      selectedIP: isEmulator ? "127.0.0.1" : target.hostname,
      bindAll: false,
      port: effectivePort,
      customURL: "",
      tunnelCode: "",
      deviceName: pageState.watchForm.deviceName,
      publicBaseURL: isEmulator ? `http://${emulatorHostAlias}:${effectivePort}` : "",
      endpoints
    }
  }
  if (entryId === "public") {
    return {
      mode: "public",
      selectedIP: currentSelectedIP(),
      bindAll: false,
      port: pageState.networkForm.port || DEFAULT_SIDECAR_PORT,
      customURL: wizardConfigEntry("public").url.trim(),
      tunnelCode: "",
      deviceName: pageState.watchForm.deviceName,
      publicBaseURL: "",
      endpoints
    }
  }
  return {
    mode: "tunnel",
    selectedIP: currentSelectedIP(),
    bindAll: false,
    port: DEFAULT_SIDECAR_PORT,
    customURL: "",
    tunnelCode: "",
    deviceName: pageState.watchForm.deviceName,
    publicBaseURL: "",
    endpoints
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
    renderApp()
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

function syncLegacyNetworkFormWithEntry(entryId) {
  if (entryId === "lan") {
    const target = new URL(wizardConfigEntry("lan").url)
    pageState.networkMode = "lan"
    pageState.networkForm.selectedIp = target.hostname
    pageState.networkForm.selectedInterface = `手动填写 · ${target.hostname}`
    pageState.networkForm.port = target.port || DEFAULT_SIDECAR_PORT
    return
  }
  if (entryId === "public") {
    pageState.networkMode = "public"
    pageState.networkForm.customUrl = wizardConfigEntry("public").url.trim()
    return
  }
  pageState.networkMode = "tunnel"
}

async function refreshPrepareChecksAction() {
  await refreshInstallerState()
  pageState.wizard.lastRefreshLabel = `已刷新 ${shortTimeLabel()}`
  renderApp()
  showNotice("已刷新自动检查。")
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
  if (pageState.pairingAction.busy) {
    return
  }
  pageState.pairingAction = {
    busy: true,
    scope
  }
  renderApp()
  try {
    clearWizardStageError()
    if (scope === "dev") {
      pageState.developerSnapshot = await invoke("ClearDeveloperPairing")
      pageState.developerRepositories = Array.isArray(pageState.developerSnapshot?.repositories) ? pageState.developerSnapshot.repositories : pageState.developerRepositories
      hydrateDeveloperStateFromSnapshot(pageState.developerSnapshot)
      pageState.generatedBootstrap = null
      renderApp()
      showNotice("已重置开发环境 dev 配对。")
      return
    }

    await invoke("ClearBackendPairing")
    pageState.snapshot = await loadSnapshot()
    pageState.backendLogs = await loadBackendLogs()
    syncInstallWizardDraft(pageState.snapshot)
    pageState.healthCheckResult = null
    pageState.generatedBootstrap = null
    if (pageState.currentPage === "install") {
      pageState.wizard.stageNotes.config = "本机配对已清空，可以重新写入配置。"
    }
    renderApp()
    showNotice("已清空本机服务 beta 配对。")
  } catch (error) {
    showNotice(`${scope === "dev" ? "重置开发环境配对" : "清空 beta 配对"}失败：${String(error)}`)
  } finally {
    pageState.pairingAction = {
      busy: false,
      scope: ""
    }
    renderApp()
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
      renderApp()
      const health = await invoke("CheckHealthWithRequest", buildBackendRequestForEntry(entryId))
      if (!health?.ok) {
        entry.validation = "error"
        entry.message = health?.message || "地址检查未通过"
        renderApp()
        if (!quiet) {
          showNotice(entry.message)
        }
        return false
      }
      entry.validation = "valid"
      entry.message = entryId === "lan" ? "同一网络内可用" : "已通过访问检查"
      entry.lastCheckedURL = raw
      renderApp()
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
        renderApp()
        if (!quiet) {
          showNotice(entry.message)
        }
        return false
      }
      entry.validation = "checking"
      entry.message = "正在兑换配置码"
      renderApp()
      pageState.snapshot = await invoke("RedeemManagedTunnelCode", code)
      pageState.backendLogs = await loadBackendLogs()
      hydrateStateFromSnapshot(pageState.snapshot)
      syncInstallWizardDraft(pageState.snapshot)
      entry.validation = "idle"
      entry.message = "已兑换，可继续校验"
    }

    const health = await checkTunnelHealthWithRetry(entry)
    if (!health?.ok) {
      entry.validation = "error"
      entry.message = health?.message || "托管隧道检查未通过"
      renderApp()
      if (!quiet) {
        showNotice(entry.message)
      }
      return false
    }
    entry.validation = "valid"
    entry.message = "已通过访问检查"
    entry.lastCheckedURL = entry.redeemedDomain
    renderApp()
    if (!quiet) {
      showNotice("托管隧道已通过检查。")
    }
    return true
  } catch (error) {
    entry.validation = "error"
    entry.message = String(error)
    renderApp()
    if (!quiet) {
      showNotice(`检查失败：${String(error)}`)
    }
    return false
  }
}

function resolveConfigWriteTarget() {
  const priorities = ["lan", "public", "tunnel"]
  return priorities.find((entryId) => wizardConfigEntry(entryId)?.enabled) || ""
}

async function handleWizardPrimaryAction() {
  const stage = pageState.wizard.currentStage
  const live = deriveLiveState(pageState.snapshot)
  if (stage === "prepare") {
    if (!wizardPrepareReady(live)) {
      showNotice("请先完成自动检查和手动确认。")
      return
    }
    markWizardStageCompleted("prepare", "环境已确认。")
    setWizardStage("connect", "开始填写手表连接信息。")
    renderApp()
    return
  }
  if (stage === "connect") {
    if (!wizardConnectReady()) {
      showNotice("请先点击开始连接，并确认目标设备。")
      return
    }
    markWizardStageCompleted("connect", pageState.wizard.stageNotes.connect || "手表已连接。")
    setWizardStage("install", "确认安装包后继续。")
    clearWizardStageError()
    renderApp()
    return
  }
  if (stage === "install") {
    if (!wizardInstallReady()) {
      showNotice("请先安装应用或打开已安装应用。")
      return
    }
    markWizardStageCompleted("install", pageState.wizard.stageNotes.install || "应用已安装并启动完成。")
    setWizardStage("config", "开始准备写入配置。")
    clearWizardStageError()
    renderApp()
    return
  }
  if (!wizardConfigReady()) {
    showNotice("请先写入配置。")
    return
  }
  pageState.currentPage = "watch"
  renderApp()
  showNotice("配置已完成，已切换到手表设备页。")
}

function handleWizardSecondaryAction() {
  const stage = pageState.wizard.currentStage
  if (stage === "prepare") {
    void refreshPrepareChecksAction()
    return
  }
  const backMap = {
    connect: "prepare",
    install: "connect",
    config: "install"
  }
  clearWizardStageError()
  setWizardStage(backMap[stage] || "prepare")
  renderApp()
}

async function retryCurrentWizardStage() {
  clearWizardStageError()
  renderApp()
  await handleWizardPrimaryAction()
}

async function runConnectStageAction() {
  if (!pairingSkipReason()) {
    if (!pageState.installForm.pairIp.trim() || !pageState.installForm.pairPort.trim() || !pageState.installForm.connectPort.trim() || !pageState.installForm.pairingCode.trim()) {
      setWizardStageError("connect", "请先填写手表 IP、配对端口、连接端口和配对码。")
      renderApp()
      showNotice("请先补全手表连接信息。")
      return false
    }
  }

  try {
    clearWizardStageError()
    renderApp()
    pageState.installerState = await invoke("RunADBPairing", {
      pairIP: pageState.installForm.pairIp.trim(),
      pairPort: pageState.installForm.pairPort.trim(),
      pairingCode: pageState.installForm.pairingCode.trim(),
      connectIP: (pageState.installForm.useSeparateConnectIP ? pageState.installForm.connectIp : pageState.installForm.pairIp).trim(),
      connectPort: pageState.installForm.connectPort.trim(),
      selectedSerial: pageState.installerState?.selectedSerial || ""
    })
    syncWizardWithInstallerState()
    if (pageState.installerState?.phase === "troubleshooting") {
      setWizardStageError("connect", pageState.installerState?.message || "无法连接到手表。")
      renderApp()
      showNotice(pageState.installerState?.message || "无法连接到手表。")
      return false
    }
    if (installerSelectionRequired()) {
      pageState.wizard.stageNotes.connect = "检测到多个设备，请先确认这次要安装的目标手表。"
      renderApp()
      showNotice("请先选择目标设备。")
      return false
    }
    if (!wizardConnectReady()) {
      setWizardStageError("connect", "当前还没有确认目标设备。")
      renderApp()
      showNotice("当前还没有确认目标设备。")
      return false
    }
    markWizardStageCompleted("connect", pageState.installerState?.message || "手表已连接。")
    pageState.wizard.stageNotes.connect = pageState.installerState?.message || "手表已连接。"
    clearWizardStageError()
    renderApp()
    showNotice(pageState.installerState?.message || "手表已连接。")
    return true
  } catch (error) {
    setWizardStageError("connect", String(error))
    renderApp()
    showNotice(`连接手表失败：${String(error)}`)
    return false
  }
}

async function runInstallStageAction() {
  if (!selectedInstallerDevice()) {
    setWizardStageError("install", "请先回到上一页确认目标设备。")
    renderApp()
    showNotice("请先确认目标设备。")
    return false
  }
  try {
    clearWizardStageError()
    renderApp()
    if (!pageState.installerState?.apk?.installed) {
      pageState.installerState = await invoke("InstallWatchApp", pageState.installerState?.selectedSerial || "")
      syncWizardWithInstallerState()
      if (pageState.installerState?.phase === "troubleshooting") {
        setWizardStageError("install", pageState.installerState?.message || "安装失败。")
        renderApp()
        showNotice(pageState.installerState?.message || "安装失败。")
        return false
      }
    }
    pageState.installerState = await invoke("LaunchWatchApp", pageState.installerState?.selectedSerial || "")
    syncWizardWithInstallerState()
    if (pageState.installerState?.phase === "troubleshooting") {
      setWizardStageError("install", pageState.installerState?.message || "应用启动失败。")
      renderApp()
      showNotice(pageState.installerState?.message || "应用启动失败。")
      return false
    }
    markWizardStageCompleted("install", pageState.installerState?.message || "应用安装并启动完成。")
    pageState.wizard.stageNotes.install = pageState.installerState?.message || "应用已安装并启动完成。"
    clearWizardStageError()
    renderApp()
    showNotice(pageState.installerState?.message || "应用已安装并启动。")
    return true
  } catch (error) {
    setWizardStageError("install", String(error))
    renderApp()
    showNotice(`安装应用失败：${String(error)}`)
    return false
  }
}

async function runConfigStageAction() {
  if (wizardConfigChecksInProgress()) {
    showNotice(`${wizardConfigCheckingLabel()} 正在检查，完成后才能写入。`)
    return false
  }
  if (!selectedInstallerDevice()) {
    setWizardStageError("config", "请先回到上一页确认目标设备。")
    renderApp()
    showNotice("请先确认目标设备。")
    return false
  }
  const enabledEntries = wizardEnabledConfigEntries()
  if (enabledEntries.length === 0) {
    setWizardStageError("config", "请至少启用一个要写入的地址。")
    renderApp()
    showNotice("请至少启用一个要写入的地址。")
    return false
  }

  for (const [entryId] of enabledEntries) {
    const ok = await validateConfigEntryAction(entryId, { quiet: true })
    if (!ok) {
      setWizardStageError("config", `${INSTALL_CONFIG_META[entryId].label} 还没有通过检查。`)
      renderApp()
      showNotice(`${INSTALL_CONFIG_META[entryId].label} 还没有通过检查。`)
      return false
    }
  }

  const writeTarget = resolveConfigWriteTarget()
  if (!writeTarget) {
    setWizardStageError("config", "当前没有可写入的地址。")
    renderApp()
    showNotice("当前没有可写入的地址。")
    return false
  }

  const bootstrapEndpoints = buildEnabledBootstrapEndpoints()
  syncLegacyNetworkFormWithEntry(writeTarget)
  const labels = enabledEntries.map(([entryId]) => INSTALL_CONFIG_META[entryId].label)
  const ok = await applyWatchBootstrapAction(buildBackendRequestForEntry(writeTarget, { endpoints: bootstrapEndpoints }), {
    successNotice: `已发送 ${labels.join("、")} 配置链接，请在手表上确认。`,
    failureStage: "config"
  })
  if (!ok) {
    return false
  }
  markWizardStageCompleted("config", `已发送 ${labels.join("、")} 配置链接，请在手表上确认。`)
  pageState.wizard.stageNotes.config = `已整理 ${labels.join("、")} 并发送到手表，等待手表确认。`
  renderApp()
  return true
}

async function installWatchAppAction() {
  try {
    pageState.installerState = await invoke("InstallWatchApp", pageState.installerState?.selectedSerial || "")
    syncWizardWithInstallerState()
    renderApp()
    showNotice(pageState.installerState?.message || "安装动作已执行。")
  } catch (error) {
    showNotice(`安装失败：${String(error)}`)
  }
}

async function launchWatchAppAction() {
  try {
    pageState.installerState = await invoke("LaunchWatchApp", pageState.installerState?.selectedSerial || "")
    syncWizardWithInstallerState()
    renderApp()
    showNotice(pageState.installerState?.message || "应用已启动。")
  } catch (error) {
    showNotice(`启动失败：${String(error)}`)
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
    const result = await invoke("BootstrapWatchOnDevice", request, pageState.installerState?.selectedSerial || "")
    pageState.installerState = result?.installer || pageState.installerState
    pageState.generatedBootstrap = result?.payload || pageState.generatedBootstrap
    syncWizardWithInstallerState()
    if (pageState.installerState?.phase === "troubleshooting") {
      throw new Error(pageState.installerState?.message || "写入失败。")
    }
    clearWizardStageError()
    renderApp()
    showNotice(pageState.installerState?.message || successNotice)
    return true
  } catch (error) {
    if (failureStage) {
      setWizardStageError(failureStage, String(error))
      renderApp()
    }
    showNotice(`写入设置失败：${String(error)}`)
    return false
  }
}

async function applyDevWatchBootstrapAction() {
  try {
    const baseURL = developerBaseUrlForCurrentForm() || suggestedDevBaseUrl()
    pageState.developerForm.devBaseUrl = baseURL
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
      deviceName: pageState.developerForm.deviceName.trim(),
      repoPath: pageState.developerForm.repoPath.trim(),
      hostAlias: currentDeveloperHostAlias()
    }, pageState.installerState?.selectedSerial || "")
    pageState.installerState = result?.installer || pageState.installerState
    pageState.generatedBootstrap = result?.payload || pageState.generatedBootstrap
    syncWizardWithInstallerState()
    renderApp()
    showNotice(pageState.installerState?.message || "已发送开发环境到手表，请在手表确认。")
  } catch (error) {
    showNotice(`发送开发环境失败：${String(error)}`)
  }
}

async function verifyWatchStatusAction() {
  try {
    pageState.installerState = await invoke("VerifyWatchStatus", pageState.installerState?.selectedSerial || "")
    syncWizardWithInstallerState()
    renderApp()
    showNotice(pageState.installerState?.message || "手表状态已校验。")
  } catch (error) {
    showNotice(`校验手表状态失败：${String(error)}`)
  }
}

async function openFolderAction(method, successNotice) {
  try {
    const errorMessage = await invoke(method)
    if (errorMessage) {
      showNotice(errorMessage)
      return
    }
    showNotice(successNotice)
  } catch (error) {
    showNotice(`打开目录失败：${String(error)}`)
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

function maybeShowTunnelExpiryNotice() {
  const tunnel = currentTunnel()
  if (!tunnel.tokenExpired) {
    pageState.tunnelExpiryNoticeKey = ""
    return
  }
  const noticeKey = `${tunnel.tunnelId}:${tunnel.tokenVersion}:${tunnel.message}`
  if (pageState.tunnelExpiryNoticeKey === noticeKey) {
    return
  }
  pageState.tunnelExpiryNoticeKey = noticeKey
  showNotice("远程访问隧道已失效，请联系管理员重新绑定设备。")
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
}

async function bootstrap() {
  const requestedPage = new URLSearchParams(window.location.search).get("page") || window.location.hash.replace(/^#/, "")
  if (validPageIds.has(requestedPage)) {
    pageState.currentPage = requestedPage
  }
  pageState.developerAcknowledged = loadDeveloperAcknowledgedState()
  const savedDeveloperForm = loadDeveloperFormState()
  if (savedDeveloperForm) {
    pageState.developerForm = {
      ...pageState.developerForm,
      ...savedDeveloperForm,
      deviceName: savedDeveloperForm.deviceName || pageState.developerForm.deviceName
    }
  }
  const savedInstallNetwork = loadInstallNetworkState()
  if (savedInstallNetwork) {
    if (savedInstallNetwork.networkMode) {
      pageState.networkMode = savedInstallNetwork.networkMode
    }
    if (savedInstallNetwork.networkForm?.customUrl) {
      pageState.networkForm.customUrl = savedInstallNetwork.networkForm.customUrl
    }
    if (savedInstallNetwork.networkForm?.port) {
      pageState.networkForm.port = savedInstallNetwork.networkForm.port
    }
    if (savedInstallNetwork.configEntries && typeof savedInstallNetwork.configEntries === "object") {
      pageState.wizard.configEntries = {
        ...pageState.wizard.configEntries,
        ...savedInstallNetwork.configEntries
      }
    }
  }
  pageState.snapshot = await loadSnapshot()
  hydrateStateFromSnapshot(pageState.snapshot)
  pageState.backendLogs = await loadBackendLogs()
  pageState.installerState = await loadInstallerStatus()
  await loadDeveloperEnvironmentSnapshot({ ensure: false })
  syncWizardWithInstallerState()
  renderApp()
  await maybeAutoStartBackend()
  ensureGlobalHealthTicker()
  await runGlobalHealthCheck({ manual: false })
}

bootstrap()

async function maybeAutoStartBackend() {
  if (pageState.backendAutoStartAttempted) {
    return
  }
  pageState.backendAutoStartAttempted = true
  if (pageState.snapshot?.backend?.running) {
    return
  }
  try {
    pageState.snapshot.backend = await invoke("StartBackend")
    const refreshed = await loadSnapshot()
    pageState.snapshot = mergeSnapshot(refreshed, pageState.snapshot.backend)
    hydrateStateFromSnapshot(pageState.snapshot)
    syncInstallWizardDraft(pageState.snapshot)
    pageState.backendLogs = await loadBackendLogs()
    renderApp()
  } catch {
    // 启动失败时保留当前快照，由显式操作再给出错误提示。
  }
}
