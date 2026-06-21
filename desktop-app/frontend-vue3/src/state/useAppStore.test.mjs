import test from "node:test"
import assert from "node:assert/strict"

import { createAppStore } from "./useAppStore.js"

function createLocalStorage() {
  const storage = new Map()
  return {
    getItem(key) {
      return storage.has(key) ? storage.get(key) : null
    },
    setItem(key, value) {
      storage.set(key, String(value))
    },
    removeItem(key) {
      storage.delete(key)
    }
  }
}

function installBrowserGlobals({ localStorage, appMethods, clipboard }) {
  Object.defineProperty(globalThis, "window", {
    value: {
      localStorage,
      setTimeout,
      clearTimeout,
      go: {
        main: {
          App: appMethods
        }
      }
    },
    configurable: true,
    writable: true
  })
  Object.defineProperty(globalThis, "navigator", {
    value: {
      clipboard
    },
    configurable: true
  })
}

test("autoStartBackend 切换会调用后端保存并写回本地偏好", async () => {
  const localStorage = createLocalStorage()
  const calls = []
  installBrowserGlobals({
    localStorage,
    appMethods: {
      SetAutoStartBackend: async (enabled) => {
        calls.push(["SetAutoStartBackend", enabled])
        return { autoStartBackend: enabled }
      }
    },
    clipboard: {
      writeText: async () => {}
    }
  })

  const store = createAppStore()
  await store.actions.toggleSettingsPreference("autoStartBackend")

  assert.deepEqual(calls, [["SetAutoStartBackend", false]])
  assert.equal(store.state.settingsPreferences.autoStartBackend, false)
  const saved = JSON.parse(localStorage.getItem("openwatcher-settings-preferences"))
  assert.equal(saved.autoStartBackend, false)
  assert.match(store.state.notifications[0]?.detail || "", /已关闭“启动后自动启动本机服务”/)
})

test("copyDiagnosticsAction 会调用后端诊断文本并复制到剪贴板", async () => {
  const localStorage = createLocalStorage()
  const calls = []
  let copied = ""
  installBrowserGlobals({
    localStorage,
    appMethods: {
      CopyDiagnostics: async () => {
        calls.push("CopyDiagnostics")
        return "完整脱敏诊断文本"
      }
    },
    clipboard: {
      writeText: async (text) => {
        copied = text
      }
    }
  })

  const store = createAppStore()
  await store.actions.copyDiagnosticsAction("logs-copy-diagnostics")

  assert.deepEqual(calls, ["CopyDiagnostics"])
  assert.equal(copied, "完整脱敏诊断文本")
  assert.equal(store.state.copiedText, "完整脱敏诊断文本")
  assert.equal(store.state.copyFeedbackKey, "logs-copy-diagnostics")
})

test("copyText 会写入剪贴板并激活按钮复制反馈", async () => {
  const localStorage = createLocalStorage()
  let copied = ""
  installBrowserGlobals({
    localStorage,
    appMethods: {},
    clipboard: {
      writeText: async (text) => {
        copied = text
      }
    }
  })

  const store = createAppStore()
  await store.actions.copyText("https://example.com", "dev-service-base-url")

  assert.equal(copied, "https://example.com")
  assert.equal(store.state.copiedText, "https://example.com")
  assert.equal(store.state.copyFeedbackKey, "dev-service-base-url")
})

test("runHealthCheckAction 会走本地 CheckHealth 而不是按前端网络模式拼请求", async () => {
  const localStorage = createLocalStorage()
  const calls = []
  installBrowserGlobals({
    localStorage,
    appMethods: {
      CheckHealth: async () => {
        calls.push("CheckHealth")
        return {
          ok: true,
          message: "OK",
          config: {
            listen: "127.0.0.1:8787",
            publicBaseUrl: "http://127.0.0.1:8787"
          },
          build: {
            version: "dev"
          }
        }
      },
      GetSnapshot: async () => ({
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
          running: true,
          message: "OpenWatcher 本机服务已启动。",
          configuredListen: "127.0.0.1:8787",
          configuredPublicBaseUrl: "http://127.0.0.1:8787"
        },
        tunnel: {
          configured: false,
          running: false,
          publicBaseUrl: ""
        },
        networkContext: {
          interfaces: [],
          recommendedIp: "192.168.31.12"
        }
      }),
      GetBackendLogs: async () => []
    },
    clipboard: {
      writeText: async () => {}
    }
  })

  const store = createAppStore()
  store.state.networkMode = "public"
  store.state.networkForm.customUrl = "https://not-used.example.com"
  await store.actions.runHealthCheckAction()

  assert.deepEqual(calls, ["CheckHealth"])
  assert.equal(store.state.healthCheckResult.ok, true)
  assert.match(store.state.notifications[0]?.detail || "", /健康检查通过/)
})

test("手动刷新设备列表会给出提示", async () => {
  const localStorage = createLocalStorage()
  installBrowserGlobals({
    localStorage,
    appMethods: {
      GetInstallerStatus: async () => ({
        adb: {
          available: true,
          version: "1.0.41"
        },
        devices: [],
        apk: {
          available: false
        }
      })
    },
    clipboard: {
      writeText: async () => {}
    }
  })

  const store = createAppStore()
  await store.actions.refreshInstallerState()

  assert.match(store.state.notifications[0]?.detail || "", /ADB 设备状态已刷新/)
})

