package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"openwatcher/desktop-app/internal/backend"
	"openwatcher/desktop-app/internal/devenv"
	"openwatcher/desktop-app/internal/installer"
	"openwatcher/desktop-app/internal/network"
	"openwatcher/desktop-app/internal/settings"
	"openwatcher/desktop-app/internal/tunnel"
	rootconfig "openwatcher/internal/config"
	"openwatcher/internal/pairing"
)

const defaultSidecarPort = "8787"
const defaultDevSidecarPort = "18787"

type BackendRequest struct {
	Mode          string                     `json:"mode"`
	SelectedIP    string                     `json:"selectedIP"`
	BindAll       bool                       `json:"bindAll"`
	Port          string                     `json:"port"`
	CustomURL     string                     `json:"customURL"`
	TunnelCode    string                     `json:"tunnelCode"`
	DeviceName    string                     `json:"deviceName"`
	PublicBaseURL string                     `json:"publicBaseURL"`
	Endpoints     []BootstrapEndpointRequest `json:"endpoints,omitempty"`
}

type BootstrapEndpointRequest struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	URL      string `json:"url"`
	Priority int    `json:"priority"`
}

type BootstrapPayload struct {
	APIBase          string `json:"apiBase"`
	PairURL          string `json:"pairUrl"`
	BootstrapURI     string `json:"bootstrapUri"`
	ADBCommand       string `json:"adbCommand"`
	DeviceName       string `json:"deviceName"`
	TokenFingerprint string `json:"tokenFingerprint"`
	CreatedAt        string `json:"createdAt"`
}

type DevBootstrapRequest struct {
	BaseURL              string `json:"baseURL"`
	DeviceName           string `json:"deviceName"`
	Mode                 string `json:"mode,omitempty"`
	RepoPath             string `json:"repoPath,omitempty"`
	HostAlias            string `json:"hostAlias,omitempty"`
	ManagedTunnelEnabled bool   `json:"managedTunnelEnabled,omitempty"`
}

type RemoteWatchBootstrapRequest struct {
	BootstrapCode string `json:"bootstrapCode"`
	Environment   string `json:"environment"`
	APIBase       string `json:"apiBase"`
	TunnelCode    string `json:"tunnelCode"`
}

type RemoteWatchBootstrapResult struct {
	BootstrapCode  string               `json:"bootstrapCode"`
	Environment    string               `json:"environment"`
	APIBase        string               `json:"apiBase"`
	TunnelRedeemed bool                 `json:"tunnelRedeemed"`
	Health         backend.HealthStatus `json:"health"`
	SubmittedAt    string               `json:"submittedAt"`
	Message        string               `json:"message"`
}

type InstallerRequest struct {
	PairIP         string `json:"pairIP"`
	PairPort       string `json:"pairPort"`
	PairingCode    string `json:"pairingCode"`
	ConnectIP      string `json:"connectIP"`
	ConnectPort    string `json:"connectPort"`
	SelectedSerial string `json:"selectedSerial"`
}

type BootstrapApplyResult struct {
	Installer installer.Status `json:"installer"`
	Payload   BootstrapPayload `json:"payload"`
}

type DeveloperEnvironmentRequest struct {
	Enabled              bool   `json:"enabled"`
	Mode                 string `json:"mode"`
	RepoPath             string `json:"repoPath"`
	BaseURL              string `json:"baseURL"`
	DeviceName           string `json:"deviceName"`
	HostAlias            string `json:"hostAlias"`
	ManagedTunnelEnabled bool   `json:"managedTunnelEnabled"`
}

type DeveloperEnvironmentSnapshot struct {
	Repositories []devenv.Repository `json:"repositories"`
	Status       devenv.Status       `json:"status"`
	Tunnel       tunnel.Status       `json:"tunnel"`
	Logs         []devenv.LogLine    `json:"logs"`
}

func (a *App) StartBackendWithRequest(request BackendRequest) backend.DesktopStatus {
	cfg := a.startConfigFromRequest(request)
	if err := a.validateRequest(request); err != nil {
		return invalidBackendStatus(request, cfg, err.Error())
	}
	mode := network.NormalizeMode(request.Mode)
	if mode != network.ModeManagedBeta {
		_ = a.tunnelManager.Stop(context.Background())
	}
	if a.backendNeedsRestart(cfg) {
		_ = a.backendManager.RestartBackend(a.processContext(), cfg)
	} else {
		_ = a.backendManager.StartBackend(a.processContext(), cfg)
	}
	health := a.postStartHealthCheck(request, cfg)
	status := a.backendManager.DesktopStatus()
	status.LastHealth = &health
	if !health.OK && mode != network.ModeManagedBeta {
		status.Message = health.Message
		status.FriendlyError = health.Message
		status.State = "error"
	}
	return status
}

func (a *App) RestartBackendWithRequest(request BackendRequest) backend.DesktopStatus {
	cfg := a.startConfigFromRequest(request)
	if err := a.validateRequest(request); err != nil {
		return invalidBackendStatus(request, cfg, err.Error())
	}
	_ = a.tunnelManager.Stop(context.Background())
	_ = a.backendManager.RestartBackend(a.processContext(), cfg)
	health := a.postStartHealthCheck(request, cfg)
	status := a.backendManager.DesktopStatus()
	status.LastHealth = &health
	if !health.OK && network.NormalizeMode(request.Mode) != network.ModeManagedBeta {
		status.Message = health.Message
		status.FriendlyError = health.Message
		status.State = "error"
	}
	return status
}

func (a *App) CheckHealthWithRequest(request BackendRequest) backend.HealthStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	return a.checkHealthWithRequest(ctx, request)
}

func (a *App) RedeemManagedTunnelCode(code string) (Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := a.tunnelManager.Redeem(ctx, code, desktopVersion()); err != nil {
		return Snapshot{}, err
	}
	return a.GetSnapshot(), nil
}

func (a *App) ExportDiagnosticsBundle() (string, error) {
	configDir, err := settings.ConfigDir()
	if err != nil {
		return "", err
	}
	outputDir := filepath.Join(configDir, "diagnostics")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("openwatcher-desktop-diagnostics-%s.txt", time.Now().UTC().Format("20060102-150405"))
	outputPath := filepath.Join(outputDir, filename)
	payload := a.diagnosticMaker.Build(a.buildDiagnosticsPayload())
	if err := os.WriteFile(outputPath, []byte(payload), 0o600); err != nil {
		return "", err
	}
	return outputPath, nil
}

func (a *App) ListDeveloperRepositories() []devenv.Repository {
	return devenv.DetectRepositories(currentRepoRoot())
}

func (a *App) GetDeveloperEnvironmentSnapshot(request DeveloperEnvironmentRequest) DeveloperEnvironmentSnapshot {
	if a.devEnvManager == nil {
		return a.developerEnvironmentSnapshot(devenv.Status{})
	}
	cfg := developerConfigFromRequest(request)
	cfg.ManagedTunnel = a.devTunnelManager != nil && a.devTunnelManager.Status().Configured && request.ManagedTunnelEnabled
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	status := a.devEnvManager.Observe(ctx, cfg)
	return a.developerEnvironmentSnapshot(status)
}

