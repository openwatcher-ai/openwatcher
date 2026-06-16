package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"openwatcher/desktop-app/internal/adb"
	"openwatcher/desktop-app/internal/logging"
	desktopruntime "openwatcher/desktop-app/internal/runtime"
	"openwatcher/desktop-app/internal/settings"
)

type Phase string

const (
	PhaseIdle         Phase = "idle"
	PhasePairing      Phase = "pairing"
	PhaseConnecting   Phase = "connecting"
	PhaseInstalling   Phase = "installing"
	PhaseLaunching    Phase = "launching"
	PhaseBootstrap    Phase = "bootstrap"
	PhaseVerifying    Phase = "verifying"
	PhaseDone         Phase = "done"
	PhaseTroubleshoot Phase = "troubleshooting"
)

type LogLine struct {
	At      string `json:"at"`
	Source  string `json:"source"`
	Message string `json:"message"`
}

type APKInfo struct {
	Path                 string `json:"path,omitempty"`
	Label                string `json:"label,omitempty"`
	VersionName          string `json:"versionName,omitempty"`
	VersionCode          int    `json:"versionCode,omitempty"`
	PackageName          string `json:"packageName,omitempty"`
	SHA256               string `json:"sha256,omitempty"`
	Debug                bool   `json:"debug"`
	Available            bool   `json:"available"`
	Message              string `json:"message,omitempty"`
	DevFallback          bool   `json:"devFallback"`
	Installed            bool   `json:"installed"`
	InstalledVersionName string `json:"installedVersionName,omitempty"`
	InstalledVersionCode int    `json:"installedVersionCode,omitempty"`
}

type Status struct {
	ADB            adb.Status   `json:"adb"`
	Devices        []adb.Device `json:"devices"`
	SelectedSerial string       `json:"selectedSerial,omitempty"`
	SelectedLabel  string       `json:"selectedLabel,omitempty"`
	SelectedPort   int          `json:"selectedPort,omitempty"`
	APK            APKInfo      `json:"apk"`
	Phase          Phase        `json:"phase"`
	Message        string       `json:"message,omitempty"`
	Logs           []LogLine    `json:"logs"`
}

type PairRequest struct {
	PairIP      string `json:"pairIP"`
	PairPort    string `json:"pairPort"`
	PairingCode string `json:"pairingCode"`
	ConnectIP   string `json:"connectIP"`
	ConnectPort string `json:"connectPort"`
}

type Manager struct {
	adb      *adb.Service
	locator  *APKLocator
	runtime  *desktopruntime.Manager
	redactor *logging.Redactor

	mu             sync.Mutex
	logs           []LogLine
	phase          Phase
	message        string
	selectedSerial string
}

func NewManager(adbService *adb.Service, locator *APKLocator, runtimeManager *desktopruntime.Manager, redactor *logging.Redactor) *Manager {
	return &Manager{
		adb:      adbService,
		locator:  locator,
		runtime:  runtimeManager,
		redactor: redactor,
		phase:    PhaseIdle,
	}
}

