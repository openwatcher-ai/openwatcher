package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"openwatcher/desktop-app/internal/adb"
	"openwatcher/desktop-app/internal/logging"
	desktopruntime "openwatcher/desktop-app/internal/runtime"
	"openwatcher/testsupport/fakeadb"
	"openwatcher/testsupport/fakeruntime"
)

const (
	flowSerial  = "192.168.1.33:40221"
	flowPackage = "ai.openwatcher.watchapp"
)

func TestMain(m *testing.M) {
	if fakeadb.MaybeRunProcess() {
		return
	}
	os.Exit(m.Run())
}

func TestManagerPairAndConnectFlow(t *testing.T) {
	t.Run("success selects connected watch", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{
			Serial:      flowSerial,
			Product:     "watch",
			Model:       "Flow Watch",
			Device:      "watch",
			TransportID: "11",
		})

		status := h.manager.PairAndConnect(context.Background(), flowPairRequest())

		assertStatus(t, status, PhaseDone, "ADB 连接已完成")
		assertSelected(t, status, flowSerial, "Flow Watch", 40221)
		if len(status.Devices) != 1 || status.Devices[0].State != "device" {
			t.Fatalf("devices = %+v", status.Devices)
		}
		assertLogContains(t, status.Logs, "Successfully paired")
		assertLogContains(t, status.Logs, "connected to "+flowSerial)
		assertOperationsContainOrdered(t, h.commands(t), []string{
			"pair 192.168.1.33:37123",
			"connect " + flowSerial,
		})
	})

	t.Run("already connected skips pair and connect", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{Connected: true, Model: "Ready Watch"})

		status := h.manager.PairAndConnect(context.Background(), flowPairRequest())

		assertStatus(t, status, PhaseDone, "检测到已连接设备，跳过无线配对。")
		assertSelected(t, status, flowSerial, "Ready Watch", 40221)
		assertNoOperation(t, h.commands(t), "pair ")
		assertNoOperation(t, h.commands(t), "connect ")
	})

	t.Run("multiple devices need manual selection", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{Devices: []fakeadb.DeviceEntry{
			{Serial: "emulator-5554", State: "device", Product: "sdk_gwear_arm64", Model: "sdk_gwear_arm64", Device: "emu64a", TransportID: "2"},
			{Serial: flowSerial, State: "device", Product: "watch", Model: "Flow Watch", Device: "watch", TransportID: "3"},
		}})

		status := h.manager.PairAndConnect(context.Background(), flowPairRequest())

		assertStatus(t, status, PhaseDone, "检测到多个 ADB 设备，请先选择目标手表")
		if status.SelectedSerial != "" {
			t.Fatalf("SelectedSerial = %q, want empty until user selects", status.SelectedSerial)
		}
		if len(status.Devices) != 2 {
			t.Fatalf("device count = %d, want 2: %+v", len(status.Devices), status.Devices)
		}
		assertOperationsContainOrdered(t, h.commands(t), []string{
			"pair 192.168.1.33:37123",
			"connect " + flowSerial,
		})
	})

	t.Run("pair authentication failure stops before connect", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{PairFailure: "auth"})

		status := h.manager.PairAndConnect(context.Background(), flowPairRequest())

		assertStatus(t, status, PhaseTroubleshoot, "无线调试认证失败，请重新确认配对码")
		assertLogContains(t, status.Logs, "failed to authenticate")
		assertOperationsContainOrdered(t, h.commands(t), []string{"pair 192.168.1.33:37123"})
		assertNoOperation(t, h.commands(t), "connect ")
	})

	t.Run("connect failure returns troubleshooting", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{ConnectFailure: "unable_to_connect"})

		status := h.manager.PairAndConnect(context.Background(), flowPairRequest())

		assertStatus(t, status, PhaseTroubleshoot, "连接设备失败，请检查 IP、端口和同一 Wi‑Fi")
		assertOperationsContainOrdered(t, h.commands(t), []string{
			"pair 192.168.1.33:37123",
			"connect " + flowSerial,
		})
	})

	t.Run("connect success without device returns troubleshooting", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{ConnectWithoutDevice: true})

		status := h.manager.PairAndConnect(context.Background(), flowPairRequest())

		assertStatus(t, status, PhaseTroubleshoot, "连接成功后仍未检测到设备")
		if len(status.Devices) != 0 {
			t.Fatalf("devices = %+v, want empty", status.Devices)
		}
		assertLogContains(t, status.Logs, "connected to "+flowSerial)
	})
}