func (a *App) EnsureDeveloperEnvironment(request DeveloperEnvironmentRequest) DeveloperEnvironmentSnapshot {
	if a.devEnvManager == nil {
		return a.developerEnvironmentSnapshot(devenv.Status{})
	}
	cfg := developerConfigFromRequest(request)
	cfg.ManagedTunnel = a.devTunnelManager != nil && a.devTunnelManager.Status().Configured && request.ManagedTunnelEnabled
	status := a.devEnvManager.Ensure(a.processContext(), cfg)
	if cfg.Enabled && cfg.ManagedTunnel && strings.EqualFold(cfg.Mode, string(devenv.ModeWorkspace)) {
		originURL := rootconfig.DefaultPublicBaseURL(devSidecarLoopbackListen(cfg.BaseURL))
		_ = a.devTunnelManager.Start(a.processContext(), originURL)
	} else if a.devTunnelManager != nil {
		_ = a.devTunnelManager.Stop(context.Background())
	}
	_ = a.persistDeveloperEnvironmentPreference(request, cfg.Enabled)
	return a.developerEnvironmentSnapshot(status)
}

func (a *App) StopDeveloperEnvironment() DeveloperEnvironmentSnapshot {
	if a.devEnvManager != nil {
		_ = a.devEnvManager.Stop(context.Background())
	}
	if a.devBackendManager != nil {
		_ = a.devBackendManager.StopBackend(context.Background())
	}
	if a.devTunnelManager != nil {
		_ = a.devTunnelManager.Stop(context.Background())
	}
	status := devenv.Status{}
	if a.devEnvManager != nil {
		status = a.devEnvManager.Status()
	}
	_ = a.persistDeveloperEnvironmentPreference(developerRequestFromStatus(status, false), false)
	return a.developerEnvironmentSnapshot(status)
}

func (a *App) RedeemDeveloperTunnelCode(code string) (DeveloperEnvironmentSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := a.devTunnelManager.Redeem(ctx, code, desktopVersion()); err != nil {
		return a.developerEnvironmentSnapshot(a.devEnvManager.Status()), err
	}
	return a.developerEnvironmentSnapshot(a.devEnvManager.Status()), nil
}

func (a *App) SubmitRemoteWatchBootstrap(request RemoteWatchBootstrapRequest) (RemoteWatchBootstrapResult, error) {
	environment := normalizeRemoteWatchEnvironment(request.Environment)
	apiBase := strings.TrimSpace(request.APIBase)
	tunnelRedeemed := false
	if tunnelCode := strings.TrimSpace(request.TunnelCode); tunnelCode != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		var status tunnel.Status
		var err error
		if environment == "dev" {
			status, err = a.devTunnelManager.Redeem(ctx, tunnelCode, desktopVersion())
		} else {
			status, err = a.tunnelManager.Redeem(ctx, tunnelCode, desktopVersion())
		}
		cancel()
		if err != nil {
			return RemoteWatchBootstrapResult{}, err
		}
		apiBase = status.PublicBaseURL
		if strings.TrimSpace(apiBase) == "" {
			return RemoteWatchBootstrapResult{}, errors.New("隧道兑换成功但未返回 API 基址")
		}
		tunnelRedeemed = true
	}

	normalizedAPIBase, err := network.ValidatePublicURL(apiBase, false)
	if err != nil {
		return RemoteWatchBootstrapResult{}, err
	}

	healthCtx, healthCancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	health, _ := backend.ProbePublicHealth(healthCtx, normalizedAPIBase, "远程配置 API 基址")
	healthCancel()

	submitCtx, submitCancel := context.WithTimeout(context.Background(), 10*time.Second)
	response, err := tunnel.NewClient().SubmitWatchBootstrapConfig(submitCtx, tunnel.WatchBootstrapConfigRequest{
		BootstrapCode: request.BootstrapCode,
		Environment:   environment,
		APIBase:       normalizedAPIBase,
		Source:        "desktop-remote-bootstrap",
	})
	submitCancel()
	if err != nil {
		return RemoteWatchBootstrapResult{}, err
	}

	message := "已提交临时配置，等待手表获取。"
	if !health.OK {
		message = "已提交临时配置，但 API 基址未通过 /healthz 检查。"
	}
	return RemoteWatchBootstrapResult{
		BootstrapCode:  response.BootstrapCode,
		Environment:    response.Config.Environment,
		APIBase:        response.Config.APIBase,
		TunnelRedeemed: tunnelRedeemed,
		Health:         health,
		SubmittedAt:    time.Now().Format(time.RFC3339),
		Message:        message,
	}, nil
}

func (a *App) developerEnvironmentSnapshot(status devenv.Status) DeveloperEnvironmentSnapshot {
	tunnelStatus := tunnel.Status{}
	if a.devTunnelManager != nil {
		tunnelStatus = a.devTunnelManager.Status()
	}
	var logs []devenv.LogLine
	if a.devEnvManager != nil {
		logs = a.devEnvManager.GetLogs(120)
	}
	return DeveloperEnvironmentSnapshot{
		Repositories: a.ListDeveloperRepositories(),
		Status:       status,
		Tunnel:       tunnelStatus,
		Logs:         logs,
	}
}

func (a *App) EnsureRuntimeDependencies() Snapshot {
	if a.runtimeManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = a.runtimeManager.EnsureAll(ctx)
	}
	return a.GetSnapshot()
}

func (a *App) PrepareWatchBootstrap(request BackendRequest) (BootstrapPayload, error) {
	mode := network.NormalizeMode(request.Mode)
	if mode == network.ModePublicURL || mode == network.ModeManagedBeta {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		health := a.checkHealthWithRequest(ctx, request)
		cancel()
		if !health.OK {
			return BootstrapPayload{}, errors.New(health.Message)
		}
	}
	cfg := a.startConfigFromRequest(request)
	return buildBootstrapPayload(cfg, request.DeviceName, a.resolveBootstrapEndpoints(request, cfg))
}

func (a *App) PrepareDevWatchBootstrap(request DevBootstrapRequest) (BootstrapPayload, error) {
	developerRequest := developerRequestFromDevBootstrap(request)
	if developerRequest.Enabled {
		_ = a.EnsureDeveloperEnvironment(developerRequest)
	}
	cfg, err := a.devRuntimeStartConfig(developerRequest.BaseURL)
	if err != nil {
		return BootstrapPayload{}, err
	}
	return buildDevBootstrapPayload(DevBootstrapRequest{
		BaseURL:    cfg.PublicBaseURL,
		DeviceName: request.DeviceName,
	})
}

func (a *App) GetInstallerStatus() installer.Status {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return a.installerManager.Status(ctx)
}

func (a *App) SelectInstallerDevice(serial string) installer.Status {
	a.installerManager.SelectDevice(serial)
	return a.GetInstallerStatus()
}