func (m *Manager) Status(ctx context.Context) Status {
	runtimeErr := m.ensureRuntime(ctx)
	adbStatus := m.adb.Status(ctx)
	devices := []adb.Device{}
	if adbStatus.Available {
		if listed, result, err := m.adb.Devices(ctx); err == nil {
			devices = listed
			if strings.TrimSpace(result.Stdout) != "" {
				m.appendLog("adb", "已刷新设备列表")
			}
		}
	}

	m.mu.Lock()
	selectedSerial := m.selectedSerial
	phase := m.phase
	message := m.message
	logs := append([]LogLine(nil), m.logs...)
	m.mu.Unlock()

	if resolvedSerial, _ := resolveSelectedSerial(devices, selectedSerial); resolvedSerial != "" && resolvedSerial != selectedSerial {
		selectedSerial = resolvedSerial
		m.mu.Lock()
		m.selectedSerial = selectedSerial
		m.mu.Unlock()
	}

	selectedLabel := adb.DeviceLabelForSerial(devices, selectedSerial)
	apkInfo := m.locator.Resolve()
	if runtimeErr != "" {
		if !adbStatus.Available {
			adbStatus.Message = runtimeErr
		}
		if !apkInfo.Available {
			apkInfo.Message = runtimeErr
		}
		if message == "" && (!adbStatus.Available || !apkInfo.Available) {
			message = runtimeErr
		}
	}
	if selectedSerial != "" && apkInfo.PackageName != "" {
		if packageInfo, _, err := m.adb.InspectPackage(ctx, selectedSerial, apkInfo.PackageName); err == nil {
			apkInfo.Installed = packageInfo.Installed
			apkInfo.InstalledVersionName = packageInfo.VersionName
			apkInfo.InstalledVersionCode = packageInfo.VersionCode
		}
	}
	return Status{
		ADB:            adbStatus,
		Devices:        devices,
		SelectedSerial: selectedSerial,
		SelectedLabel:  selectedLabel,
		SelectedPort:   adb.PortFromSerial(selectedSerial),
		APK:            apkInfo,
		Phase:          phase,
		Message:        message,
		Logs:           logs,
	}
}

func (m *Manager) ensureRuntime(ctx context.Context) string {
	if m.runtime == nil {
		return ""
	}
	if err := m.runtime.EnsureInstaller(ctx); err != nil {
		return err.Error()
	}
	return ""
}

func (m *Manager) SelectDevice(serial string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.selectedSerial = strings.TrimSpace(serial)
}

func (m *Manager) Pair(ctx context.Context, request PairRequest) Status {
	pairIP := strings.TrimSpace(request.PairIP)
	pairPort, err := parsePort(request.PairPort)
	if err != nil {
		m.setState(PhaseTroubleshoot, "配对端口非法")
		return m.Status(ctx)
	}
	if pairIP == "" || strings.TrimSpace(request.PairingCode) == "" {
		m.setState(PhaseTroubleshoot, "请填写配对 IP、配对端口和配对码")
		return m.Status(ctx)
	}

	m.setState(PhasePairing, "正在执行 adb pair")
	if result, err := m.adb.Pair(ctx, pairIP, pairPort, request.PairingCode); err != nil {
		m.appendCommand("adb", result)
		m.setState(PhaseTroubleshoot, err.Error())
		return m.Status(ctx)
	} else {
		m.appendCommand("adb", result)
	}

	m.setState(PhaseDone, "ADB 配对已完成")
	return m.Status(ctx)
}

func (m *Manager) Connect(ctx context.Context, request PairRequest) Status {
	connectIP := strings.TrimSpace(request.ConnectIP)
	connectPort, err := parsePort(request.ConnectPort)
	if err != nil {
		m.setState(PhaseTroubleshoot, "连接端口非法")
		return m.Status(ctx)
	}
	if connectIP == "" {
		m.setState(PhaseTroubleshoot, "请填写连接 IP 和连接端口")
		return m.Status(ctx)
	}

	m.setState(PhaseConnecting, "正在执行 adb connect")
	if result, err := m.adb.Connect(ctx, connectIP, connectPort); err != nil {
		m.appendCommand("adb", result)
		m.setState(PhaseTroubleshoot, err.Error())
		return m.Status(ctx)
	} else {
		m.appendCommand("adb", result)
	}

	status := m.Status(ctx)
	if len(status.Devices) == 0 {
		m.setState(PhaseTroubleshoot, "连接成功后仍未检测到设备")
		return m.Status(ctx)
	}
	selectedSerial, needsSelection := resolveSelectedSerial(status.Devices, status.SelectedSerial)
	if selectedSerial != "" && selectedSerial != status.SelectedSerial {
		m.SelectDevice(selectedSerial)
		status = m.Status(ctx)
	}
	if needsSelection {
		m.setState(PhaseDone, "检测到多个 ADB 设备，请先选择目标手表")
		return m.Status(ctx)
	}
	m.setState(PhaseDone, "ADB 连接已完成")
	return m.Status(ctx)
}

