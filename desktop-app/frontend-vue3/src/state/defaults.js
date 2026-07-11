export const DEFAULT_SIDECAR_PORT = "8787"
export const DEFAULT_DEV_SIDECAR_PORT = "18787"
export const GLOBAL_HEALTH_INTERVAL_MS = 15000
export const DEVELOPER_STARTUP_WAIT_MS = 30000

export const navItems = [
  { id: "install", label: "安装向导", icon: "Wand2" },
  { id: "watch", label: "手表设备", icon: "Watch" },
  { id: "logs", label: "日志与诊断", icon: "FileText" },
  { id: "settings", label: "设置", icon: "Settings" }
]

export const settingsTabs = [
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

export const watchTabs = [
  { id: "overview", label: "概览" },
  { id: "app", label: "应用" },
  { id: "config", label: "配置" },
  { id: "connection", label: "连接" },
  { id: "remote", label: "远程初始化" },
  { id: "history", label: "设备记录" }
]

export const logTabs = [
  { id: "events", label: "事件" },
  { id: "raw", label: "原始日志" },
  { id: "diagnosis", label: "诊断建议" },
  { id: "export", label: "导出" }
]

export const prepareManualChecks = [
  { id: "wifi", label: "手表已连接 Wi-Fi，且与电脑同一局域网" },
  { id: "developer", label: "已启用开发者模式" },
  { id: "wireless", label: "已开启无线调试" },
  { id: "pairPage", label: "已打开“使用配对码配对设备”页面" }
]

export const installWizardStages = [
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

export const installConfigMeta = {
  lan: {
    label: "局域网",
    icon: "Globe2",
    inputLabel: "局域网地址",
    validateLabel: "重新检测",
    inactiveText: "启用后会一起发送到手表确认。"
  },
  public: {
    label: "公网",
    icon: "Link2",
    inputLabel: "公网地址",
    validateLabel: "重新校验",
    inactiveText: "启用后会一起发送到手表确认。"
  },
  tunnel: {
    label: "托管隧道",
    icon: "Cloud",
    inputLabel: "配置码",
    validateLabel: "兑换配置码",
    inactiveText: "先兑换，再决定是否一起写入。"
  }
}

export const fallbackSnapshot = {
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
    message: "未找到 OpenWatcher 本机服务组件。请把二进制放到 bundled/openwatcher/，或在仓库根目录生成 bin/openwatcher。",
    resolvedBinary: "",
    binarySource: "bundled/openwatcher/",
    friendlyError: "未找到 OpenWatcher 本机服务组件。",
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
        label: "Wi-Fi (en0) - 192.168.31.12",
        ip: "192.168.31.12",
        recommended: true
      }
    ],
    recommendedIp: "192.168.31.12",
    recommendedTag: "Wi-Fi (en0) - 192.168.31.12"
  }
}

export const fallbackInstallerState = {
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
  runtime: {
    platform: "",
    resources: {}
  },
  phase: "idle",
  message: "",
  logs: []
}

export const sampleEventTimeline = [
  { time: "10:24:33.128", event: "ADB 配对成功: 192.168.31.88:37153", level: "INFO", source: "ADB 安装" },
  { time: "10:24:33.682", event: "已连接手表: watch (192.168.31.88)", level: "INFO", source: "手表 App" },
  { time: "10:24:34.116", event: "开始安装 APK: openwatcher-watch-release.apk", level: "INFO", source: "ADB 安装" },
  { time: "10:24:35.209", event: "APK 安装成功", level: "SUCCESS", source: "ADB 安装" },
  { time: "10:24:36.771", event: "已发送 bootstrap 配置链接，等待手表确认", level: "INFO", source: "本机服务" },
  { time: "10:24:37.215", event: "手表确认配置: 配置已保存", level: "INFO", source: "手表 App" },
  { time: "10:24:37.998", event: "手表请求 /api/status 成功", level: "INFO", source: "网络访问" }
]

export const sampleRawLogs = [
  "[10:24:33.128] [INFO] [adb] Successfully paired with 192.168.31.88:37153",
  "[10:24:33.682] [INFO] [watch] Connected to watch (192.168.31.88)",
  "[10:24:34.116] [INFO] [adb] Installing APK: openwatcher-watch-release.apk",
  "[10:24:35.209] [SUCCESS] [adb] APK installed successfully",
  "[10:24:36.771] [INFO] [backend] Sent bootstrap configuration to watch",
  "[10:24:37.215] [INFO] [watch] Configuration acknowledged and saved",
  "[10:24:37.998] [INFO] [network] GET /api/status 200 OK (192.168.31.88)",
  "[10:24:40.512] [INFO] [backend] Health check 127.0.0.1:8787 OK"
]

export const sampleDevices = [
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

export function suggestedLanURL(snapshot = fallbackSnapshot) {
  const ip = snapshot?.networkContext?.recommendedIp || fallbackSnapshot.networkContext.recommendedIp
  const listen = snapshot?.backend?.lastHealth?.config?.listen || snapshot?.backend?.configuredListen || `127.0.0.1:${DEFAULT_SIDECAR_PORT}`
  const port = listen.split(":").pop() || DEFAULT_SIDECAR_PORT
  return ip ? `http://${ip}:${port}` : ""
}

export function createInstallConfigEntries(snapshot = fallbackSnapshot) {
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

export function createInitialState() {
  return {
    currentPage: "install",
    theme: "dark",
    snapshot: structuredClone(fallbackSnapshot),
    installerState: structuredClone(fallbackInstallerState),
    developerRepositories: [],
    developerSnapshot: null,
    backendLogs: [],
    copiedText: "",
    tunnelExpiryNoticeKey: "",
    backendAutoStartAttempted: false,
    globalHealthTicker: null,
    globalHealthRunning: false,
    backendHealthCheckRunning: false,
    installerRefreshRunning: false,
	    updateCheckRunning: false,
	    desktopUpdateRunning: false,
	    desktopUpdateProgress: null,
    codexHooksInstalling: false,
    codexHookState: {
      codexHome: "",
      hooksPath: "",
      hooksPathLabel: "~/.codex/hooks.json",
      reviewLocation: "Codex App → 设置 → 钩子 → 来自用户配置",
      installed: false,
      changed: false,
      backupPath: "",
      message: "尚未检测 Codex hooks。"
    },
    floatingWidget: {
      enabled: false,
      running: false,
      restartAttempts: 0,
      message: "尚未读取悬浮球状态。",
      busy: false,
      repairing: false,
      refreshing: false,
      checkedAt: ""
    },
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
    developerConfirmModalOpen: false,
    copyFeedbackKey: "",
    notificationCounter: 0,
    notifications: [],
    notificationPanelOpen: false,
    selectedLogSource: "all",
    networkMode: "lan",
    settingsTab: "general",
    settingsPreferences: {
      launchAtLogin: true,
      autoStartBackend: true,
      minimizeToTray: true,
      redactDiagnostics: true,
      anonymousCompatibilityReports: true,
      allowUpdateChecks: true,
      cleanupPairingLogs: true,
      useSystemAdb: true,
      developerLogs: true,
      showRawCommandOutput: false,
      experimentalManagedTunnel: true
    },
    watchTab: "overview",
    logsTab: "events",
    healthCheckResult: null,
	    updateCheckResult: null,
	    desktopUpdateStatus: null,
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
      selectedInterface: "Wi-Fi (en0) - 192.168.31.12",
      selectedIp: "192.168.31.12",
      port: DEFAULT_SIDECAR_PORT,
      customUrl: "https://openwatcher.example.com",
      tunnelCode: ""
    }
  }
}
