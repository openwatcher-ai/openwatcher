package adb

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"openwatcher/desktop-app/internal/logging"
	"openwatcher/testsupport/fakeadb"
)

const (
	testSerial  = "192.168.1.33:40221"
	testPackage = "ai.openwatcher.watchapp"
)

func TestMain(m *testing.M) {
	if fakeadb.MaybeRunProcess() {
		return
	}
	os.Exit(m.Run())
}

func TestParseDevices(t *testing.T) {
	raw := `List of devices attached
emulator-5554 device product:sdk_gwear_arm64 model:sdk_gwear_arm64 device:emu64a transport_id:2
192.168.1.33:40221 device product:watch model:Xiaomi_Watch device:watch transport_id:3
`

	devices := parseDevices(raw)
	if len(devices) != 2 {
		t.Fatalf("device count = %d", len(devices))
	}
	if !devices[0].IsEmulator || !devices[0].IsWatch {
		t.Fatalf("expected first device to be emulator watch: %+v", devices[0])
	}
	if devices[0].HostAlias != "10.0.2.2" {
		t.Fatalf("expected emulator host alias 10.0.2.2, got %+v", devices[0])
	}
	if devices[1].DisplayName != "Xiaomi Watch" {
		t.Fatalf("displayName = %q", devices[1].DisplayName)
	}
}

func TestPortFromSerial(t *testing.T) {
	if got := PortFromSerial("192.168.1.12:40221"); got != 40221 {
		t.Fatalf("PortFromSerial = %d", got)
	}
}

func TestServiceDevicesMatrix(t *testing.T) {
	tests := []struct {
		name           string
		state          fakeadb.State
		wantCount      int
		wantStates     []string
		stdoutContains []string
	}{
		{
			name:           "empty devices",
			state:          fakeadb.State{},
			wantCount:      0,
			stdoutContains: []string{"List of devices attached"},
		},
		{
			name: "single watch",
			state: fakeadb.State{
				Connected:   true,
				Serial:      testSerial,
				Product:     "watch",
				Model:       "Pixel Watch",
				Device:      "watch",
				TransportID: "7",
			},
			wantCount:      1,
			wantStates:     []string{"device"},
			stdoutContains: []string{testSerial, "device", "product:watch", "model:Pixel_Watch", "transport_id:7"},
		},
		{
			name: "multiple devices",
			state: fakeadb.State{Devices: []fakeadb.DeviceEntry{
				{Serial: "emulator-5554", State: "device", Product: "sdk_gwear_arm64", Model: "sdk_gwear_arm64", Device: "emu64a", TransportID: "2"},
				{Serial: testSerial, State: "device", Product: "watch", Model: "Xiaomi Watch", Device: "watch", TransportID: "3"},
			}},
			wantCount:      2,
			wantStates:     []string{"device", "device"},
			stdoutContains: []string{"emulator-5554", testSerial, "model:Xiaomi_Watch"},
		},
		{
			name: "offline device",
			state: fakeadb.State{Devices: []fakeadb.DeviceEntry{
				{Serial: testSerial, State: "offline", Product: "watch", Model: "Offline Watch", Device: "watch", TransportID: "4"},
			}},
			wantCount:      1,
			wantStates:     []string{"offline"},
			stdoutContains: []string{testSerial, "offline", "model:Offline_Watch"},
		},
		{
			name: "unauthorized device",
			state: fakeadb.State{Devices: []fakeadb.DeviceEntry{
				{Serial: testSerial, State: "unauthorized", Product: "watch", Model: "Locked Watch", Device: "watch", TransportID: "5"},
			}},
			wantCount:      1,
			wantStates:     []string{"unauthorized"},
			stdoutContains: []string{testSerial, "unauthorized", "model:Locked_Watch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _ := newFakeService(t, tt.state)
			devices, result, err := service.Devices(context.Background())
			if err != nil {
				t.Fatalf("Devices err = %v, result=%+v", err, result)
			}
			assertCommandResult(t, result, 0, tt.stdoutContains, nil)
			assertEmpty(t, "stderr", result.Stderr)
			if len(devices) != tt.wantCount {
				t.Fatalf("device count = %d, want %d: %+v", len(devices), tt.wantCount, devices)
			}
			for i, wantState := range tt.wantStates {
				if devices[i].State != wantState {
					t.Fatalf("device[%d].State = %q, want %q: %+v", i, devices[i].State, wantState, devices[i])
				}
			}
		})
	}
}

