package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"openwatcher/desktop-app/internal/adb"
	"openwatcher/desktop-app/internal/backend"
	"openwatcher/desktop-app/internal/codex"
	"openwatcher/desktop-app/internal/devenv"
	"openwatcher/desktop-app/internal/diagnostics"
	"openwatcher/desktop-app/internal/installer"
	"openwatcher/desktop-app/internal/logging"
	"openwatcher/desktop-app/internal/network"
	desktopruntime "openwatcher/desktop-app/internal/runtime"
	"openwatcher/desktop-app/internal/settings"
	"openwatcher/desktop-app/internal/tunnel"
	rootconfig "openwatcher/internal/config"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var desktopProductVersion = "dev"

type App struct {
	ctx               context.Context
	processCtx        context.Context
	processCancel     context.CancelFunc
	codexDetector     *codex.Detector
	backendManager    *backend.Manager
	devBackendManager *backend.Manager
	devEnvManager     *devenv.Manager
	diagnosticMaker   *diagnostics.Builder
	installerManager  *installer.Manager
	runtimeManager    *desktopruntime.Manager
	tunnelManager     *tunnel.Manager
	devTunnelManager  *tunnel.Manager
}

type Snapshot struct {
	ProductVersion string                `json:"productVersion"`
	System         SystemSnapshot        `json:"system"`
	Codex          codex.Status          `json:"codex"`
	Backend        backend.DesktopStatus `json:"backend"`
	Tunnel         tunnel.Status         `json:"tunnel"`
	AccessMode     network.Mode          `json:"accessMode"`
	NetworkContext network.Context       `json:"networkContext"`
	Actions        []ActionCard          `json:"actions"`
	GeneratedAt    string                `json:"generatedAt"`
}

type SystemSnapshot struct {
	Platform         string `json:"platform"`
	Architecture     string `json:"architecture"`
	GoVersion        string `json:"goVersion"`
	DesktopConfigDir string `json:"desktopConfigDir"`
}

type ActionCard struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Primary     bool   `json:"primary"`
}

type DiagnosticsPayload struct {
	Snapshot             Snapshot                     `json:"snapshot"`
	BackendLogs          []backend.LogLine            `json:"backendLogs"`
	DeveloperEnvironment DeveloperEnvironmentSnapshot `json:"developerEnvironment"`
	Installer            installer.Status             `json:"installer"`
	GeneratedAt          string                       `json:"generatedAt"`
}

type DesktopSettingsState struct {
	AutoStartBackend     bool                                  `json:"autoStartBackend"`
	DeveloperEnvironment settings.DeveloperEnvironmentSettings `json:"developerEnvironment"`
}