func (m *Manager) PairAndConnect(ctx context.Context, request PairRequest) Status {
	current := m.Status(ctx)
	if len(current.Devices) == 1 && current.SelectedSerial != "" {
		m.setState(PhaseDone, "检测到已连接设备，跳过无线配对。")
		return m.Status(ctx)
	}

	paired := m.Pair(ctx, request)
	if paired.Phase == PhaseTroubleshoot {
		return paired
	}
	if strings.TrimSpace(request.ConnectIP) == "" {
		request.ConnectIP = strings.TrimSpace(request.PairIP)
	}
	if strings.TrimSpace(request.ConnectPort) == "" {
		request.ConnectPort = strings.TrimSpace(request.PairPort)
	}
	return m.Connect(ctx, request)
}

func (m *Manager) VerifySelected(ctx context.Context) Status {
	status := m.Status(ctx)
	if status.SelectedSerial == "" {
		m.setState(PhaseTroubleshoot, "未检测到目标设备")
		return m.Status(ctx)
	}
	if status.APK.PackageName == "" {
		m.setState(PhaseTroubleshoot, "未找到可校验的手表包名")
		return m.Status(ctx)
	}

	m.setState(PhaseVerifying, "正在校验手表状态")
	packageInfo, result, err := m.adb.InspectPackage(ctx, status.SelectedSerial, status.APK.PackageName)
	m.appendCommand("adb", result)
	if err != nil {
		m.setState(PhaseTroubleshoot, err.Error())
		return m.Status(ctx)
	}
	if !packageInfo.Installed {
		m.setState(PhaseTroubleshoot, "手表 App 尚未安装到当前设备")
		return m.Status(ctx)
	}
	m.appendLog("watch", fmt.Sprintf("已确认 %s 安装在 %s", status.APK.PackageName, status.SelectedSerial))
	m.setState(PhaseDone, "已校验手表状态，请在手表确认 bootstrap 确认页")
	return m.Status(ctx)
}

func (m *Manager) InstallSelected(ctx context.Context) Status {
	status := m.Status(ctx)
	if status.SelectedSerial == "" {
		m.setState(PhaseTroubleshoot, "未检测到可安装的设备，请先配对或选择设备")
		return m.Status(ctx)
	}
	if !status.APK.Available {
		m.setState(PhaseTroubleshoot, status.APK.Message)
		return m.Status(ctx)
	}
	if blocked := installPolicyMessage(status); blocked != "" {
		m.setState(PhaseTroubleshoot, blocked)
		return m.Status(ctx)
	}

	m.setState(PhaseInstalling, "正在安装手表 APK")
	result, err := m.adb.Install(contextWithTimeout(ctx, 90*time.Second), status.SelectedSerial, status.APK.Path)
	m.appendCommand("adb", result)
	if err != nil {
		m.setState(PhaseTroubleshoot, err.Error())
		return m.Status(ctx)
	}

	message := "手表 APK 安装完成"
	if status.APK.DevFallback {
		message = "已安装 debug APK，仅用于本地验证"
	}
	m.setState(PhaseDone, message)
	return m.Status(ctx)
}

func (m *Manager) LaunchSelected(ctx context.Context) Status {
	status := m.Status(ctx)
	if status.SelectedSerial == "" {
		m.setState(PhaseTroubleshoot, "未检测到目标设备")
		return m.Status(ctx)
	}
	if status.APK.PackageName == "" {
		m.setState(PhaseTroubleshoot, "未找到可启动的手表包名")
		return m.Status(ctx)
	}

	m.setState(PhaseLaunching, "正在启动手表 App")
	result, err := m.adb.StartApp(contextWithTimeout(ctx, 20*time.Second), status.SelectedSerial, status.APK.PackageName)
	m.appendCommand("adb", result)
	if err != nil {
		m.setState(PhaseTroubleshoot, err.Error())
		return m.Status(ctx)
	}
	m.setState(PhaseDone, "手表 App 已启动")
	return m.Status(ctx)
}

