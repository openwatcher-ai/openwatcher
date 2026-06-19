export async function invoke(method, ...args) {
  const target = window?.go?.main?.App?.[method]
  if (target) {
    return target(...args)
  }
  const fallback = fallbackHandlers[method]
  if (fallback) {
    return fallback(...args)
  }
  throw new Error(`Wails 绑定缺少 ${method}`)
}

export async function copyToClipboard(text) {
  if (window?.runtime?.ClipboardSetText) {
    return window.runtime.ClipboardSetText(text)
  }
  return navigator.clipboard.writeText(text)
}

export function toggleWindowMaximise() {
  return window?.runtime?.WindowToggleMaximise?.()
}

export function onRuntimeEvent(eventName, callback) {
  if (window?.runtime?.EventsOn) {
    return window.runtime.EventsOn(eventName, callback)
  }
  return () => {}
}

const mockSnapshot = {
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
    state: "running",
    message: "OpenWatcher 本机服务已启动。",
    running: true,
    recentLogCount: 3,
    healthProbePath: "http://127.0.0.1:8787/healthz",
    configPathLabel: "~/.openwatcher/config.json",
    configuredListen: "127.0.0.1:8787",
    configuredPublicBaseUrl: "http://127.0.0.1:8787",
    lastHealth: {
      ok: true,
      message: "OK",
      config: {
        listen: "0.0.0.0:8787",
        publicBaseUrl: "http://192.168.31.12:8787"
      },
      build: {
        version: "dev"
      }
    }
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
  accessMode: "局域网模式",
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

const mockInstallerState = {
  adb: {
    available: true,
    path: "/mock/platform-tools/adb",
    version: "Android Debug Bridge version 1.0.41",
    message: "ADB 已就绪"
  },
  devices: [
    {
      serial: "192.168.31.88:40221",
      displayName: "Xiaomi Watch",
      model: "M2233W1",
      product: "Xiaomi",
      device: "arm64-v8a",
      state: "device",
      isWatch: true,
      isEmulator: false
    }
  ],
  selectedSerial: "192.168.31.88:40221",
  selectedLabel: "Xiaomi Watch",
  selectedPort: 40221,
  apk: {
    available: true,
    installed: true,
    path: "/mock/openwatcher-watch-release.apk",
    label: "openwatcher-watch-release.apk",
    versionName: "dev-watch",
    versionCode: 12,
    installedVersionName: "dev-watch",
    installedVersionCode: 12,
    packageName: "ai.openwatcher.watchapp",
    sha256: "9b06f7f6e43a4c6d3f2f1f62d78f2e201fd55a5a782f779c7fc45b4ce3f5f3c2",
    debug: false,
    devFallback: false,
    message: "release APK 已就绪"
  },
  phase: "idle",
  message: "手表已连接。",
  logs: [
    { at: "10:24:33", source: "adb", message: "connected to 192.168.31.88:40221" },
    { at: "10:24:35", source: "adb", message: "APK installed successfully" }
  ]
}

const mockDeveloperSnapshot = {
  repositories: [],
  status: {
    state: "stopped",
    running: false,
    message: "开发环境未启动",
    baseURL: "http://10.0.2.2:18787",
    lastCheckedAt: new Date().toISOString(),
    resolvedRepoPath: "",
    resolvedScriptPath: "scripts/start-local.sh",
    startCommand: "scripts/start-local.sh",
    envFilePresent: false,
    externallyManaged: false,
    startedAt: "",
    lastHealth: null,
    logFileLabel: ""
  },
  tunnel: {
    configured: false,
    running: false,
    publicBaseUrl: "",
    message: "未激活开发隧道"
  },
  logs: []
}

const fallbackHandlers = {
  GetDesktopSettings: async () => ({
    autoStartBackend: true,
    developerEnvironment: {
      enabled: false,
      mode: "workspace",
      repoPath: "",
      baseUrl: "http://10.0.2.2:18787",
      deviceName: "watch",
      hostAlias: "10.0.2.2",
      managedTunnelEnabled: false
    }
  }),
  GetSnapshot: async () => structuredClone(mockSnapshot),
  GetBackendLogs: async () => [
    { at: "10:24:40", message: "[backend] Health check OK" },
    { at: "10:24:41", message: "[network] GET /api/status 200 OK" }
  ],
  CheckHealth: async () => ({
    ok: true,
    message: "OK",
    endpoint: "http://127.0.0.1:8787/healthz",
    config: {
      listen: "127.0.0.1:8787",
      publicBaseUrl: "http://127.0.0.1:8787"
    },
    build: {
      version: "dev"
    }
  }),
  CheckForUpdates: async (currentWatchVersion) => ({
    channel: "beta",
    checkedAt: new Date().toISOString(),
    releaseTag: "beta-2026.06.13.1",
    releaseSummary: "修复桌面更新检查与交互反馈",
    releaseUrl: "https://github.com/openwatcher-ai/openwatcher/releases/tag/beta-2026.06.13.1",
    notesUrl: "https://openwatcher.ai/changelog/beta-2026.06.13.1.json",
    currentDesktopVersion: "dev",
    latestDesktopVersion: "dev-next",
    desktopUpdateAvailable: true,
    desktopDownloadUrl: "https://example.com/desktop_v0.1.0_macos_arm64.zip",
    desktopArtifact: "desktop_v0.1.0_macos_arm64.zip",
    desktopSha256: "fixture-sha",
    desktopSizeBytes: 42881234,
    desktopArchiveKind: "zip",
    desktopInstallable: true,
    currentWatchVersion: currentWatchVersion || "dev-watch",
    latestWatchVersion: "dev-watch-next",
    watchUpdateAvailable: Boolean(currentWatchVersion),
    watchDownloadUrl: "https://example.com/openwatcher-watchapp.apk",
    releaseNotes: [
      { component: "桌面应用", text: "修复桌面更新检查与交互反馈" },
      { component: "运行时依赖", text: "更新内置 helper 与运行资源处理" }
    ]
  }),
  GetDesktopUpdateStatus: async () => ({
    phase: "",
    message: ""
  }),
  InstallDesktopUpdate: async () => ({
    phase: "restarting",
    message: "更新程序已启动，Desktop 将自动重启",
    version: "dev-next",
    artifact: "desktop_v0.1.0_macos_arm64.zip"
  }),
  GetInstallerStatus: async () => structuredClone(mockInstallerState),
  GetDeveloperEnvironmentSnapshot: async () => structuredClone(mockDeveloperSnapshot),
  EnsureRuntimeDependencies: async () => structuredClone(mockSnapshot),
  CheckHealthWithRequest: async () => ({ ok: true, message: "OK" }),
  StartBackend: async () => structuredClone(mockSnapshot.backend),
  StartBackendWithRequest: async () => structuredClone(mockSnapshot.backend),
  RestartBackendWithRequest: async () => structuredClone(mockSnapshot.backend),
  SelectInstallerDevice: async (_, serial) => ({ ...structuredClone(mockInstallerState), selectedSerial: serial }),
  RunADBPairing: async () => structuredClone(mockInstallerState),
  InstallWatchApp: async () => structuredClone(mockInstallerState),
  LaunchWatchApp: async () => structuredClone(mockInstallerState),
  PrepareWatchBootstrap: async (_, request) => ({
    deviceName: request?.deviceName || "watch",
    apiBase: request?.publicBaseURL || request?.customURL || "http://192.168.31.12:8787",
    tokenFingerprint: "mock-1234",
    bootstrapUri: "openwatcher://bootstrap/mock",
    createdAt: new Date().toISOString()
  }),
  BootstrapWatchOnDevice: async (_, request) => ({
    installer: structuredClone(mockInstallerState),
    payload: {
      deviceName: request?.deviceName || "watch",
      apiBase: request?.publicBaseURL || request?.customURL || "http://192.168.31.12:8787",
      tokenFingerprint: "mock-1234",
      bootstrapUri: "openwatcher://bootstrap/mock",
      createdAt: new Date().toISOString()
    }
  }),
  PrepareDevWatchBootstrap: async (_, request) => ({
    deviceName: request?.deviceName || "watch",
    apiBase: request?.baseURL || "http://10.0.2.2:18787",
    tokenFingerprint: "dev-mock",
    bootstrapUri: "openwatcher://bootstrap/dev-mock",
    createdAt: new Date().toISOString()
  }),
  BootstrapDevWatchOnDevice: async () => ({
    installer: structuredClone(mockInstallerState),
    payload: {
      deviceName: "watch",
      apiBase: "http://10.0.2.2:18787",
      tokenFingerprint: "dev-mock",
      bootstrapUri: "openwatcher://bootstrap/dev-mock",
      createdAt: new Date().toISOString()
    }
  }),
  ClearDeveloperEnvironmentLogs: async () => ({ ...structuredClone(mockDeveloperSnapshot), logs: [] }),
  StopDeveloperEnvironment: async () => structuredClone(mockDeveloperSnapshot),
  EnsureDeveloperEnvironment: async () => ({
    ...structuredClone(mockDeveloperSnapshot),
    status: {
      ...mockDeveloperSnapshot.status,
      state: "running",
      running: true,
      message: "开发环境已启动",
      startedAt: new Date().toISOString(),
      lastHealth: { ok: true, message: "OK" }
    },
    logs: [{ at: new Date().toISOString(), message: "dev server started" }]
  }),
  CopyDiagnostics: async () => "OpenWatcher mock diagnostics",
  ExportDiagnosticsBundle: async () => "/tmp/openwatcher-diagnostics.zip",
  SetAutoStartBackend: async (enabled) => ({ autoStartBackend: Boolean(enabled) }),
  GetCodexHookStatus: async () => ({
    codexHome: "~/.codex",
    hooksPath: "~/.codex/hooks.json",
    hooksPathLabel: "~/.codex/hooks.json",
    reviewLocation: "Codex App → 设置 → 钩子 → 来自用户配置",
    installed: false,
    changed: false,
    backupPath: "",
    message: "尚未安装 OpenWatcher Codex hooks。"
  }),
  InstallCodexHooks: async () => ({
    codexHome: "~/.codex",
    hooksPath: "~/.codex/hooks.json",
    hooksPathLabel: "~/.codex/hooks.json",
    reviewLocation: "Codex App → 设置 → 钩子 → 来自用户配置",
    installed: true,
    changed: true,
    backupPath: "~/.codex/hooks.json.openwatcher-backup-20260613-150000",
    message: "已写入 OpenWatcher hooks；信任状态请在 Codex App 中确认。"
  }),
  ClearBackendPairing: async () => true,
  ClearDeveloperPairing: async () => structuredClone(mockDeveloperSnapshot),
  OpenDesktopConfigDir: async () => "",
  OpenBackendConfigDir: async () => "",
  OpenCodexHome: async () => "",
  OpenCodexHooksFile: async () => "",
  OpenDeveloperLogFile: async () => "",
  OpenDeveloperEnvFile: async () => "",
  ChooseDeveloperRepositoryDir: async () => "examples/openwatcher"
}