func NewApp() *App {
	redactor := logging.NewRedactor()
	runtimeManager := desktopruntime.NewManager(settings.AppRoot(), desktopVersion())
	manager := backend.NewManager(
		backend.NewBinaryLocator(settings.AppRoot()),
		redactor,
	)
	devManager := backend.NewManager(
		backend.NewBinaryLocator(settings.AppRoot()),
		redactor,
	)
	adbService := adb.NewService(adb.NewBinaryLocator(settings.AppRoot()), redactor)
	return &App{
		codexDetector:     codex.NewDetector(),
		backendManager:    manager,
		devBackendManager: devManager,
		devEnvManager:     devenv.NewManager(redactor),
		diagnosticMaker:   diagnostics.NewBuilder(redactor),
		installerManager:  installer.NewManager(adbService, installer.NewAPKLocator(settings.AppRoot()), runtimeManager, redactor),
		runtimeManager:    runtimeManager,
		tunnelManager:     tunnel.NewManager(settings.AppRoot(), runtimeManager, redactor),
		devTunnelManager:  tunnel.NewNamedManager(settings.AppRoot(), runtimeManager, redactor, "managed-dev-tunnel", "开发环境托管隧道"),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.processCtx, a.processCancel = context.WithCancel(ctx)
	desktopSettings, _ := settings.LoadDesktopSettings()
	if desktopSettings.AutoStartBackend {
		_ = a.backendManager.StartBackend(a.processContext(), a.backendStartConfig())
	}
	if desktopSettings.DeveloperEnvironment.Enabled {
		_ = a.ensureDeveloperEnvironmentFromSettings(desktopSettings.DeveloperEnvironment)
	}
	if a.runtimeManager != nil {
		go func() {
			backgroundCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			_ = a.runtimeManager.EnsureAll(backgroundCtx)
		}()
	}
}

func (a *App) shutdown(context.Context) {
	if a.processCancel != nil {
		a.processCancel()
	}
	_ = a.tunnelManager.Stop(context.Background())
	_ = a.backendManager.StopBackend(context.Background())
	if a.devBackendManager != nil {
		_ = a.devBackendManager.StopBackend(context.Background())
	}
	if a.devEnvManager != nil {
		_ = a.devEnvManager.Stop(context.Background())
	}
	if a.devTunnelManager != nil {
		_ = a.devTunnelManager.Stop(context.Background())
	}
}

func (a *App) GetSnapshot() Snapshot {
	configDir, err := settings.ConfigDirLabel()
	if err != nil {
		configDir = "未能解析"
	}
	backendStatus := a.backendManager.DesktopStatus()
	if backendStatus.Running {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		_, _ = a.backendManager.CheckHealth(ctx)
		cancel()
		backendStatus = a.backendManager.DesktopStatus()
	}

	return Snapshot{
		ProductVersion: desktopVersionLabel(),
		System: SystemSnapshot{
			Platform:         runtime.GOOS,
			Architecture:     runtime.GOARCH,
			GoVersion:        runtime.Version(),
			DesktopConfigDir: configDir,
		},
		Codex:          a.codexDetector.Inspect(),
		Backend:        backendStatus,
		Tunnel:         a.tunnelManager.Status(),
		AccessMode:     network.ModeUnconfigured,
		NetworkContext: network.DetectContext(),
		GeneratedAt:    time.Now().Format(time.RFC3339),
		Actions: []ActionCard{
			{
				ID:          "install-watch",
				Title:       "开始安装手表 App",
				Description: "进入安装向导，后续接入无线 ADB 配对、安装与 bootstrap。",
				Primary:     true,
			},
			{
				ID:          "start-backend",
				Title:       "启动本机服务",
				Description: "尝试定位 bundled sidecar 或开发模式二进制，并启动本机服务。",
			},
			{
				ID:          "access-mode",
				Title:       "配置访问方式",
				Description: "支持局域网模式、自定义公网 URL 与 OpenWatcher 托管隧道。",
			},
			{
				ID:          "logs",
				Title:       "打开日志",
				Description: "查看脱敏后的 Desktop 与 sidecar 日志摘要。",
			},
		},
	}
}

func desktopVersion() string {
	value := strings.TrimSpace(desktopProductVersion)
	if value == "" {
		return "dev"
	}
	return value
}

func desktopVersionLabel() string {
	value := desktopVersion()
	if strings.HasPrefix(value, "v") || strings.HasPrefix(value, "dev") {
		return value
	}
	return "v" + value
}

func (a *App) StartBackend() backend.DesktopStatus {
	_ = a.backendManager.StartBackend(a.processContext(), a.backendStartConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	_, _ = a.backendManager.CheckHealth(ctx)
	cancel()
	return a.backendManager.DesktopStatus()
}

func (a *App) StopBackend() backend.DesktopStatus {
	_ = a.tunnelManager.Stop(context.Background())
	_ = a.backendManager.StopBackend(context.Background())
	return a.backendManager.DesktopStatus()
}

func (a *App) RestartBackend() backend.DesktopStatus {
	_ = a.backendManager.RestartBackend(a.processContext(), a.backendStartConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	_, _ = a.backendManager.CheckHealth(ctx)
	cancel()
	return a.backendManager.DesktopStatus()
}

func (a *App) CheckHealth() backend.HealthStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	status, _ := a.backendManager.CheckHealth(ctx)
	return status
}

func (a *App) GetBackendLogs() []backend.LogLine {
	return a.backendManager.GetBackendLogs(120)
}

func (a *App) CopyDiagnostics() string {
	return a.diagnosticMaker.Build(a.buildDiagnosticsPayload())
}

func (a *App) GetDesktopSettings() DesktopSettingsState {
	loaded, _ := settings.LoadDesktopSettings()
	return DesktopSettingsState{
		AutoStartBackend:     loaded.AutoStartBackend,
		DeveloperEnvironment: loaded.DeveloperEnvironment,
	}
}

func (a *App) SetAutoStartBackend(enabled bool) (DesktopSettingsState, error) {
	loaded, err := settings.LoadDesktopSettings()
	if err != nil {
		loaded = settings.DefaultDesktopSettings()
	}
	loaded.AutoStartBackend = enabled
	if err := settings.SaveDesktopSettings(loaded); err != nil {
		return DesktopSettingsState{}, err
	}
	return DesktopSettingsState{
		AutoStartBackend:     loaded.AutoStartBackend,
		DeveloperEnvironment: loaded.DeveloperEnvironment,
	}, nil
}

func (a *App) CheckForUpdates(currentWatchVersion string) (desktopruntime.UpdateCheckResult, error) {
	if a.runtimeManager == nil {
		return desktopruntime.UpdateCheckResult{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.runtimeManager.CheckForUpdates(ctx, currentWatchVersion)
}

func (a *App) GetDesktopUpdateStatus() desktopruntime.DesktopUpdateStatus {
	if a.runtimeManager == nil {
		return desktopruntime.DesktopUpdateStatus{}
	}
	return a.runtimeManager.GetDesktopUpdateStatus()
}

func (a *App) InstallDesktopUpdate() (desktopruntime.DesktopUpdateInstallResult, error) {
	if a.runtimeManager == nil {
		return desktopruntime.DesktopUpdateInstallResult{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result, err := a.runtimeManager.PrepareDesktopUpdate(ctx, func(progress desktopruntime.DesktopUpdateProgress) {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "desktop-update-progress", progress)
		}
	})
	if err != nil {
		return desktopruntime.DesktopUpdateInstallResult{}, err
	}
	if a.ctx != nil {
		go func() {
			time.Sleep(600 * time.Millisecond)
			wailsruntime.Quit(a.ctx)
		}()
	}
	return result, nil
}

func (a *App) OpenDesktopConfigDir() string {
	path, err := settings.ConfigDir()
	if err != nil {
		return "无法解析 Desktop 配置目录"
	}
	return openPath(path)
}

func (a *App) OpenBackendConfigDir() string {
	configPath := settings.BackendConfigPath()
	if configPath == "" {
		return "未找到本机服务配置路径"
	}
	return openPath(filepath.Dir(configPath))
}

func (a *App) OpenCodexHome() string {
	codexHome, err := rootconfig.ResolveCodexHome("")
	if err != nil {
		return "无法解析 Codex 目录"
	}
	return openPath(codexHome)
}

func (a *App) GetCodexHookStatus() codex.HookStatus {
	binaryPath := ""
	if resolved, err := backend.NewBinaryLocator(settings.AppRoot()).Resolve(); err == nil {
		binaryPath = resolved.Path
	}
	status, err := codex.InspectOpenWatcherHooks(binaryPath)
	if err != nil {
		status.Message = friendlyHookStatusError(err)
	}
	if strings.TrimSpace(binaryPath) == "" && !status.Installed {
		status.Message = backend.NewBinaryLocator(settings.AppRoot()).FriendlyError()
	}
	return status
}

func (a *App) InstallCodexHooks() (codex.HookStatus, error) {
	locator := backend.NewBinaryLocator(settings.AppRoot())
	resolved, err := locator.Resolve()
	if err != nil {
		status, statusErr := codex.InspectOpenWatcherHooks("")
		if statusErr != nil {
			status.Message = friendlyHookStatusError(statusErr)
		}
		status.Message = locator.FriendlyError()
		return status, err
	}
	return codex.InstallOpenWatcherHooks(resolved.Path)
}

func (a *App) OpenCodexHooksFile() string {
	path, err := codex.HooksPath()
	if err != nil {
		return "无法解析 Codex hooks.json 路径"
	}
	return openPath(path)
}

func (a *App) ChooseDeveloperRepositoryDir(defaultPath string) (string, error) {
	options := wailsruntime.OpenDialogOptions{
		Title:                "选择开发仓库目录",
		CanCreateDirectories: true,
	}
	candidate := strings.TrimSpace(defaultPath)
	if candidate == "" {
		candidate = currentRepoRoot()
	}
	if candidate != "" {
		options.DefaultDirectory = candidate
	}
	return wailsruntime.OpenDirectoryDialog(a.ctx, options)
}

func (a *App) OpenDeveloperLogFile() string {
	if a.devEnvManager == nil {
		return "未找到开发环境日志"
	}
	return openPath(a.devEnvManager.LogPath())
}

func (a *App) ClearDeveloperEnvironmentLogs() DeveloperEnvironmentSnapshot {
	if a.devEnvManager != nil {
		a.devEnvManager.ClearLogs()
	}
	return a.developerEnvironmentSnapshot(a.devEnvManager.Status())
}

func (a *App) OpenDeveloperEnvFile(repoPath string) string {
	target := devenv.EnvFilePath(firstNonBlank(strings.TrimSpace(repoPath), currentRepoRoot()))
	if strings.TrimSpace(target) == "" {
		return "未找到 .env.development"
	}
	return openPath(target)
}

func (a *App) backendStartConfig() backend.StartConfig {
	return a.startConfigFromRequest(BackendRequest{
		Mode: string(network.ModeLAN),
	})
}

func (a *App) buildDiagnosticsPayload() DiagnosticsPayload {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return DiagnosticsPayload{
		Snapshot:             a.GetSnapshot(),
		BackendLogs:          a.backendManager.GetBackendLogs(200),
		DeveloperEnvironment: a.developerEnvironmentSnapshot(a.devEnvManager.Status()),
		Installer:            a.installerManager.Status(ctx),
		GeneratedAt:          time.Now().Format(time.RFC3339),
	}
}

func (a *App) processContext() context.Context {
	if a.processCtx != nil {
		return a.processCtx
	}
	return context.Background()
}

func openPath(path string) string {
	if path == "" {
		return "目标路径为空"
	}
	cmd := exec.Command("open", path)
	if err := cmd.Run(); err != nil {
		return "打开目录失败"
	}
	return ""
}

func friendlyHookStatusError(err error) string {
	if err == nil {
		return ""
	}
	return "读取 Codex hooks 状态失败：" + err.Error()
}