func (m *Manager) StartBootstrap(ctx context.Context, deepLink string) Status {
	status := m.Status(ctx)
	if status.SelectedSerial == "" {
		m.setState(PhaseTroubleshoot, "未检测到目标设备")
		return m.Status(ctx)
	}

	m.setState(PhaseBootstrap, "正在发送 bootstrap deep link")
	result, err := m.adb.StartDeepLink(contextWithTimeout(ctx, 20*time.Second), status.SelectedSerial, deepLink)
	m.appendCommand("adb", result)
	if err != nil {
		m.setState(PhaseTroubleshoot, err.Error())
		return m.Status(ctx)
	}
	m.setState(PhaseDone, "已发送配置链接，请在手表上确认")
	return m.Status(ctx)
}

func (m *Manager) appendCommand(source string, result adb.CommandResult) {
	commandLine := strings.TrimSpace(result.Command)
	if commandLine != "" {
		m.appendLog(source, commandLine)
	}
	for _, block := range []string{result.Stdout, result.Stderr} {
		for _, line := range strings.Split(block, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			m.appendLog(source, trimmed)
		}
	}
}

func (m *Manager) appendLog(source string, message string) {
	trimmed := strings.TrimSpace(m.redactor.RedactLine(message))
	if trimmed == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, LogLine{
		At:      time.Now().Format(time.RFC3339),
		Source:  source,
		Message: trimmed,
	})
	if len(m.logs) > 200 {
		m.logs = append([]LogLine(nil), m.logs[len(m.logs)-200:]...)
	}
}

func (m *Manager) setState(phase Phase, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.phase = phase
	m.message = message
}

func contextWithTimeout(parent context.Context, timeout time.Duration) context.Context {
	if _, ok := parent.Deadline(); ok {
		return parent
	}
	ctx, _ := context.WithTimeout(parent, timeout)
	return ctx
}

func parsePort(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, errors.New("empty port")
	}
	value := 0
	for _, r := range trimmed {
		if r < '0' || r > '9' {
			return 0, errors.New("invalid port")
		}
		value = value*10 + int(r-'0')
	}
	if value < 1 || value > 65535 {
		return 0, errors.New("port out of range")
	}
	return value, nil
}

func resolveSelectedSerial(devices []adb.Device, current string) (string, bool) {
	trimmed := strings.TrimSpace(current)
	if trimmed != "" {
		for _, device := range devices {
			if device.Serial == trimmed {
				return trimmed, false
			}
		}
	}
	if len(devices) == 1 {
		return devices[0].Serial, false
	}
	return "", len(devices) > 1
}

func installPolicyMessage(status Status) string {
	if !status.APK.Debug {
		return ""
	}
	for _, device := range status.Devices {
		if device.Serial != status.SelectedSerial {
			continue
		}
		if device.IsEmulator {
			return ""
		}
		return "当前仅检测到 debug 手表 APK。公开发布和真实设备安装只允许 release 包；如需继续，请先提供 openwatcher-watch-release.apk。"
	}
	return ""
}

type APKLocator struct {
	appRoot string
}

func NewAPKLocator(appRoot string) *APKLocator {
	return &APKLocator{appRoot: appRoot}
}

type apkCandidate struct {
	label       string
	path        string
	debug       bool
	devFallback bool
	useManifest bool
}