func TestManagerInstallSelectedFlow(t *testing.T) {
	t.Run("success installs runtime release apk", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{Connected: true, Model: "Install Watch"})
		h.manager.SelectDevice(flowSerial)

		status := h.manager.InstallSelected(context.Background())

		assertStatus(t, status, PhaseDone, "手表 APK 安装完成")
		assertSelected(t, status, flowSerial, "Install Watch", 40221)
		if !status.APK.Available || status.APK.Debug || status.APK.PackageName != flowPackage {
			t.Fatalf("APK = %+v", status.APK)
		}
		if !strings.HasSuffix(status.APK.Path, filepath.Join("watch-apk", "openwatcher-watch-release.apk")) {
			t.Fatalf("APK path = %q", status.APK.Path)
		}
		if !status.APK.Installed {
			t.Fatalf("APK installed flag = false: %+v", status.APK)
		}
		state := fakeadb.ReadState(t, fakeadb.StatePath(h.runtime.ADBBinaryPath))
		if !state.Installed || state.APKPath != h.runtime.WatchAPKPath {
			t.Fatalf("fake adb install state = %+v, want APK %q", state, h.runtime.WatchAPKPath)
		}
		assertLogContains(t, status.Logs, "Success")
		assertOperationsContainOrdered(t, h.commands(t), []string{"install -r " + h.runtime.WatchAPKPath})
	})

	t.Run("no selected device", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{})

		status := h.manager.InstallSelected(context.Background())

		assertStatus(t, status, PhaseTroubleshoot, "未检测到可安装的设备，请先配对或选择设备")
		assertNoOperation(t, h.commands(t), "install ")
	})

	t.Run("version downgrade", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{Connected: true, InstallFailure: "version_downgrade"})
		h.manager.SelectDevice(flowSerial)

		status := h.manager.InstallSelected(context.Background())

		assertStatus(t, status, PhaseTroubleshoot, "安装失败：设备上已有更高版本")
		assertLogContains(t, status.Logs, "INSTALL_FAILED_VERSION_DOWNGRADE")
		assertOperationsContainOrdered(t, h.commands(t), []string{"install -r " + h.runtime.WatchAPKPath})
	})
}

func TestManagerLaunchSelectedFlow(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{Connected: true, Installed: true, Model: "Launch Watch"})
		h.manager.SelectDevice(flowSerial)

		status := h.manager.LaunchSelected(context.Background())

		assertStatus(t, status, PhaseDone, "手表 App 已启动")
		assertSelected(t, status, flowSerial, "Launch Watch", 40221)
		assertLogContains(t, status.Logs, "Events injected: 1")
		assertOperationsContainOrdered(t, h.commands(t), []string{"shell monkey"})
	})

	t.Run("no selected device", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{})

		status := h.manager.LaunchSelected(context.Background())

		assertStatus(t, status, PhaseTroubleshoot, "未检测到目标设备")
		assertNoOperation(t, h.commands(t), "shell monkey")
	})

	t.Run("adb launch failure", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{Connected: true, Installed: true, MonkeyFailure: "package_not_found"})
		h.manager.SelectDevice(flowSerial)

		status := h.manager.LaunchSelected(context.Background())

		assertStatus(t, status, PhaseTroubleshoot, "monkey: unknown package: "+flowPackage)
		assertLogContains(t, status.Logs, "monkey: unknown package")
		assertOperationsContainOrdered(t, h.commands(t), []string{"shell monkey"})
	})
}

