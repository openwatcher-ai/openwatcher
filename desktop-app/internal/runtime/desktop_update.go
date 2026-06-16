package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"time"

	"openwatcher/desktop-app/internal/processutil"
	"openwatcher/desktop-app/internal/selfupdate"
	"openwatcher/desktop-app/internal/settings"
)

type DesktopUpdateProgress struct {
	Phase           string `json:"phase"`
	Message         string `json:"message"`
	Percent         int    `json:"percent"`
	DownloadedBytes int64  `json:"downloadedBytes,omitempty"`
	TotalBytes      int64  `json:"totalBytes,omitempty"`
}

type DesktopUpdateInstallResult struct {
	Phase    string `json:"phase"`
	Message  string `json:"message"`
	Version  string `json:"version,omitempty"`
	Artifact string `json:"artifact,omitempty"`
}

type DesktopUpdateStatus struct {
	Phase      string `json:"phase"`
	Message    string `json:"message"`
	Version    string `json:"version,omitempty"`
	Artifact   string `json:"artifact,omitempty"`
	BackupPath string `json:"backupPath,omitempty"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
}

func (m *Manager) GetDesktopUpdateStatus() DesktopUpdateStatus {
	status, err := selfupdate.ReadStatus(m.desktopUpdateStatusPath())
	if err != nil {
		return DesktopUpdateStatus{}
	}
	return DesktopUpdateStatus{
		Phase:      status.Phase,
		Message:    status.Message,
		Version:    status.Version,
		Artifact:   status.Artifact,
		BackupPath: status.BackupPath,
		UpdatedAt:  status.UpdatedAt,
	}
}

func (m *Manager) PrepareDesktopUpdate(ctx context.Context, progress func(DesktopUpdateProgress)) (DesktopUpdateInstallResult, error) {
	report := func(phase string, message string, percent int, downloaded int64, total int64) {
		if progress != nil {
			progress(DesktopUpdateProgress{
				Phase:           phase,
				Message:         message,
				Percent:         percent,
				DownloadedBytes: downloaded,
				TotalBytes:      total,
			})
		}
	}

	check, err := m.CheckForUpdates(ctx, "")
	if err != nil {
		return DesktopUpdateInstallResult{}, err
	}
	if !check.DesktopUpdateAvailable {
		return DesktopUpdateInstallResult{}, fmt.Errorf("当前 Desktop 已是最新版本")
	}
	if !check.DesktopInstallable {
		if strings.TrimSpace(check.DesktopInstallMessage) != "" {
			return DesktopUpdateInstallResult{}, errors.New(check.DesktopInstallMessage)
		}
		return DesktopUpdateInstallResult{}, fmt.Errorf("当前 Desktop 更新包不可自动安装")
	}

	resource := ResourceDescriptor{
		Version:     check.LatestDesktopVersion,
		Artifact:    check.DesktopArtifact,
		URL:         check.DesktopDownloadURL,
		SHA256:      check.DesktopSHA256,
		SizeBytes:   check.DesktopSizeBytes,
		ArchiveKind: check.DesktopArchiveKind,
	}
	_ = selfupdate.WriteStatus(m.desktopUpdateStatusPath(), selfupdate.NewStatus("downloading", "正在下载 Desktop 更新包", resource.Version, resource.Artifact, ""))
	report("downloading", "正在下载 Desktop 更新包", 0, 0, resource.SizeBytes)
	archivePath, err := m.downloadDesktopArchive(ctx, resource, report)
	if err != nil {
		_ = selfupdate.WriteStatus(m.desktopUpdateStatusPath(), selfupdate.NewStatus("failed", err.Error(), resource.Version, resource.Artifact, ""))
		return DesktopUpdateInstallResult{}, err
	}

	report("extracting", "正在解压 Desktop 更新包", 100, resource.SizeBytes, resource.SizeBytes)
	sourceDir, err := m.stageDesktopUpdate(archivePath)
	if err != nil {
		_ = selfupdate.WriteStatus(m.desktopUpdateStatusPath(), selfupdate.NewStatus("failed", err.Error(), resource.Version, resource.Artifact, ""))
		return DesktopUpdateInstallResult{}, err
	}

	helperPath, err := m.prepareUpdaterHelper()
	if err != nil {
		_ = selfupdate.WriteStatus(m.desktopUpdateStatusPath(), selfupdate.NewStatus("failed", err.Error(), resource.Version, resource.Artifact, ""))
		return DesktopUpdateInstallResult{}, err
	}
	targetPath, launchPath, err := currentDesktopInstallPaths(m.platform)
	if err != nil {
		_ = selfupdate.WriteStatus(m.desktopUpdateStatusPath(), selfupdate.NewStatus("failed", err.Error(), resource.Version, resource.Artifact, ""))
		return DesktopUpdateInstallResult{}, err
	}

	report("restarting", "正在启动更新程序，Desktop 将自动重启", 100, resource.SizeBytes, resource.SizeBytes)
	_ = selfupdate.WriteStatus(m.desktopUpdateStatusPath(), selfupdate.NewStatus("restarting", "正在启动更新程序，Desktop 将自动重启", resource.Version, resource.Artifact, ""))
	if err := startUpdaterHelper(helperPath, selfupdate.HelperOptions{
		SourceDir:  sourceDir,
		TargetPath: targetPath,
		LaunchPath: launchPath,
		StatusPath: m.desktopUpdateStatusPath(),
		BackupRoot: m.desktopUpdateBackupRoot(),
		Platform:   stdruntime.GOOS,
		Version:    resource.Version,
		Artifact:   resource.Artifact,
	}, m.desktopUpdateLogPath()); err != nil {
		_ = selfupdate.WriteStatus(m.desktopUpdateStatusPath(), selfupdate.NewStatus("failed", err.Error(), resource.Version, resource.Artifact, ""))
		return DesktopUpdateInstallResult{}, err
	}

	return DesktopUpdateInstallResult{
		Phase:    "restarting",
		Message:  "更新程序已启动，Desktop 将自动重启",
		Version:  resource.Version,
		Artifact: resource.Artifact,
	}, nil
}

func (m *Manager) downloadDesktopArchive(ctx context.Context, resource ResourceDescriptor, progress func(string, string, int, int64, int64)) (string, error) {
	runtimeRoot, err := m.RuntimeRoot()
	if err != nil {
		return "", err
	}
	downloadsDir := filepath.Join(runtimeRoot, "downloads")
	if err := os.MkdirAll(downloadsDir, 0o755); err != nil {
		return "", err
	}
	targetPath := filepath.Join(downloadsDir, resource.Artifact)
	if match, _ := fileMatchesSHA256(targetPath, resource.SHA256); match {
		if resource.SizeBytes <= 0 {
			return targetPath, nil
		}
		if info, err := os.Stat(targetPath); err == nil && info.Size() == resource.SizeBytes {
			progress("downloaded", "Desktop 更新包已下载", 100, resource.SizeBytes, resource.SizeBytes)
			return targetPath, nil
		}
	}
	_ = os.Remove(targetPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resource.URL, nil)
	if err != nil {
		return "", fmt.Errorf("构造 Desktop 更新下载请求失败")
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载 Desktop 更新失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载 Desktop 更新失败：HTTP %d", resp.StatusCode)
	}
	tmpPath := targetPath + ".part"
	file, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	var downloaded int64
	buffer := make([]byte, 128*1024)
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			written, writeErr := file.Write(buffer[:n])
			if writeErr != nil {
				file.Close()
				_ = os.Remove(tmpPath)
				return "", fmt.Errorf("写入 Desktop 更新包失败")
			}
			downloaded += int64(written)
			percent := 0
			if resource.SizeBytes > 0 {
				percent = int(downloaded * 100 / resource.SizeBytes)
				if percent > 100 {
					percent = 100
				}
			}
			progress("downloading", "正在下载 Desktop 更新包", percent, downloaded, resource.SizeBytes)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			file.Close()
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("读取 Desktop 更新响应失败")
		}
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if resource.SizeBytes > 0 {
		if info, err := os.Stat(tmpPath); err == nil && info.Size() != resource.SizeBytes {
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("Desktop 更新包大小不匹配：%s", resource.Artifact)
		}
	}
	match, err := fileMatchesSHA256(tmpPath, resource.SHA256)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if !match {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("Desktop 更新包校验失败：%s", resource.Artifact)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	progress("downloaded", "Desktop 更新包已下载", 100, resource.SizeBytes, resource.SizeBytes)
	return targetPath, nil
}

func (m *Manager) stageDesktopUpdate(archivePath string) (string, error) {
	root := filepath.Join(m.desktopUpdateRoot(), "staged", time.Now().UTC().Format("20060102-150405"))
	if err := os.RemoveAll(root); err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	if err := extractZip(archivePath, root, ""); err != nil {
		_ = os.RemoveAll(root)
		return "", fmt.Errorf("解压 Desktop 更新包失败：%w", err)
	}
	sourceDir, err := findStagedDesktopRoot(root, m.platform)
	if err != nil {
		_ = os.RemoveAll(root)
		return "", err
	}
	return sourceDir, nil
}

func findStagedDesktopRoot(root string, platform string) (string, error) {
	if strings.HasPrefix(platform, "darwin-") {
		var appPath string
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if appPath != "" {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".app") {
				appPath = path
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return "", err
		}
		if appPath == "" {
			return "", fmt.Errorf("Desktop 更新包内未找到 .app")
		}
		return appPath, nil
	}

	candidates := []string{root}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if entry.IsDir() {
			candidates = append(candidates, filepath.Join(root, entry.Name()))
		}
	}
	for _, candidate := range candidates {
		if fileExists(filepath.Join(candidate, desktopExecutableName(platform))) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Desktop 更新包内未找到可执行文件")
}

func (m *Manager) prepareUpdaterHelper() (string, error) {
	source, err := m.resolveUpdaterHelper()
	if err != nil {
		return "", err
	}
	targetDir := filepath.Join(m.desktopUpdateRoot(), "helper")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(targetDir, strings.TrimSuffix(desktopUpdaterName(m.platform), ".exe")+"-"+time.Now().UTC().Format("20060102-150405")+helperExtension(m.platform))
	if err := copyFile(source, target, 0o755); err != nil {
		return "", err
	}
	return target, nil
}

func (m *Manager) resolveUpdaterHelper() (string, error) {
	if strings.TrimSpace(m.updaterHelperPath) != "" {
		return m.updaterHelperPath, nil
	}
	name := desktopUpdaterName(m.platform)
	for _, root := range settings.BundledResourceRoots(m.appRoot) {
		candidate := filepath.Join(root, "updater", m.platform, name)
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未找到 Desktop 更新程序：%s", name)
}

func startUpdaterHelper(helperPath string, options selfupdate.HelperOptions, logPath string) error {
	args := []string{
		"--source", options.SourceDir,
		"--target", options.TargetPath,
		"--launch", options.LaunchPath,
		"--status", options.StatusPath,
		"--backup-root", options.BackupRoot,
		"--platform", options.Platform,
		"--version", options.Version,
		"--artifact", options.Artifact,
	}
	cmd := exec.Command(helperPath, args...)
	processutil.HideConsoleWindow(cmd)
	var logFile *os.File
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err == nil {
		if opened, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); openErr == nil {
			logFile = opened
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		}
	}
	err := cmd.Start()
	if logFile != nil {
		_ = logFile.Close()
	}
	return err
}

func currentDesktopInstallPaths(platform string) (string, string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	exePath, _ = filepath.Abs(exePath)
	if strings.HasPrefix(platform, "darwin-") {
		exeDir := filepath.Dir(exePath)
		contentsDir := filepath.Dir(exeDir)
		if filepath.Base(exeDir) != "MacOS" || filepath.Base(contentsDir) != "Contents" {
			return "", "", fmt.Errorf("当前运行环境不是 macOS App Bundle，不能自动替换")
		}
		appPath := filepath.Dir(contentsDir)
		return appPath, appPath, nil
	}
	targetDir := filepath.Dir(exePath)
	return targetDir, filepath.Join(targetDir, desktopExecutableName(platform)), nil
}

func (m *Manager) desktopUpdateRoot() string {
	runtimeRoot, err := m.RuntimeRoot()
	if err != nil {
		return filepath.Join(os.TempDir(), "openwatcher-self-update")
	}
	return filepath.Join(runtimeRoot, "self-update")
}

func (m *Manager) desktopUpdateStatusPath() string {
	return filepath.Join(m.desktopUpdateRoot(), "status.json")
}

func (m *Manager) desktopUpdateBackupRoot() string {
	return filepath.Join(m.desktopUpdateRoot(), "backups")
}

func (m *Manager) desktopUpdateLogPath() string {
	return filepath.Join(m.desktopUpdateRoot(), "updater.log")
}

func desktopExecutableName(platform string) string {
	if strings.HasPrefix(platform, "windows-") {
		return "openwatcher.exe"
	}
	return "openwatcher"
}

func desktopUpdaterName(platform string) string {
	if strings.HasPrefix(platform, "windows-") {
		return "openwatcher-updater.exe"
	}
	return "openwatcher-updater"
}

func helperExtension(platform string) string {
	if strings.HasPrefix(platform, "windows-") {
		return ".exe"
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