func TestServicePairMatrix(t *testing.T) {
	tests := []struct {
		name           string
		state          fakeadb.State
		wantExitCode   int
		stdoutContains []string
		stderrContains []string
		wantErr        string
	}{
		{
			name:           "success",
			wantExitCode:   0,
			stdoutContains: []string{"Successfully paired to 192.168.1.33:37123"},
		},
		{
			name:           "authentication failure",
			state:          fakeadb.State{PairFailure: "auth"},
			wantExitCode:   1,
			stderrContains: []string{"failed to authenticate to 192.168.1.33:37123"},
			wantErr:        "无线调试认证失败，请重新确认配对码",
		},
		{
			name:           "bad pairing code",
			state:          fakeadb.State{PairFailure: "bad_code"},
			wantExitCode:   1,
			stderrContains: []string{"Failed: Wrong pairing code"},
			wantErr:        "Failed: Wrong pairing code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, adbPath := newFakeService(t, tt.state)
			result, err := service.Pair(context.Background(), "192.168.1.33", 37123, "123456")
			assertErrorMessage(t, err, tt.wantErr)
			assertCommandResult(t, result, tt.wantExitCode, tt.stdoutContains, tt.stderrContains)
			if tt.wantErr == "" {
				assertEmpty(t, "stderr", result.Stderr)
			} else {
				assertEmpty(t, "stdout", result.Stdout)
			}
			records := fakeadb.ReadCommands(t, fakeadb.CommandsPath(adbPath))
			if len(records) != 1 {
				t.Fatalf("command count = %d, want 1: %+v", len(records), records)
			}
			if !records[0].StdinProvided {
				t.Fatalf("pairing code should be passed on stdin: %+v", records[0])
			}
			if strings.Contains(result.Command, "123456") {
				t.Fatalf("pairing code leaked into command: %q", result.Command)
			}
		})
	}
}

func TestServiceConnectMatrix(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		service, _ := newFakeService(t, fakeadb.State{})
		result, err := service.Connect(context.Background(), "192.168.1.33", 40221)
		assertErrorMessage(t, err, "")
		assertCommandResult(t, result, 0, []string{"connected to " + testSerial}, nil)
		assertEmpty(t, "stderr", result.Stderr)

		devices, devicesResult, err := service.Devices(context.Background())
		if err != nil {
			t.Fatalf("Devices after connect err = %v, result=%+v", err, devicesResult)
		}
		assertCommandResult(t, devicesResult, 0, []string{testSerial, "device"}, nil)
		if len(devices) != 1 || devices[0].Serial != testSerial {
			t.Fatalf("devices after connect = %+v", devices)
		}
	})

	t.Run("connection failure", func(t *testing.T) {
		service, _ := newFakeService(t, fakeadb.State{ConnectFailure: "unable_to_connect"})
		result, err := service.Connect(context.Background(), "192.168.1.33", 40221)
		assertErrorMessage(t, err, "连接设备失败，请检查 IP、端口和同一 Wi‑Fi")
		assertCommandResult(t, result, 1, nil, []string{"unable to connect to " + testSerial})
		assertEmpty(t, "stdout", result.Stdout)
	})

	t.Run("success but device does not appear", func(t *testing.T) {
		service, _ := newFakeService(t, fakeadb.State{ConnectWithoutDevice: true})
		result, err := service.Connect(context.Background(), "192.168.1.33", 40221)
		assertErrorMessage(t, err, "")
		assertCommandResult(t, result, 0, []string{"connected to " + testSerial}, nil)
		assertEmpty(t, "stderr", result.Stderr)

		devices, devicesResult, err := service.Devices(context.Background())
		if err != nil {
			t.Fatalf("Devices after no-device connect err = %v, result=%+v", err, devicesResult)
		}
		assertCommandResult(t, devicesResult, 0, []string{"List of devices attached"}, nil)
		assertEmpty(t, "stderr", devicesResult.Stderr)
		if len(devices) != 0 {
			t.Fatalf("devices after no-device connect = %+v, want empty", devices)
		}
	})
}