func TestManagerStartBootstrapFlow(t *testing.T) {
	const rawToken = "flow-secret-token-012345"
	deepLink := "openwatcher://bootstrap?deviceName=Flow%20Watch&deviceToken=" + rawToken + "&source=desktop-bootstrap"

	t.Run("success records deep link and redacts token in logs", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{Connected: true})
		h.manager.SelectDevice(flowSerial)

		status := h.manager.StartBootstrap(context.Background(), deepLink)

		assertStatus(t, status, PhaseDone, "已发送配置链接，请在手表上确认")
		state := fakeadb.ReadState(t, fakeadb.StatePath(h.runtime.ADBBinaryPath))
		if state.LastDeepLink != deepLink {
			t.Fatalf("LastDeepLink = %q, want %q", state.LastDeepLink, deepLink)
		}
		assertLogContains(t, status.Logs, "Status: ok")
		assertLogNotContains(t, status.Logs, rawToken)
		assertOperationsContainOrdered(t, h.commands(t), []string{"shell am start"})
	})

	t.Run("adb start failure", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{Connected: true, DeepLinkFailure: "start_failed"})
		h.manager.SelectDevice(flowSerial)

		status := h.manager.StartBootstrap(context.Background(), deepLink)

		assertStatus(t, status, PhaseTroubleshoot, "Error: Activity not started, unable to resolve Intent")
		state := fakeadb.ReadState(t, fakeadb.StatePath(h.runtime.ADBBinaryPath))
		if state.LastDeepLink != "" {
			t.Fatalf("LastDeepLink = %q, want empty after failure", state.LastDeepLink)
		}
		assertLogContains(t, status.Logs, "Activity not started")
		assertLogNotContains(t, status.Logs, rawToken)
	})
}

func TestManagerVerifySelectedFlow(t *testing.T) {
	t.Run("success with installed package", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{
			Connected:   true,
			Installed:   true,
			VersionName: "1.2.3",
			VersionCode: 123,
		})
		h.manager.SelectDevice(flowSerial)

		status := h.manager.VerifySelected(context.Background())

		assertStatus(t, status, PhaseDone, "已校验手表状态，请在手表确认 bootstrap 确认页")
		if !status.APK.Installed || status.APK.InstalledVersionName != "1.2.3" || status.APK.InstalledVersionCode != 123 {
			t.Fatalf("APK installed info = %+v", status.APK)
		}
		assertLogContains(t, status.Logs, "已确认 "+flowPackage+" 安装在 "+flowSerial)
		assertOperationsContainOrdered(t, h.commands(t), []string{
			"shell pm path",
			"shell dumpsys package",
		})
	})

	t.Run("no selected device", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{})

		status := h.manager.VerifySelected(context.Background())

		assertStatus(t, status, PhaseTroubleshoot, "未检测到目标设备")
	})

	t.Run("package not installed", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{Connected: true, Installed: false})
		h.manager.SelectDevice(flowSerial)

		status := h.manager.VerifySelected(context.Background())

		assertStatus(t, status, PhaseTroubleshoot, "手表 App 尚未安装到当前设备")
		assertOperationsContainOrdered(t, h.commands(t), []string{"shell pm path"})
	})

	t.Run("version read failure still verifies installed package", func(t *testing.T) {
		h := newFlowHarness(t, fakeadb.State{Connected: true, Installed: true, DumpsysFailure: "version"})
		h.manager.SelectDevice(flowSerial)

		status := h.manager.VerifySelected(context.Background())

		assertStatus(t, status, PhaseDone, "已校验手表状态，请在手表确认 bootstrap 确认页")
		if !status.APK.Installed || status.APK.InstalledVersionName != "" || status.APK.InstalledVersionCode != 0 {
			t.Fatalf("APK installed info = %+v, want installed without version", status.APK)
		}
		assertLogContains(t, status.Logs, "Unable to read package version")
	})
}

type flowHarness struct {
	configRoot string
	appRoot    string
	runtime    *fakeruntime.Runtime
	manager    *Manager
}