func (a *App) RunADBPairing(request InstallerRequest) installer.Status {
	if strings.TrimSpace(request.SelectedSerial) != "" {
		a.installerManager.SelectDevice(request.SelectedSerial)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return a.installerManager.PairAndConnect(ctx, installer.PairRequest{
		PairIP:      request.PairIP,
		PairPort:    request.PairPort,
		PairingCode: request.PairingCode,
		ConnectIP:   request.ConnectIP,
		ConnectPort: request.ConnectPort,
	})
}

func (a *App) RunADBPair(request InstallerRequest) installer.Status {
	if strings.TrimSpace(request.SelectedSerial) != "" {
		a.installerManager.SelectDevice(request.SelectedSerial)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return a.installerManager.Pair(ctx, installer.PairRequest{
		PairIP:      request.PairIP,
		PairPort:    request.PairPort,
		PairingCode: request.PairingCode,
	})
}

func (a *App) RunADBConnect(request InstallerRequest) installer.Status {
	if strings.TrimSpace(request.SelectedSerial) != "" {
		a.installerManager.SelectDevice(request.SelectedSerial)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return a.installerManager.Connect(ctx, installer.PairRequest{
		ConnectIP:   request.ConnectIP,
		ConnectPort: request.ConnectPort,
	})
}

func (a *App) InstallWatchApp(selectedSerial string) installer.Status {
	if strings.TrimSpace(selectedSerial) != "" {
		a.installerManager.SelectDevice(selectedSerial)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return a.installerManager.InstallSelected(ctx)
}

func (a *App) LaunchWatchApp(selectedSerial string) installer.Status {
	if strings.TrimSpace(selectedSerial) != "" {
		a.installerManager.SelectDevice(selectedSerial)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return a.installerManager.LaunchSelected(ctx)
}

func (a *App) VerifyWatchStatus(selectedSerial string) installer.Status {
	if strings.TrimSpace(selectedSerial) != "" {
		a.installerManager.SelectDevice(selectedSerial)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return a.installerManager.VerifySelected(ctx)
}

func (a *App) BootstrapWatchOnDevice(request BackendRequest, selectedSerial string) (BootstrapApplyResult, error) {
	if strings.TrimSpace(selectedSerial) != "" {
		a.installerManager.SelectDevice(selectedSerial)
	}
	cfg := a.startConfigFromRequest(request)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	healthStatus := a.checkHealthWithRequest(ctx, request)
	if !healthStatus.OK {
		message := "本机服务未就绪，请先启动本机服务并通过健康检查"
		if strings.TrimSpace(healthStatus.Message) != "" {
			message = healthStatus.Message
		}
		return BootstrapApplyResult{Installer: a.GetInstallerStatus()}, errors.New(message)
	}

	payload, err := buildBootstrapPayload(cfg, request.DeviceName, a.resolveBootstrapEndpoints(request, cfg))
	if err != nil {
		return BootstrapApplyResult{Installer: a.GetInstallerStatus()}, err
	}
	if err := a.overwriteBackendPairing(
		a.backendManager,
		defaultSidecarPort,
		cfg,
		rootconfig.PairingSlotBeta,
		extractTokenFromBootstrap(payload.BootstrapURI),
		payload.DeviceName,
	); err != nil {
		return BootstrapApplyResult{Installer: a.GetInstallerStatus()}, err
	}

	ctx, cancel = context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	installerStatus := a.installerManager.StartBootstrap(ctx, payload.BootstrapURI)
	if installerStatus.Phase == installer.PhaseTroubleshoot {
		return BootstrapApplyResult{Installer: installerStatus}, errors.New(installerStatus.Message)
	}
	return BootstrapApplyResult{
		Installer: installerStatus,
		Payload:   payload,
	}, nil
}

func (a *App) BootstrapDevWatchOnDevice(request DevBootstrapRequest, selectedSerial string) (BootstrapApplyResult, error) {
	if strings.TrimSpace(selectedSerial) != "" {
		a.installerManager.SelectDevice(selectedSerial)
	}
	developerRequest := developerRequestFromDevBootstrap(request)
	devSnapshot := a.EnsureDeveloperEnvironment(developerRequest)
	cfg, err := a.devRuntimeStartConfig(developerRequest.BaseURL)
	if err != nil {
		return BootstrapApplyResult{Installer: a.GetInstallerStatus()}, err
	}
	payload, err := buildDevBootstrapPayload(DevBootstrapRequest{
		BaseURL:    cfg.PublicBaseURL,
		DeviceName: request.DeviceName,
	})
	if err != nil {
		return BootstrapApplyResult{Installer: a.GetInstallerStatus()}, err
	}
	if err := a.persistBackendPairing(
		cfg,
		rootconfig.PairingSlotDev,
		extractTokenFromBootstrap(payload.BootstrapURI),
		payload.DeviceName,
	); err != nil {
		return BootstrapApplyResult{Installer: a.GetInstallerStatus()}, err
	}
	if developerRequest.Enabled && strings.EqualFold(developerRequest.Mode, string(devenv.ModeWorkspace)) {
		_ = a.devEnvManager.Stop(context.Background())
		devSnapshot = a.EnsureDeveloperEnvironment(developerRequest)
	}
	if a.devTunnelManager.Status().Configured && developerRequest.ManagedTunnelEnabled {
		_ = a.devTunnelManager.Start(a.processContext(), rootconfig.DefaultPublicBaseURL(devSidecarLoopbackListen(developerRequest.BaseURL)))
	}
	if status := devSnapshot.Status; developerRequest.Enabled && status.LastHealth != nil && !status.LastHealth.OK {
		return BootstrapApplyResult{Installer: a.GetInstallerStatus()}, errors.New(firstNonBlank(status.Message, status.LastHealth.Message, "开发环境尚未就绪"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	installerStatus := a.installerManager.StartBootstrap(ctx, payload.BootstrapURI)
	if installerStatus.Phase == installer.PhaseTroubleshoot {
		return BootstrapApplyResult{Installer: installerStatus}, errors.New(installerStatus.Message)
	}
	return BootstrapApplyResult{
		Installer: installerStatus,
		Payload:   payload,
	}, nil
}

func (a *App) ClearBackendPairing() (backend.DesktopStatus, error) {
	cfg, resolvedPath, err := a.loadMutableBackendConfig()
	if err != nil {
		return a.backendManager.DesktopStatus(), err
	}
	clearPairingConfig(&cfg, rootconfig.PairingSlotBeta)
	ensureManagedAPKDistDir(&cfg)
	if err := rootconfig.Save(resolvedPath, cfg); err != nil {
		return a.backendManager.DesktopStatus(), fmt.Errorf("保存本机服务配置失败")
	}
	status := a.backendManager.DesktopStatus()
	if !status.Running {
		return status, nil
	}
	runtimeCfg := a.runtimeConfigForConfigMutation(a.backendManager, defaultSidecarPort, backend.StartConfig{
		ConfigPath:    resolvedPath,
		Listen:        cfg.Listen,
		PublicBaseURL: cfg.PublicBaseURL,
		PairingSlot:   string(rootconfig.PairingSlotBeta),
	})
	if err := a.backendManager.RestartBackend(a.processContext(), runtimeCfg); err != nil {
		return a.backendManager.DesktopStatus(), fmt.Errorf("重启本机服务失败")
	}
	health := a.waitForBackendHealth(runtimeCfg, 12*time.Second)
	status = a.backendManager.DesktopStatus()
	status.LastHealth = &health
	if !health.OK {
		return status, errors.New("清空 beta 配对后本机服务未就绪")
	}
	return status, nil
}

func (a *App) ClearDeveloperPairing() (DeveloperEnvironmentSnapshot, error) {
	cfg, resolvedPath, err := a.loadMutableBackendConfig()
	if err != nil {
		return a.developerEnvironmentSnapshot(a.devEnvManager.Status()), err
	}
	clearPairingConfig(&cfg, rootconfig.PairingSlotDev)
	ensureManagedAPKDistDir(&cfg)
	if err := rootconfig.Save(resolvedPath, cfg); err != nil {
		return a.developerEnvironmentSnapshot(a.devEnvManager.Status()), fmt.Errorf("保存本机服务配置失败")
	}

	status := a.devEnvManager.Status()
	request := developerRequestFromStatus(status, status.Running)
	if status.Running {
		_ = a.devEnvManager.Stop(context.Background())
		if a.devBackendManager != nil {
			devCfg := a.runtimeConfigForConfigMutation(a.devBackendManager, defaultDevSidecarPort, backend.StartConfig{
				ConfigPath:    resolvedPath,
				Listen:        net.JoinHostPort("127.0.0.1", defaultDevSidecarPort),
				PublicBaseURL: rewriteURLPort(cfg.EffectivePublicBaseURL(), defaultDevSidecarPort),
				PairingSlot:   string(rootconfig.PairingSlotDev),
			})
			_ = a.devBackendManager.RestartBackend(a.processContext(), devCfg)
		}
		snapshot := a.EnsureDeveloperEnvironment(request)
		return snapshot, nil
	}
	snapshot := a.GetDeveloperEnvironmentSnapshot(request)
	return snapshot, nil
}

func (a *App) loadMutableBackendConfig() (rootconfig.Config, string, error) {
	configPath := settings.BackendConfigPath()
	if strings.TrimSpace(configPath) == "" {
		return rootconfig.Config{}, "", errors.New("未找到本机服务配置路径")
	}
	cfg, resolvedPath, err := rootconfig.Load(configPath)
	if err != nil {
		return rootconfig.Config{}, "", fmt.Errorf("读取本机服务配置失败")
	}
	return cfg, resolvedPath, nil
}

func (a *App) checkHealthWithRequest(ctx context.Context, request BackendRequest) backend.HealthStatus {
	cfg := a.startConfigFromRequest(request)
	mode := network.NormalizeMode(request.Mode)
	if err := a.validateRequest(request); err != nil {
		return constrainedHealthStatus(request, cfg, backend.HealthStatus{
			OK:       false,
			Message:  err.Error(),
			Endpoint: healthEndpointForMode(request, cfg),
		})
	}
	if mode == network.ModeManagedBeta {
		localStatus, _ := a.backendManager.CheckHealthWithConfig(ctx, cfg)
		if !localStatus.OK {
			_ = a.backendManager.RestartBackend(a.processContext(), cfg)
			localStatus = a.waitForBackendHealth(cfg, 12*time.Second)
			if !localStatus.OK {
				localStatus.Message = "本地 OpenWatcher 本机服务未就绪，请先启动本机服务。"
				return localStatus
			}
		}
		if err := a.tunnelManager.Start(a.processContext(), rootconfig.DefaultPublicBaseURL(cfg.Listen)); err != nil {
			localStatus.Message = "启动托管隧道失败: " + err.Error()
			return localStatus
		}
		status, err := a.tunnelManager.PublicHealth(ctx)
		if err != nil {
			return status
		}
		return status
	}
	if mode == network.ModePublicURL {
		status, err := backend.ProbePublicHealth(ctx, cfg.PublicBaseURL, "自定义公网 URL")
		if err != nil {
			return constrainedHealthStatus(request, cfg, status)
		}
		return constrainedHealthStatus(request, cfg, status)
	}
	status, _ := a.backendManager.CheckHealthWithConfig(ctx, cfg)
	return constrainedHealthStatus(request, cfg, status)
}

func buildBootstrapPayload(cfg backend.StartConfig, requestedDeviceName string, requestedEndpoints []BootstrapEndpointRequest) (BootstrapPayload, error) {
	token, err := generateDeviceToken()
	if err != nil {
		return BootstrapPayload{}, fmt.Errorf("生成设备 token 失败")
	}
	deviceName := strings.TrimSpace(requestedDeviceName)
	if deviceName == "" {
		deviceName = "watch"
	}
	endpoints, err := normalizeBootstrapEndpoints(requestedEndpoints, cfg.PublicBaseURL)
	if err != nil {
		return BootstrapPayload{}, err
	}
	apiBase := strings.TrimRight(endpoints[0].URL, "/")
	bootstrapURI, err := buildBootstrapURI(endpoints, token, deviceName)
	if err != nil {
		return BootstrapPayload{}, err
	}
	return BootstrapPayload{
		APIBase:          apiBase,
		PairURL:          buildPairURL(apiBase, token, deviceName),
		BootstrapURI:     bootstrapURI,
		ADBCommand:       `adb shell am start -a android.intent.action.VIEW -d "` + bootstrapURI + `"`,
		DeviceName:       deviceName,
		TokenFingerprint: fingerprintToken(token),
		CreatedAt:        time.Now().Format(time.RFC3339),
	}, nil
}

func buildDevBootstrapPayload(request DevBootstrapRequest) (BootstrapPayload, error) {
	token, err := generateDeviceToken()
	if err != nil {
		return BootstrapPayload{}, fmt.Errorf("生成设备 token 失败")
	}
	deviceName := strings.TrimSpace(request.DeviceName)
	if deviceName == "" {
		deviceName = "watch"
	}
	baseURL, err := network.ValidatePublicURL(request.BaseURL, false)
	if err != nil {
		return BootstrapPayload{}, err
	}
	endpoints := []BootstrapEndpointRequest{
		{
			ID:       "dev-primary",
			Label:    "开发环境",
			URL:      strings.TrimRight(baseURL, "/"),
			Priority: 0,
		},
	}
	bootstrapURI, err := buildDevBootstrapURI(endpoints, token, deviceName)
	if err != nil {
		return BootstrapPayload{}, err
	}
	apiBase := strings.TrimRight(baseURL, "/")
	return BootstrapPayload{
		APIBase:          apiBase,
		PairURL:          buildPairURL(apiBase, token, deviceName),
		BootstrapURI:     bootstrapURI,
		ADBCommand:       `adb shell am start -a android.intent.action.VIEW -d "` + bootstrapURI + `"`,
		DeviceName:       deviceName,
		TokenFingerprint: fingerprintToken(token),
		CreatedAt:        time.Now().Format(time.RFC3339),
	}, nil
}

func (a *App) devRuntimeStartConfig(baseURL string) (backend.StartConfig, error) {
	normalizedBaseURL, err := network.ValidatePublicURL(baseURL, false)
	if err != nil {
		return backend.StartConfig{}, err
	}
	return a.resolveRuntimeStartConfigForManager(a.devBackendManager, defaultDevSidecarPort, backend.StartConfig{
		ConfigPath:    settings.BackendConfigPath(),
		Listen:        devSidecarLoopbackListen(normalizedBaseURL),
		PublicBaseURL: normalizedBaseURL,
		PairingSlot:   string(rootconfig.PairingSlotDev),
	}), nil
}

func (a *App) resolveBootstrapEndpoints(request BackendRequest, cfg backend.StartConfig) []BootstrapEndpointRequest {
	if len(request.Endpoints) == 0 {
		return nil
	}

	resolved := make([]BootstrapEndpointRequest, 0, len(request.Endpoints))
	tunnelURL := strings.TrimRight(strings.TrimSpace(a.tunnelManager.Status().PublicBaseURL), "/")
	for _, item := range request.Endpoints {
		next := item
		switch strings.TrimSpace(item.ID) {
		case "lan":
			next.URL = cfg.PublicBaseURL
		case "public":
			if strings.TrimSpace(request.CustomURL) != "" {
				next.URL = network.NormalizePublicURL(request.CustomURL)
			}
		case "managedTunnel":
			if tunnelURL != "" {
				next.URL = tunnelURL
			}
		}
		resolved = append(resolved, next)
	}
	return resolved
}

func extractTokenFromBootstrap(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Query().Get("deviceToken")
}

func (a *App) startConfigFromRequest(request BackendRequest) backend.StartConfig {
	ctx := network.DetectContext()
	mode := network.NormalizeMode(request.Mode)
	selectedIP := strings.TrimSpace(request.SelectedIP)
	if selectedIP == "" {
		selectedIP = ctx.RecommendedIP
	}
	port := network.NormalizePort(request.Port)

	var listen string
	var publicBaseURL string
	switch mode {
	case network.ModePublicURL:
		listen = network.BuildListen("127.0.0.1", port, false)
		publicBaseURL = network.NormalizePublicURL(request.CustomURL)
	case network.ModeManagedBeta:
		listen = network.BuildListen("127.0.0.1", defaultSidecarPort, false)
		publicBaseURL = a.tunnelManager.Status().PublicBaseURL
	default:
		listen = network.BuildListen(selectedIP, port, true)
		publicBaseURL = network.BuildLANBaseURL(selectedIP, port)
	}
	if strings.TrimSpace(request.PublicBaseURL) != "" {
		publicBaseURL = strings.TrimRight(strings.TrimSpace(request.PublicBaseURL), "/")
	}

	return a.resolveRuntimeStartConfig(backend.StartConfig{
		ConfigPath:    settings.BackendConfigPath(),
		Listen:        listen,
		PublicBaseURL: publicBaseURL,
		PairingSlot:   string(rootconfig.PairingSlotBeta),
	})
}

func (a *App) resolveRuntimeStartConfig(cfg backend.StartConfig) backend.StartConfig {
	return a.resolveRuntimeStartConfigForManager(a.backendManager, defaultSidecarPort, cfg)
}

func (a *App) resolveRuntimeStartConfigForManager(
	manager *backend.Manager,
	defaultLoopbackPort string,
	cfg backend.StartConfig,
) backend.StartConfig {
	cfg = normalizeDesktopStartConfig(cfg)
	if !isLoopbackListen(cfg.Listen) {
		return cfg
	}

	requestedPort, _ := portFromListen(cfg.Listen)
	reusedCurrent := false
	if manager != nil {
		if current := currentLoopbackListen(manager.DesktopStatus()); current != "" {
			if current == cfg.Listen || requestedPort == defaultLoopbackPort {
				cfg.Listen = current
				reusedCurrent = true
			}
		}
	}
	if !reusedCurrent {
		if resolved, err := backend.ResolveLoopbackListen(cfg.Listen); err == nil {
			cfg.Listen = resolved
		}
	}

	cfg.PublicBaseURL = rewriteConfiguredPublicBaseURL(cfg.PublicBaseURL, cfg.Listen)
	return cfg
}

func normalizeDesktopStartConfig(cfg backend.StartConfig) backend.StartConfig {
	if strings.TrimSpace(cfg.ConfigPath) == "" {
		cfg.ConfigPath = settings.BackendConfigPath()
	}
	if strings.TrimSpace(cfg.Listen) == "" {
		cfg.Listen = rootconfig.DefaultListen
	}
	cfg.PublicBaseURL = rootconfig.NormalizePublicBaseURL(cfg.PublicBaseURL)
	if cfg.PublicBaseURL == "" {
		cfg.PublicBaseURL = rootconfig.DefaultPublicBaseURL(cfg.Listen)
	}
	if strings.TrimSpace(cfg.PairingSlot) == "" {
		cfg.PairingSlot = string(rootconfig.PairingSlotBeta)
	}
	return cfg
}

func currentLoopbackListen(status backend.DesktopStatus) string {
	if !status.Running || !isLoopbackListen(status.ConfiguredListen) {
		return ""
	}
	return strings.TrimSpace(status.ConfiguredListen)
}

func isLoopbackListen(listen string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(listen))
	if err != nil {
		return false
	}
	trimmed := strings.Trim(strings.TrimSpace(host), "[]")
	if trimmed == "localhost" {
		return true
	}
	ip := net.ParseIP(trimmed)
	return ip != nil && ip.IsLoopback()
}

func rewriteConfiguredPublicBaseURL(rawURL, listen string) string {
	trimmed := rootconfig.NormalizePublicBaseURL(rawURL)
	if trimmed == "" || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	actualPort, ok := portFromListen(listen)
	if !ok {
		return trimmed
	}
	return rewriteURLPort(trimmed, actualPort)
}

func portFromListen(listen string) (string, bool) {
	_, port, err := net.SplitHostPort(strings.TrimSpace(listen))
	if err != nil || strings.TrimSpace(port) == "" {
		return "", false
	}
	return port, true
}

func rewriteURLPort(rawURL, port string) string {
	if strings.TrimSpace(rawURL) == "" || strings.TrimSpace(port) == "" {
		return strings.TrimRight(strings.TrimSpace(rawURL), "/")
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return strings.TrimRight(strings.TrimSpace(rawURL), "/")
	}
	host := parsed.Hostname()
	if host == "" {
		return strings.TrimRight(strings.TrimSpace(rawURL), "/")
	}
	parsed.Host = net.JoinHostPort(host, port)
	return strings.TrimRight(parsed.String(), "/")
}

func generateDeviceToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func fingerprintToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])[:8]
}

func setPairingConfig(
	cfg *rootconfig.Config,
	slot rootconfig.PairingSlot,
	token string,
	deviceName string,
	now time.Time,
) {
	cfg.SetPairingForSlot(
		slot,
		pairing.HashToken(token),
		strings.TrimSpace(deviceName),
		now.Format(time.RFC3339),
	)
}

func clearPairingConfig(cfg *rootconfig.Config, slot rootconfig.PairingSlot) {
	cfg.ClearPairingForSlot(slot)
}

func developerRequestFromStatus(status devenv.Status, enabled bool) DeveloperEnvironmentRequest {
	cfg := status.Config
	mode := firstNonBlank(cfg.Mode, status.Mode, string(devenv.ModeWorkspace))
	repoPath := firstNonBlank(cfg.RepoPath, status.ResolvedRepoPath, currentRepoRoot())
	baseURL := firstNonBlank(cfg.BaseURL, status.BaseURL)
	deviceName := firstNonBlank(cfg.DeviceName, "watch")
	hostAlias := firstNonBlank(cfg.HostAlias, status.HostAlias, "10.0.2.2")
	return DeveloperEnvironmentRequest{
		Enabled:              enabled,
		Mode:                 mode,
		RepoPath:             repoPath,
		BaseURL:              baseURL,
		DeviceName:           deviceName,
		ManagedTunnelEnabled: cfg.ManagedTunnel || status.ManagedTunnelEnabled,
		HostAlias:            hostAlias,
	}
}

func developerRequestFromSettings(value settings.DeveloperEnvironmentSettings, enabled bool) DeveloperEnvironmentRequest {
	return DeveloperEnvironmentRequest{
		Enabled:              enabled,
		Mode:                 firstNonBlank(value.Mode, string(devenv.ModeWorkspace)),
		RepoPath:             firstNonBlank(value.RepoPath, currentRepoRoot()),
		BaseURL:              firstNonBlank(value.BaseURL, defaultDeveloperBaseURL()),
		DeviceName:           firstNonBlank(value.DeviceName, "watch"),
		ManagedTunnelEnabled: value.ManagedTunnelEnabled,
		HostAlias:            firstNonBlank(value.HostAlias, "10.0.2.2"),
	}
}

func developerSettingsFromRequest(request DeveloperEnvironmentRequest, enabled bool) settings.DeveloperEnvironmentSettings {
	cfg := developerConfigFromRequest(request)
	return settings.DeveloperEnvironmentSettings{
		Enabled:              enabled,
		Mode:                 firstNonBlank(cfg.Mode, string(devenv.ModeWorkspace)),
		RepoPath:             firstNonBlank(cfg.RepoPath, currentRepoRoot()),
		BaseURL:              firstNonBlank(cfg.BaseURL, defaultDeveloperBaseURL()),
		DeviceName:           firstNonBlank(cfg.DeviceName, "watch"),
		HostAlias:            firstNonBlank(cfg.HostAlias, "10.0.2.2"),
		ManagedTunnelEnabled: cfg.ManagedTunnel,
	}
}

func (a *App) persistDeveloperEnvironmentPreference(request DeveloperEnvironmentRequest, enabled bool) error {
	loaded, err := settings.LoadDesktopSettings()
	if err != nil {
		loaded = settings.DefaultDesktopSettings()
	}
	loaded.DeveloperEnvironment = developerSettingsFromRequest(request, enabled)
	return settings.SaveDesktopSettings(loaded)
}

func (a *App) ensureDeveloperEnvironmentFromSettings(value settings.DeveloperEnvironmentSettings) error {
	if a.devEnvManager == nil {
		return nil
	}
	request := developerRequestFromSettings(value, true)
	cfg := developerConfigFromRequest(request)
	cfg.ManagedTunnel = a.devTunnelManager != nil && a.devTunnelManager.Status().Configured && request.ManagedTunnelEnabled
	_ = a.devEnvManager.Ensure(a.processContext(), cfg)
	if cfg.ManagedTunnel && strings.EqualFold(cfg.Mode, string(devenv.ModeWorkspace)) {
		originURL := rootconfig.DefaultPublicBaseURL(devSidecarLoopbackListen(cfg.BaseURL))
		_ = a.devTunnelManager.Start(a.processContext(), originURL)
	}
	return nil
}

func ensureManagedAPKDistDir(cfg *rootconfig.Config) {
	distDir := filepath.Clean(filepath.Join(settings.AppRoot(), "..", "dist"))
	if strings.TrimSpace(cfg.ApkDistDir) == "" ||
		strings.TrimSpace(cfg.ApkDistDir) == rootconfig.DefaultApkDistDir ||
		!filepath.IsAbs(strings.TrimSpace(cfg.ApkDistDir)) {
		cfg.ApkDistDir = distDir
	}
}

func (a *App) runtimeConfigForConfigMutation(
	manager *backend.Manager,
	defaultLoopbackPort string,
	base backend.StartConfig,
) backend.StartConfig {
	status := backend.DesktopStatus{}
	if manager != nil {
		status = manager.DesktopStatus()
	}
	cfg := backend.StartConfig{
		ConfigPath:    firstNonBlank(base.ConfigPath, settings.BackendConfigPath()),
		Listen:        firstNonBlank(base.Listen, status.ConfiguredListen),
		PublicBaseURL: firstNonBlank(base.PublicBaseURL, status.ConfiguredPublicBaseURL),
		PairingSlot:   firstNonBlank(base.PairingSlot, status.ConfiguredPairingSlot),
	}
	return a.resolveRuntimeStartConfigForManager(manager, defaultLoopbackPort, cfg)
}

func (a *App) overwriteBackendPairing(
	manager *backend.Manager,
	defaultLoopbackPort string,
	cfg backend.StartConfig,
	pairingSlot rootconfig.PairingSlot,
	token string,
	deviceName string,
) error {
	if !pairing.IsUsableToken(token) {
		return errors.New("生成的设备 token 无效")
	}
	if err := a.persistBackendPairing(cfg, pairingSlot, token, deviceName); err != nil {
		return err
	}
	runtimeCfg := a.runtimeConfigForConfigMutation(manager, defaultLoopbackPort, backend.StartConfig{
		ConfigPath:    firstNonBlank(cfg.ConfigPath, settings.BackendConfigPath()),
		Listen:        cfg.Listen,
		PublicBaseURL: cfg.PublicBaseURL,
		PairingSlot:   string(pairingSlot),
	})
	if manager == nil {
		return errors.New("本机服务管理器不可用")
	}
	if err := manager.RestartBackend(a.processContext(), runtimeCfg); err != nil {
		return fmt.Errorf("重启本机服务失败")
	}
	health := a.waitForBackendHealthWithManager(manager, runtimeCfg, 12*time.Second)
	if !health.OK {
		return errors.New("本机服务重启后未就绪")
	}
	return nil
}

func (a *App) persistBackendPairing(
	cfg backend.StartConfig,
	pairingSlot rootconfig.PairingSlot,
	token string,
	deviceName string,
) error {
	if !pairing.IsUsableToken(token) {
		return errors.New("生成的设备 token 无效")
	}
	configPath := firstNonBlank(cfg.ConfigPath, settings.BackendConfigPath())
	if strings.TrimSpace(configPath) == "" {
		return errors.New("未找到本机服务配置路径")
	}
	loaded, resolvedPath, err := rootconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("读取本机服务配置失败")
	}
	now := time.Now()
	setPairingConfig(&loaded, pairingSlot, token, deviceName, now)
	ensureManagedAPKDistDir(&loaded)
	currentPairing := loaded.PairingForSlot(pairingSlot)
	if err := pairing.RecordBinding(
		pairing.HistoryPath(resolvedPath),
		pairing.BindingRecord{
			TokenHash:  currentPairing.TokenHash,
			DeviceName: currentPairing.DeviceName,
			PairedAt:   currentPairing.PairedAt,
			Source:     "desktop-bootstrap",
		},
	); err != nil {
		return fmt.Errorf("记录配对历史失败")
	}
	if pairingSlot == rootconfig.PairingSlotDev {
		allowlistPath := pairing.ResolveRelativeToConfig(resolvedPath, loaded.DevUpdateAllowlist)
		if err := pairing.AddAllowlistTokenHash(allowlistPath, currentPairing.TokenHash); err != nil {
			return fmt.Errorf("更新 dev 白名单失败")
		}
	}
	if err := rootconfig.Save(resolvedPath, loaded); err != nil {
		return fmt.Errorf("保存配对信息失败")
	}
	return nil
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func currentRepoRoot() string {
	return filepath.Clean(filepath.Join(settings.AppRoot(), ".."))
}

func developerConfigFromRequest(request DeveloperEnvironmentRequest) devenv.Config {
	repoPath := strings.TrimSpace(request.RepoPath)
	if repoPath == "" {
		repoPath = currentRepoRoot()
	}
	baseURL := strings.TrimRight(strings.TrimSpace(request.BaseURL), "/")
	if baseURL == "" {
		hostAlias := firstNonBlank(strings.TrimSpace(request.HostAlias), "10.0.2.2")
		baseURL = "http://" + net.JoinHostPort(hostAlias, defaultDevSidecarPort)
	}
	return devenv.Config{
		Enabled:       request.Enabled,
		Mode:          string(devenv.ModeWorkspace),
		RepoPath:      repoPath,
		BaseURL:       baseURL,
		DeviceName:    firstNonBlank(request.DeviceName, "watch"),
		ManagedTunnel: request.ManagedTunnelEnabled,
		HostAlias:     firstNonBlank(request.HostAlias, "10.0.2.2"),
	}
}

func defaultDeveloperBaseURL() string {
	return "http://" + net.JoinHostPort("10.0.2.2", defaultDevSidecarPort)
}

func defaultDevSidecarLoopbackListen() string {
	return devSidecarLoopbackListen(defaultDeveloperBaseURL())
}

func devSidecarLoopbackListen(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return net.JoinHostPort("127.0.0.1", defaultDevSidecarPort)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return net.JoinHostPort("127.0.0.1", defaultDevSidecarPort)
	}
	port := strings.TrimSpace(parsed.Port())
	if port == "" {
		port = defaultDevSidecarPort
	}
	return net.JoinHostPort("127.0.0.1", port)
}

func developerRequestFromDevBootstrap(request DevBootstrapRequest) DeveloperEnvironmentRequest {
	return DeveloperEnvironmentRequest{
		Enabled:              true,
		Mode:                 string(devenv.ModeWorkspace),
		RepoPath:             strings.TrimSpace(request.RepoPath),
		HostAlias:            strings.TrimSpace(request.HostAlias),
		BaseURL:              strings.TrimSpace(request.BaseURL),
		DeviceName:           strings.TrimSpace(request.DeviceName),
		ManagedTunnelEnabled: request.ManagedTunnelEnabled,
	}
}

func normalizeRemoteWatchEnvironment(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "dev") {
		return "dev"
	}
	return "beta"
}

func pairDevice(ctx context.Context, cfg backend.StartConfig, token, deviceName string) error {
	body, err := json.Marshal(map[string]string{
		"deviceToken": token,
		"deviceName":  deviceName,
	})
	if err != nil {
		return fmt.Errorf("构造配对请求失败")
	}
	endpoint := rootconfig.DefaultPublicBaseURL(cfg.Listen) + "/api/pair"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("构造配对请求失败")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("保存配对信息失败")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("当前本机服务已存在配对信息，请先清空配对后再重新生成")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("保存配对信息失败，HTTP %d", resp.StatusCode)
	}
	return nil
}

func buildPairURL(apiBase, token, deviceName string) string {
	values := url.Values{}
	values.Set("deviceToken", token)
	if strings.TrimSpace(deviceName) != "" {
		values.Set("deviceName", deviceName)
	}
	return strings.TrimRight(apiBase, "/") + "/pair?" + values.Encode()
}

func normalizeBootstrapEndpoints(requested []BootstrapEndpointRequest, fallbackURL string) ([]BootstrapEndpointRequest, error) {
	if len(requested) == 0 {
		trimmed := strings.TrimSpace(fallbackURL)
		if trimmed == "" {
			return nil, fmt.Errorf("至少需要一个可写入的入口")
		}
		return []BootstrapEndpointRequest{
			{
				ID:       "primary",
				Label:    "默认地址",
				URL:      strings.TrimRight(trimmed, "/"),
				Priority: 0,
			},
		}, nil
	}

	normalized := make([]BootstrapEndpointRequest, 0, len(requested))
	for index, item := range requested {
		id := strings.TrimSpace(item.ID)
		label := strings.TrimSpace(item.Label)
		rawURL := strings.TrimSpace(item.URL)
		if id == "" || label == "" || rawURL == "" {
			return nil, fmt.Errorf("入口配置不完整")
		}
		if _, err := url.ParseRequestURI(rawURL); err != nil {
			return nil, fmt.Errorf("入口地址非法")
		}
		normalized = append(normalized, BootstrapEndpointRequest{
			ID:       id,
			Label:    label,
			URL:      strings.TrimRight(rawURL, "/"),
			Priority: index,
		})
	}
	return normalized, nil
}

func buildBootstrapURI(endpoints []BootstrapEndpointRequest, token, deviceName string) (string, error) {
	return buildBootstrapURIWithHost("bootstrap", "desktop-bootstrap", endpoints, token, deviceName)
}

func buildDevBootstrapURI(endpoints []BootstrapEndpointRequest, token, deviceName string) (string, error) {
	return buildBootstrapURIWithHost("dev-bootstrap", "desktop-dev", endpoints, token, deviceName)
}

func buildBootstrapURIWithHost(host, source string, endpoints []BootstrapEndpointRequest, token, deviceName string) (string, error) {
	payloadBytes, err := json.Marshal(endpoints)
	if err != nil {
		return "", fmt.Errorf("构造入口列表失败")
	}
	values := url.Values{}
	values.Set("endpoints", base64.RawURLEncoding.EncodeToString(payloadBytes))
	values.Set("deviceToken", token)
	if strings.TrimSpace(deviceName) != "" {
		values.Set("deviceName", deviceName)
	}
	values.Set("source", source)
	return "openwatcher://" + host + "?" + values.Encode(), nil
}

func validateBackendRequestMode(request BackendRequest) error {
	mode := network.NormalizeMode(request.Mode)
	switch mode {
	case network.ModePublicURL:
		_, err := network.ValidatePublicURL(request.CustomURL, true)
		return err
	default:
		return nil
	}
}

func invalidBackendStatus(request BackendRequest, cfg backend.StartConfig, message string) backend.DesktopStatus {
	health := constrainedHealthStatus(request, cfg, backend.HealthStatus{
		OK:       false,
		Message:  message,
		Endpoint: healthEndpointForMode(request, cfg),
	})
	return backend.DesktopStatus{
		State:                   "error",
		Message:                 message,
		FriendlyError:           message,
		HealthProbePath:         health.Endpoint,
		ConfigPathLabel:         trimConfigPathLabel(cfg.ConfigPath),
		ConfiguredListen:        cfg.Listen,
		ConfiguredPublicBaseURL: cfg.PublicBaseURL,
		LastHealth:              &health,
	}
}

func constrainedHealthStatus(request BackendRequest, cfg backend.StartConfig, status backend.HealthStatus) backend.HealthStatus {
	status.Endpoint = healthEndpointForMode(request, cfg)
	mode := network.NormalizeMode(request.Mode)
	switch mode {
	case network.ModePublicURL:
		if status.HTTPCode == 0 && !status.OK && strings.TrimSpace(status.Message) == "" {
			status.Message = "无法访问自定义公网 URL 的 /healthz"
			return status
		}
		if !looksLikeOpenWatcherHealth(status) {
			status.OK = false
			status.Message = "自定义公网 URL 的 /healthz 不是 OpenWatcher 响应，请检查反向代理目标是否正确"
			return status
		}
		if status.Config.NoAuth {
			status.OK = false
			status.Message = "自定义公网 URL 对应的 OpenWatcher 后端启用了 no-auth。公开发布路径不允许这样配置"
			return status
		}
		if !status.OK {
			if status.HTTPCode > 0 {
				status.Message = fmt.Sprintf("自定义公网 URL 的 /healthz 返回 HTTP %d", status.HTTPCode)
			} else if strings.TrimSpace(status.Message) == "" {
				status.Message = "自定义公网 URL 的 /healthz 校验失败"
			}
			return status
		}
		status.Message = "已验证自定义公网 URL 的 /healthz，并确认目标为 OpenWatcher"
	}
	return status
}

func healthEndpointForMode(request BackendRequest, cfg backend.StartConfig) string {
	mode := network.NormalizeMode(request.Mode)
	switch mode {
	case network.ModePublicURL:
		if strings.TrimSpace(cfg.PublicBaseURL) == "" {
			return ""
		}
		return strings.TrimRight(cfg.PublicBaseURL, "/") + "/healthz"
	case network.ModeManagedBeta:
		if strings.TrimSpace(cfg.PublicBaseURL) == "" {
			return ""
		}
		return strings.TrimRight(cfg.PublicBaseURL, "/") + "/healthz"
	default:
		return deriveHealthEndpointFromConfig(cfg)
	}
}

func deriveHealthEndpointFromConfig(cfg backend.StartConfig) string {
	listen := strings.TrimSpace(cfg.Listen)
	if listen == "" {
		listen = rootconfig.DefaultListen
	}
	return rootconfig.DefaultPublicBaseURL(listen) + "/healthz"
}

func looksLikeOpenWatcherHealth(status backend.HealthStatus) bool {
	return (strings.TrimSpace(status.Build.Version) != "" || strings.TrimSpace(status.Build.Commit) != "" || strings.TrimSpace(status.Build.BuiltAt) != "") &&
		(strings.TrimSpace(status.Config.Listen) != "" || strings.TrimSpace(status.Config.PublicBaseURL) != "")
}

func probeOpenWatcherHealth(ctx context.Context, baseURL string) (backend.HealthStatus, error) {
	return backend.ProbePublicHealth(ctx, baseURL, "自定义公网 URL")
}

func (a *App) validateRequest(request BackendRequest) error {
	if err := validateBackendRequestMode(request); err != nil {
		return err
	}
	if network.NormalizeMode(request.Mode) == network.ModeManagedBeta {
		status := a.tunnelManager.Status()
		if !status.Configured || strings.TrimSpace(status.PublicBaseURL) == "" {
			return errors.New("请先输入配置码并完成兑换，再启动 OpenWatcher 托管隧道。")
		}
	}
	return nil
}

func (a *App) postStartHealthCheck(request BackendRequest, cfg backend.StartConfig) backend.HealthStatus {
	mode := network.NormalizeMode(request.Mode)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	if mode != network.ModeManagedBeta {
		return a.checkHealthWithRequest(ctx, request)
	}

	localHealth := a.waitForBackendHealth(cfg, 12*time.Second)
	if !localHealth.OK {
		localHealth.Message = "本地 OpenWatcher 本机服务未就绪，请先启动本机服务。"
		return localHealth
	}
	if err := a.tunnelManager.Start(a.processContext(), rootconfig.DefaultPublicBaseURL(cfg.Listen)); err != nil {
		localHealth.Message = "启动托管隧道失败: " + err.Error()
		return localHealth
	}

	remoteCtx, remoteCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer remoteCancel()
	if remoteHealth, err := a.tunnelManager.PublicHealth(remoteCtx); err == nil {
		return remoteHealth
	}
	return localHealth
}

func (a *App) waitForBackendHealth(cfg backend.StartConfig, timeout time.Duration) backend.HealthStatus {
	return a.waitForBackendHealthWithManager(a.backendManager, cfg, timeout)
}

func (a *App) waitForBackendHealthWithManager(
	manager *backend.Manager,
	cfg backend.StartConfig,
	timeout time.Duration,
) backend.HealthStatus {
	if manager == nil {
		return backend.HealthStatus{Message: "本机服务管理器不可用"}
	}
	deadline := time.Now().Add(timeout)
	var last backend.HealthStatus
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		status, _ := manager.CheckHealthWithConfig(ctx, cfg)
		cancel()
		last = status
		if status.OK {
			return status
		}
		time.Sleep(300 * time.Millisecond)
	}
	return last
}

func (a *App) backendNeedsRestart(cfg backend.StartConfig) bool {
	cfg = normalizeDesktopStartConfig(cfg)
	status := a.backendManager.DesktopStatus()
	if !status.Running {
		return false
	}
	return strings.TrimSpace(status.ConfiguredListen) != cfg.Listen ||
		rootconfig.NormalizePublicBaseURL(status.ConfiguredPublicBaseURL) != cfg.PublicBaseURL ||
		strings.TrimSpace(status.ConfiguredPairingSlot) != strings.TrimSpace(cfg.PairingSlot)
}

func trimConfigPathLabel(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	home, err := rootconfig.ResolveUserHome()
	if err != nil || strings.TrimSpace(home) == "" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