func TestServiceInstallMatrix(t *testing.T) {
	tests := []struct {
		name           string
		state          fakeadb.State
		wantExitCode   int
		stdoutContains []string
		stderrContains []string
		wantErr        string
		wantInstalled  bool
	}{
		{
			name:           "success",
			state:          fakeadb.State{Connected: true},
			wantExitCode:   0,
			stdoutContains: []string{"Success"},
			wantInstalled:  true,
		},
		{
			name:           "version downgrade",
			state:          fakeadb.State{Connected: true, InstallFailure: "version_downgrade"},
			wantExitCode:   1,
			stderrContains: []string{"INSTALL_FAILED_VERSION_DOWNGRADE"},
			wantErr:        "安装失败：设备上已有更高版本",
		},
		{
			name:           "update incompatible",
			state:          fakeadb.State{Connected: true, InstallFailure: "update_incompatible"},
			wantExitCode:   1,
			stderrContains: []string{"INSTALL_FAILED_UPDATE_INCOMPATIBLE"},
			wantErr:        "安装失败：设备上已存在签名不一致的包",
		},
		{
			name:           "generic failure",
			state:          fakeadb.State{Connected: true, InstallFailure: "generic"},
			wantExitCode:   1,
			stderrContains: []string{"INSTALL_FAILED_INTERNAL_ERROR"},
			wantErr:        "adb: failed to install apk: Failure [INSTALL_FAILED_INTERNAL_ERROR]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, adbPath := newFakeService(t, tt.state)
			apkPath := filepath.Join(t.TempDir(), "openwatcher-watch.apk")
			result, err := service.Install(context.Background(), testSerial, apkPath)
			assertErrorMessage(t, err, tt.wantErr)
			assertCommandResult(t, result, tt.wantExitCode, tt.stdoutContains, tt.stderrContains)
			if tt.wantErr == "" {
				assertEmpty(t, "stderr", result.Stderr)
			} else {
				assertEmpty(t, "stdout", result.Stdout)
			}
			state := fakeadb.ReadState(t, fakeadb.StatePath(adbPath))
			if state.Installed != tt.wantInstalled {
				t.Fatalf("Installed = %v, want %v: %+v", state.Installed, tt.wantInstalled, state)
			}
			if tt.wantInstalled && state.APKPath != apkPath {
				t.Fatalf("APKPath = %q, want %q", state.APKPath, apkPath)
			}
		})
	}
}

func TestServiceStartAppMatrix(t *testing.T) {
	tests := []struct {
		name           string
		state          fakeadb.State
		wantExitCode   int
		stdoutContains []string
		stderrContains []string
		wantErr        string
	}{
		{
			name:           "success",
			state:          fakeadb.State{Connected: true, Installed: true},
			wantExitCode:   0,
			stdoutContains: []string{"Events injected: 1"},
		},
		{
			name:           "package not found",
			state:          fakeadb.State{Connected: true, MonkeyFailure: "package_not_found"},
			wantExitCode:   1,
			stderrContains: []string{"monkey: unknown package: " + testPackage},
			wantErr:        "monkey: unknown package: " + testPackage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _ := newFakeService(t, tt.state)
			result, err := service.StartApp(context.Background(), testSerial, testPackage)
			assertErrorMessage(t, err, tt.wantErr)
			assertCommandResult(t, result, tt.wantExitCode, tt.stdoutContains, tt.stderrContains)
			if tt.wantErr == "" {
				assertEmpty(t, "stderr", result.Stderr)
			} else {
				assertEmpty(t, "stdout", result.Stdout)
			}
		})
	}
}

func TestServiceStartDeepLinkMatrix(t *testing.T) {
	deepLink := "openwatcher://bootstrap?deviceName=Test%20Watch&source=desktop-bootstrap"

	t.Run("records deep link success", func(t *testing.T) {
		service, adbPath := newFakeService(t, fakeadb.State{Connected: true})
		result, err := service.StartDeepLink(context.Background(), testSerial, deepLink)
		assertErrorMessage(t, err, "")
		assertCommandResult(t, result, 0, []string{"Starting: Intent", "Status: ok", deepLink}, nil)
		assertEmpty(t, "stderr", result.Stderr)

		state := fakeadb.ReadState(t, fakeadb.StatePath(adbPath))
		if state.LastDeepLink != deepLink {
			t.Fatalf("LastDeepLink = %q, want %q", state.LastDeepLink, deepLink)
		}
		records := fakeadb.ReadCommands(t, fakeadb.CommandsPath(adbPath))
		if len(records) != 1 || records[0].Operation != "shell am start" || records[0].DeepLink != deepLink {
			t.Fatalf("deep link command record = %+v", records)
		}
	})

	t.Run("start failure", func(t *testing.T) {
		service, adbPath := newFakeService(t, fakeadb.State{Connected: true, DeepLinkFailure: "start_failed"})
		result, err := service.StartDeepLink(context.Background(), testSerial, deepLink)
		assertErrorMessage(t, err, "Error: Activity not started, unable to resolve Intent")
		assertCommandResult(t, result, 1, nil, []string{"Activity not started"})
		assertEmpty(t, "stdout", result.Stdout)

		state := fakeadb.ReadState(t, fakeadb.StatePath(adbPath))
		if state.LastDeepLink != "" {
			t.Fatalf("LastDeepLink = %q, want empty on failure", state.LastDeepLink)
		}
	})
}