func newFlowHarness(t *testing.T, adbState fakeadb.State) *flowHarness {
	t.Helper()
	configRoot := t.TempDir()
	t.Setenv("HOME", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("OPENWATCHER_CONFIG", filepath.Join(configRoot, "openwatcher", "config.json"))

	appRoot := filepath.Join(t.TempDir(), "desktop-app")
	if err := os.MkdirAll(appRoot, 0o755); err != nil {
		t.Fatalf("mkdir app root: %v", err)
	}

	fakeRuntime := fakeruntime.Start(t, appRoot)
	runtimeManager := desktopruntime.NewManager(appRoot, "0.2.0")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := runtimeManager.EnsureInstaller(ctx); err != nil {
		t.Fatalf("EnsureInstaller err = %v", err)
	}
	fakeadb.WriteState(t, fakeadb.StatePath(fakeRuntime.ADBBinaryPath), adbState)

	redactor := logging.NewRedactor()
	adbService := adb.NewService(adb.NewBinaryLocator(appRoot), redactor)
	manager := NewManager(adbService, NewAPKLocator(appRoot), runtimeManager, redactor)

	return &flowHarness{
		configRoot: configRoot,
		appRoot:    appRoot,
		runtime:    fakeRuntime,
		manager:    manager,
	}
}

func flowPairRequest() PairRequest {
	return PairRequest{
		PairIP:      "192.168.1.33",
		PairPort:    "37123",
		PairingCode: "123456",
		ConnectIP:   "192.168.1.33",
		ConnectPort: "40221",
	}
}

func (h *flowHarness) commands(t testing.TB) []fakeadb.CommandRecord {
	t.Helper()
	return fakeadb.ReadCommands(t, fakeadb.CommandsPath(h.runtime.ADBBinaryPath))
}

func assertStatus(t *testing.T, status Status, wantPhase Phase, wantMessage string) {
	t.Helper()
	if status.Phase != wantPhase {
		t.Fatalf("Phase = %q, want %q: %+v", status.Phase, wantPhase, status)
	}
	if status.Message != wantMessage {
		t.Fatalf("Message = %q, want %q: %+v", status.Message, wantMessage, status)
	}
}

func assertSelected(t *testing.T, status Status, wantSerial string, wantLabel string, wantPort int) {
	t.Helper()
	if status.SelectedSerial != wantSerial || status.SelectedLabel != wantLabel || status.SelectedPort != wantPort {
		t.Fatalf("selection = serial %q label %q port %d, want %q %q %d: %+v", status.SelectedSerial, status.SelectedLabel, status.SelectedPort, wantSerial, wantLabel, wantPort, status)
	}
}

func assertLogContains(t *testing.T, logs []LogLine, want string) {
	t.Helper()
	for _, line := range logs {
		if strings.Contains(line.Message, want) {
			return
		}
	}
	t.Fatalf("logs do not contain %q: %+v", want, logs)
}

func assertLogNotContains(t *testing.T, logs []LogLine, unwanted string) {
	t.Helper()
	for _, line := range logs {
		if strings.Contains(line.Message, unwanted) {
			t.Fatalf("logs contain %q: %+v", unwanted, logs)
		}
	}
}

func assertOperationsContainOrdered(t *testing.T, records []fakeadb.CommandRecord, want []string) {
	t.Helper()
	operations := operations(records)
	cursor := 0
	for _, operation := range operations {
		if cursor >= len(want) {
			break
		}
		if operation == want[cursor] {
			cursor++
		}
	}
	if cursor != len(want) {
		t.Fatalf("operations = %#v, missing ordered entries %#v", operations, want[cursor:])
	}
}

func assertNoOperation(t *testing.T, records []fakeadb.CommandRecord, prefix string) {
	t.Helper()
	for _, operation := range operations(records) {
		if strings.HasPrefix(operation, prefix) {
			t.Fatalf("operation %q should not be present in %#v", prefix, operations(records))
		}
	}
}

func operations(records []fakeadb.CommandRecord) []string {
	result := make([]string, 0, len(records))
	for _, record := range records {
		if record.Operation == "" {
			continue
		}
		result = append(result, record.Operation)
	}
	return result
}