func (l *APKLocator) Resolve() APKInfo {
	for _, candidate := range l.candidates() {
		info, err := os.Stat(candidate.path)
		if err != nil || info.IsDir() {
			continue
		}
		sha, _ := fileSHA256(candidate.path)
		versionName, versionCode := versionFromAPKOutputMetadata(candidate.path)
		if candidate.useManifest {
			if manifestVersionName, manifestVersionCode := cachedRuntimeWatchVersion(); manifestVersionName != "" || manifestVersionCode > 0 {
				versionName = manifestVersionName
				versionCode = manifestVersionCode
			}
		}
		packageName := "ai.openwatcher.watchapp"
		if candidate.debug {
			packageName = "ai.openwatcher.watchapp.debug"
			if versionName != "" {
				versionName += "-debug"
			}
		}
		return APKInfo{
			Path:        candidate.path,
			Label:       candidate.label,
			VersionName: versionName,
			VersionCode: versionCode,
			PackageName: packageName,
			SHA256:      sha,
			Debug:       candidate.debug,
			Available:   true,
			DevFallback: candidate.devFallback,
			Message:     "",
		}
	}
	return APKInfo{
		Available: false,
		Message:   "未找到可安装的手表 APK。请检查网络后稍等片刻让 Desktop 自动下载，或先在本地构建 watch-app，或在 bundled/watch-apk 中提供 release APK。",
	}
}

func (l *APKLocator) candidates() []apkCandidate {
	root := l.repoRoot()
	candidates := make([]apkCandidate, 0, 8)
	if runtimeRoot, err := settings.RuntimeDir(); err == nil {
		candidates = append(candidates, apkCandidate{
			label:       "cached watch release APK",
			path:        filepath.Join(runtimeRoot, "watch-apk", "openwatcher-watch-release.apk"),
			useManifest: true,
		})
	}
	for _, bundledRoot := range settings.BundledResourceRoots(l.appRoot) {
		candidates = append(candidates,
			apkCandidate{
				label: "bundled watch release APK",
				path:  filepath.Join(bundledRoot, "watch-apk", "openwatcher-watch-release.apk"),
			},
			apkCandidate{
				label:       "bundled watch debug APK",
				path:        filepath.Join(bundledRoot, "watch-apk", "app-debug.apk"),
				debug:       true,
				devFallback: true,
			},
		)
	}
	candidates = append(candidates,
		apkCandidate{
			label:       "watch-app local beta output",
			path:        filepath.Join(root, "watch-app", "app", "build", "outputs", "apk", "localBeta", "app-localBeta.apk"),
			devFallback: true,
		},
		apkCandidate{
			label: "watch-app release output",
			path:  filepath.Join(root, "watch-app", "app", "build", "outputs", "apk", "release", "app-release.apk"),
		},
		apkCandidate{
			label:       "watch-app debug output",
			path:        filepath.Join(root, "watch-app", "app", "build", "outputs", "apk", "debug", "app-debug.apk"),
			debug:       true,
			devFallback: true,
		},
	)
	return candidates
}

func cachedRuntimeWatchVersion() (string, int) {
	manifest, err := desktopruntime.LoadCachedManifest()
	if err != nil {
		return "", 0
	}
	return manifest.Resources.WatchAPK.VersionName, manifest.Resources.WatchAPK.VersionCode
}

func (l *APKLocator) repoRoot() string {
	return filepath.Clean(filepath.Join(l.appRoot, ".."))
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func versionFromAPKOutputMetadata(apkPath string) (string, int) {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(apkPath), "output-metadata.json"))
	if err != nil {
		return "", 0
	}
	var metadata struct {
		Elements []struct {
			OutputFile  string `json:"outputFile"`
			VersionName string `json:"versionName"`
			VersionCode int    `json:"versionCode"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return "", 0
	}
	apkName := filepath.Base(apkPath)
	for _, element := range metadata.Elements {
		if element.OutputFile == "" || element.OutputFile == apkName {
			return strings.TrimSpace(element.VersionName), element.VersionCode
		}
	}
	return "", 0
}