func TestServiceInspectPackageMatrix(t *testing.T) {
	t.Run("installed with version", func(t *testing.T) {
		service, _ := newFakeService(t, fakeadb.State{
			Connected:   true,
			Installed:   true,
			VersionName: "1.2.3",
			VersionCode: 123,
		})
		pathResult, err := service.Shell(context.Background(), testSerial, "pm", "path", testPackage)
		assertErrorMessage(t, err, "")
		assertCommandResult(t, pathResult, 0, []string{"package:/data/app/" + testPackage + "/base.apk"}, nil)
		assertEmpty(t, "stderr", pathResult.Stderr)

		dumpResult, err := service.Shell(context.Background(), testSerial, "dumpsys", "package", testPackage)
		assertErrorMessage(t, err, "")
		assertCommandResult(t, dumpResult, 0, []string{"Package [" + testPackage + "]", "versionName=1.2.3", "versionCode=123"}, nil)
		assertEmpty(t, "stderr", dumpResult.Stderr)

		info, result, err := service.InspectPackage(context.Background(), testSerial, testPackage)
		assertErrorMessage(t, err, "")
		assertCommandResult(t, result, 0, []string{"versionName=1.2.3", "versionCode=123"}, nil)
		assertEmpty(t, "stderr", result.Stderr)
		if !info.Installed || info.VersionName != "1.2.3" || info.VersionCode != 123 || info.Path == "" {
			t.Fatalf("PackageInfo = %+v", info)
		}
	})

	t.Run("not installed", func(t *testing.T) {
		service, _ := newFakeService(t, fakeadb.State{Connected: true, Installed: false})
		info, result, err := service.InspectPackage(context.Background(), testSerial, testPackage)
		assertErrorMessage(t, err, "")
		assertCommandResult(t, result, 1, nil, []string{"package " + testPackage + " was not found"})
		assertEmpty(t, "stdout", result.Stdout)
		if info.Installed || info.Path != "" || info.VersionName != "" || info.VersionCode != 0 {
			t.Fatalf("PackageInfo = %+v, want not installed", info)
		}
	})

	t.Run("version read failure keeps installed path", func(t *testing.T) {
		service, _ := newFakeService(t, fakeadb.State{Connected: true, Installed: true, DumpsysFailure: "version"})
		info, result, err := service.InspectPackage(context.Background(), testSerial, testPackage)
		assertErrorMessage(t, err, "")
		assertCommandResult(t, result, 1, nil, []string{"Unable to read package version for " + testPackage})
		assertEmpty(t, "stdout", result.Stdout)
		if !info.Installed || info.Path == "" || info.VersionName != "" || info.VersionCode != 0 {
			t.Fatalf("PackageInfo = %+v, want installed without version", info)
		}
	})
}

func newFakeService(t *testing.T, state fakeadb.State) (*Service, string) {
	t.Helper()
	configRoot := t.TempDir()
	t.Setenv("HOME", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("ANDROID_HOME", filepath.Join(t.TempDir(), "missing-android-home"))
	t.Setenv("ANDROID_SDK_ROOT", filepath.Join(t.TempDir(), "missing-android-sdk-root"))

	appRoot := filepath.Join(t.TempDir(), "desktop-app")
	adbPath := filepath.Join(appRoot, "bundled", "platform-tools", runtime.GOOS+"-"+runtime.GOARCH, fakeadb.BinaryName())
	fakeadb.InstallBinary(t, adbPath)
	fakeadb.WriteState(t, fakeadb.StatePath(adbPath), state)

	service := NewService(NewBinaryLocator(appRoot), logging.NewRedactor())
	return service, adbPath
}

func assertCommandResult(t *testing.T, result CommandResult, wantExitCode int, stdoutContains []string, stderrContains []string) {
	t.Helper()
	if result.ExitCode != wantExitCode {
		t.Fatalf("ExitCode = %d, want %d: %+v", result.ExitCode, wantExitCode, result)
	}
	for _, want := range stdoutContains {
		if !strings.Contains(result.Stdout, want) {
			t.Fatalf("stdout missing %q\nstdout=%q\nresult=%+v", want, result.Stdout, result)
		}
	}
	for _, want := range stderrContains {
		if !strings.Contains(result.Stderr, want) {
			t.Fatalf("stderr missing %q\nstderr=%q\nresult=%+v", want, result.Stderr, result)
		}
	}
}

func assertErrorMessage(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("err = nil, want %q", want)
	}
	if err.Error() != want {
		t.Fatalf("err = %q, want %q", err.Error(), want)
	}
}

func assertEmpty(t *testing.T, field string, value string) {
	t.Helper()
	if strings.TrimSpace(value) != "" {
		t.Fatalf("%s = %q, want empty", field, value)
	}
}