test("准备清单会显示安装资源下载进度和速度", () => {
  const localStorage = createLocalStorage()
  installBrowserGlobals({
    localStorage,
    appMethods: {},
    clipboard: {
      writeText: async () => {}
    }
  })

  const store = createAppStore()
  store.state.installerState = {
    ...store.state.installerState,
    adb: {
      available: false,
      message: "正在下载安装工具"
    },
    runtime: {
      platform: "windows-amd64",
      resources: {
        platformTools: {
          kind: "platformTools",
          phase: "downloading",
          ready: false,
          downloadedBytes: 1048576,
          totalBytes: 4194304,
          percent: 25,
          bytesPerSecond: 524288,
          message: "正在下载安装工具"
        }
      }
    }
  }

  const checks = store.selectors.wizardAutoChecks(store.live.value)
  const tool = checks.find((item) => item.id === "tool")

  assert.equal(tool.tag, "下载中")
  assert.equal(tool.progress.percent, 25)
  assert.match(tool.progress.label, /512 KB\/s/)
  assert.match(tool.progress.label, /1\.0 MB \/ 4\.0 MB/)
})

test("手动全局健康检查完成后会给出提示", async () => {
  const localStorage = createLocalStorage()
  installBrowserGlobals({
    localStorage,
    appMethods: {
      GetSnapshot: async () => ({
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
          running: true,
          message: "OpenWatcher 本机服务已启动。",
          configuredListen: "127.0.0.1:8787",
          configuredPublicBaseUrl: "http://127.0.0.1:8787",
          lastHealth: {
            ok: true,
            message: "OK",
            config: {
              listen: "127.0.0.1:8787",
              publicBaseUrl: "http://127.0.0.1:8787"
            },
            build: {
              version: "dev"
            }
          }
        },
        tunnel: {
          configured: false,
          running: false,
          publicBaseUrl: ""
        },
        networkContext: {
          interfaces: [],
          recommendedIp: "192.168.31.12"
        }
      }),
      GetInstallerStatus: async () => ({
        adb: {
          available: true,
          version: "1.0.41"
        },
        devices: [],
        apk: {
          available: true
        }
      }),
      EnsureRuntimeDependencies: async () => ({
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
          running: true,
          message: "OpenWatcher 本机服务已启动。",
          configuredListen: "127.0.0.1:8787",
          configuredPublicBaseUrl: "http://127.0.0.1:8787",
          lastHealth: {
            ok: true,
            message: "OK",
            config: {
              listen: "127.0.0.1:8787",
              publicBaseUrl: "http://127.0.0.1:8787"
            },
            build: {
              version: "dev"
            }
          }
        },
        tunnel: {
          configured: false,
          running: false,
          publicBaseUrl: ""
        },
        networkContext: {
          interfaces: [],
          recommendedIp: "192.168.31.12"
        }
      }),
      GetDeveloperEnvironmentSnapshot: async () => ({
        repositories: [],
        status: {
          running: false,
          message: "开发环境未启动"
        },
        tunnel: {
          configured: false,
          running: false,
          publicBaseUrl: ""
        },
        logs: []
      })
    },
    clipboard: {
      writeText: async () => {}
    }
  })

  const store = createAppStore()
  await store.actions.runGlobalHealthCheck({ manual: true })

  assert.match(store.state.notifications[0]?.detail || "", /已完成健康检查/)
})

test("checkForUpdatesAction 会调用后端更新检查并保存结果", async () => {
  const localStorage = createLocalStorage()
  const calls = []
  installBrowserGlobals({
    localStorage,
    appMethods: {
      CheckForUpdates: async (currentWatchVersion) => {
        calls.push(["CheckForUpdates", currentWatchVersion])
        return {
          channel: "beta",
          checkedAt: "2026-06-13T00:00:00Z",
          currentDesktopVersion: "dev",
          latestDesktopVersion: "dev-next",
          desktopUpdateAvailable: true,
          currentWatchVersion,
          latestWatchVersion: "dev-watch-next",
          watchUpdateAvailable: true,
          releaseSummary: "修复桌面更新检查与交互反馈"
        }
      }
    },
    clipboard: {
      writeText: async () => {}
    }
  })

  const store = createAppStore()
  store.state.installerState = {
    apk: {
      installedVersionName: "dev-watch"
    }
  }

  await store.actions.checkForUpdatesAction()

  assert.deepEqual(calls, [["CheckForUpdates", "dev-watch"]])
  assert.equal(store.state.updateCheckResult.latestDesktopVersion, "dev-next")
  assert.equal(store.state.updateCheckResult.latestWatchVersion, "dev-watch-next")
  assert.match(store.state.notifications[0]?.detail || "", /发现可用更新/)
})

test("installDesktopUpdateAction 会调用后端安装并保存状态", async () => {
  const calls = []
  installBrowserGlobals({
    localStorage: createLocalStorage(),
    appMethods: {
      InstallDesktopUpdate: async () => {
        calls.push(["InstallDesktopUpdate"])
        return {
          phase: "restarting",
          message: "更新程序已启动，Desktop 将自动重启",
          version: "dev-next",
          artifact: "desktop_v0.1.0_macos_arm64.zip"
        }
      }
    },
    clipboard: {
      writeText: async () => {}
    }
  })

  const store = createAppStore()
  store.state.updateCheckResult = {
    desktopUpdateAvailable: true,
    desktopInstallable: true,
    latestDesktopVersion: "dev-next",
    desktopArtifact: "desktop_v0.1.0_macos_arm64.zip",
    desktopSizeBytes: 1024
  }

  await store.actions.installDesktopUpdateAction()

  assert.deepEqual(calls, [["InstallDesktopUpdate"]])
  assert.equal(store.state.desktopUpdateStatus.phase, "restarting")
  assert.equal(store.state.desktopUpdateStatus.version, "dev-next")
  assert.match(store.state.notifications[0]?.detail || "", /自动重启/)
})
