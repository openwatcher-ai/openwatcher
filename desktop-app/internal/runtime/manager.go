package runtime

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"sync"
	"time"

	"openwatcher/desktop-app/internal/settings"
)

const defaultChannelManifestURL = "https://openwatcher.ai/channels/beta.json"

type Manager struct {
	appRoot            string
	desktopVersion     string
	channelManifestURL string
	platform           string
	httpClient         *http.Client
	now                func() time.Time
	fetchTTL           time.Duration
	updaterHelperPath  string

	mu            sync.Mutex
	manifest      *Manifest
	lastFetchedAt time.Time

	ensureMu sync.Mutex

	progressMu sync.Mutex
	progress   map[ResourceKind]ResourceProgress

	asyncMu       sync.Mutex
	ensureRunning bool
}

type ResourcePhase string

const (
	ResourcePhaseChecking    ResourcePhase = "checking"
	ResourcePhaseDownloading ResourcePhase = "downloading"
	ResourcePhaseVerifying   ResourcePhase = "verifying"
	ResourcePhaseExtracting  ResourcePhase = "extracting"
	ResourcePhaseReady       ResourcePhase = "ready"
	ResourcePhaseError       ResourcePhase = "error"
)

type ResourceProgress struct {
	Kind           string        `json:"kind"`
	Artifact       string        `json:"artifact,omitempty"`
	Version        string        `json:"version,omitempty"`
	Phase          ResourcePhase `json:"phase"`
	Ready          bool          `json:"ready"`
	Downloaded     int64         `json:"downloadedBytes,omitempty"`
	Total          int64         `json:"totalBytes,omitempty"`
	Percent        int           `json:"percent,omitempty"`
	BytesPerSecond int64         `json:"bytesPerSecond,omitempty"`
	Message        string        `json:"message,omitempty"`
	UpdatedAt      string        `json:"updatedAt,omitempty"`
}

type Status struct {
	Platform  string                      `json:"platform"`
	Resources map[string]ResourceProgress `json:"resources"`
}

type resourceState struct {
	Kind            string   `json:"kind"`
	Platform        string   `json:"platform,omitempty"`
	Artifact        string   `json:"artifact"`
	SHA256          string   `json:"sha256"`
	URL             string   `json:"url"`
	Version         string   `json:"version"`
	VersionName     string   `json:"versionName,omitempty"`
	VersionCode     int      `json:"versionCode,omitempty"`
	ArchiveKind     string   `json:"archiveKind,omitempty"`
	BinRelativePath string   `json:"binRelativePath,omitempty"`
	ExtraFiles      []string `json:"extraFiles,omitempty"`
	UpdatedAt       string   `json:"updatedAt"`
}

func NewManager(appRoot string, desktopVersion string) *Manager {
	return &Manager{
		appRoot:            appRoot,
		desktopVersion:     strings.TrimSpace(desktopVersion),
		channelManifestURL: resolveChannelManifestURL(appRoot),
		platform:           stdruntime.GOOS + "-" + stdruntime.GOARCH,
		httpClient:         &http.Client{Timeout: 90 * time.Second},
		now:                time.Now,
		fetchTTL:           15 * time.Minute,
		progress:           make(map[ResourceKind]ResourceProgress),
	}
}

func (m *Manager) StartEnsureInstaller() {
	m.startEnsure(ResourcePlatformTools, ResourceWatchAPK)
}

func (m *Manager) StartEnsureAll() {
	m.startEnsure(ResourcePlatformTools, ResourceWatchAPK, ResourceCloudflared)
}

func (m *Manager) startEnsure(kinds ...ResourceKind) {
	m.asyncMu.Lock()
	if m.ensureRunning {
		m.asyncMu.Unlock()
		return
	}
	m.ensureRunning = true
	m.asyncMu.Unlock()

	go func() {
		defer func() {
			m.asyncMu.Lock()
			m.ensureRunning = false
			m.asyncMu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = m.ensure(ctx, kinds...)
	}()
}

func (m *Manager) Status() Status {
	m.progressMu.Lock()
	defer m.progressMu.Unlock()
	resources := make(map[string]ResourceProgress, len(m.progress))
	for kind, progress := range m.progress {
		resources[string(kind)] = progress
	}
	return Status{
		Platform:  m.platform,
		Resources: resources,
	}
}

func (m *Manager) EnsureInstaller(ctx context.Context) error {
	return m.ensure(ctx, ResourcePlatformTools, ResourceWatchAPK)
}

func (m *Manager) EnsureTunnel(ctx context.Context) error {
	return m.ensure(ctx, ResourceCloudflared)
}

func (m *Manager) EnsureAll(ctx context.Context) error {
	return m.ensure(ctx, ResourcePlatformTools, ResourceWatchAPK, ResourceCloudflared)
}

func (m *Manager) RuntimeRoot() (string, error) {
	return settings.RuntimeDir()
}

func (m *Manager) ensure(ctx context.Context, kinds ...ResourceKind) error {
	m.ensureMu.Lock()
	defer m.ensureMu.Unlock()

	manifest, err := m.currentManifest(ctx)
	if err != nil {
		return err
	}
	if err := ensureDesktopVersion(manifest.DesktopMinVersion, m.desktopVersion); err != nil {
		return err
	}
	for _, kind := range kinds {
		if err := m.ensureResource(ctx, manifest, kind); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) currentManifest(ctx context.Context) (Manifest, error) {
	m.mu.Lock()
	manifest := m.manifest
	shouldFetch := manifest == nil || m.now().Sub(m.lastFetchedAt) >= m.fetchTTL
	m.mu.Unlock()

	var fetchErr error
	if shouldFetch {
		fetched, err := m.fetchManifest(ctx)
		if err == nil {
			if err := m.writeCurrentManifest(fetched); err != nil {
				return Manifest{}, err
			}
			m.mu.Lock()
			m.manifest = &fetched
			m.lastFetchedAt = m.now()
			m.mu.Unlock()
			return fetched, nil
		}
		fetchErr = err
	}

	if manifest != nil {
		return *manifest, nil
	}
	if cached, err := LoadCachedManifest(); err == nil {
		m.mu.Lock()
		m.manifest = &cached
		m.mu.Unlock()
		return cached, nil
	} else if fetchErr == nil {
		fetchErr = err
	}
	if fetchErr != nil {
		return Manifest{}, fetchErr
	}
	return Manifest{}, fmt.Errorf("未找到可用的 runtime manifest")
}

func (m *Manager) fetchManifest(ctx context.Context) (Manifest, error) {
	channelManifest, err := m.fetchChannelManifest(ctx)
	if err != nil {
		return Manifest{}, err
	}
	return m.fetchRuntimeManifest(ctx, channelManifest.Runtime.ManifestURL, channelManifest.Runtime.ManifestSHA256)
}

func (m *Manager) writeCurrentManifest(manifest Manifest) error {
	runtimeRoot, err := m.RuntimeRoot()
	if err != nil {
		return err
	}
	target := filepath.Join(runtimeRoot, "manifests", "current.json")
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomically(target, payload, 0o644)
}

func (m *Manager) ensureResource(ctx context.Context, manifest Manifest, kind ResourceKind) error {
	resource, err := manifest.Resource(kind, m.platform)
	if err != nil {
		m.setResourceError(kind, ResourceDescriptor{}, err)
		return err
	}
	m.setResourceProgress(kind, resource, ResourcePhaseChecking, 0, resource.SizeBytes, 0, false, resourceCheckingMessage(kind))
	switch kind {
	case ResourceWatchAPK:
		err = m.ensureWatchAPK(ctx, resource)
	case ResourcePlatformTools:
		err = m.ensureArchiveResource(ctx, kind, resource, filepath.Join("platform-tools", m.platform))
	case ResourceCloudflared:
		err = m.ensureArchiveResource(ctx, kind, resource, filepath.Join("cloudflared", m.platform))
	default:
		err = fmt.Errorf("未知 runtime 资源类型：%s", kind)
	}
	if err != nil {
		m.setResourceError(kind, resource, err)
	}
	return err
}

func (m *Manager) ensureWatchAPK(ctx context.Context, resource ResourceDescriptor) error {
	runtimeRoot, err := m.RuntimeRoot()
	if err != nil {
		return err
	}
	targetDir := filepath.Join(runtimeRoot, "watch-apk")
	targetPath := filepath.Join(targetDir, "openwatcher-watch-release.apk")
	statePath := filepath.Join(targetDir, "metadata.json")
	if stateMatches(statePath, ResourceWatchAPK, m.platform, resource) {
		if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
			m.setResourceReady(ResourceWatchAPK, resource, "手表安装包已就绪")
			return nil
		}
	}

	downloadPath, err := m.ensureDownload(ctx, ResourceWatchAPK, resource)
	if err != nil {
		return err
	}
	m.setResourceProgress(ResourceWatchAPK, resource, ResourcePhaseVerifying, resource.SizeBytes, resource.SizeBytes, 0, false, "正在写入手表安装包")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	tmpPath := targetPath + ".tmp"
	if err := copyFile(downloadPath, tmpPath, 0o644); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := writeResourceState(statePath, ResourceWatchAPK, m.platform, resource, m.now()); err != nil {
		return err
	}
	m.setResourceReady(ResourceWatchAPK, resource, "手表安装包已就绪")
	return nil
}

func (m *Manager) ensureArchiveResource(ctx context.Context, kind ResourceKind, resource ResourceDescriptor, relativeTarget string) error {
	runtimeRoot, err := m.RuntimeRoot()
	if err != nil {
		return err
	}
	targetDir := filepath.Join(runtimeRoot, filepath.FromSlash(relativeTarget))
	statePath := filepath.Join(targetDir, ".resource.json")
	if stateMatches(statePath, kind, m.platform, resource) && resourceFilesReady(targetDir, resource) {
		m.setResourceReady(kind, resource, resourceReadyMessage(kind))
		return nil
	}

	downloadPath, err := m.ensureDownload(ctx, kind, resource)
	if err != nil {
		return err
	}
	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return err
	}
	tempDir, err := os.MkdirTemp(parentDir, filepath.Base(targetDir)+".tmp-")
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(tempDir)
		}
	}()
	switch strings.ToLower(strings.TrimSpace(resource.ArchiveKind)) {
	case "zip":
		m.setResourceProgress(kind, resource, ResourcePhaseExtracting, resource.SizeBytes, resource.SizeBytes, 0, false, resourceExtractingMessage(kind))
		if err := extractZip(downloadPath, tempDir, archiveStripPrefix(resource.BinRelativePath)); err != nil {
			return err
		}
	case "tgz":
		m.setResourceProgress(kind, resource, ResourcePhaseExtracting, resource.SizeBytes, resource.SizeBytes, 0, false, resourceExtractingMessage(kind))
		if err := extractTGZ(downloadPath, tempDir, archiveStripPrefix(resource.BinRelativePath)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("不支持的归档格式：%s", resource.ArchiveKind)
	}
	if !resourceFilesReady(tempDir, resource) {
		return fmt.Errorf("%s 解压后缺少预期文件", kind)
	}
	if err := writeResourceState(filepath.Join(tempDir, ".resource.json"), kind, m.platform, resource, m.now()); err != nil {
		return err
	}
	_ = os.RemoveAll(targetDir)
	if err := os.Rename(tempDir, targetDir); err != nil {
		return err
	}
	ok = true
	m.setResourceReady(kind, resource, resourceReadyMessage(kind))
	return nil
}

func (m *Manager) ensureDownload(ctx context.Context, kind ResourceKind, resource ResourceDescriptor) (string, error) {
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
		m.setResourceProgress(kind, resource, ResourcePhaseVerifying, resource.SizeBytes, resource.SizeBytes, 0, false, resourceCachedMessage(kind))
		return targetPath, nil
	}
	_ = os.Remove(targetPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resource.URL, nil)
	if err != nil {
		return "", fmt.Errorf("构造资源下载请求失败")
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载资源失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载资源失败：HTTP %d", resp.StatusCode)
	}
	tmpPath := targetPath + ".part"
	file, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	totalBytes := resource.SizeBytes
	if resp.ContentLength > 0 {
		totalBytes = resp.ContentLength
	}
	m.setResourceProgress(kind, resource, ResourcePhaseDownloading, 0, totalBytes, 0, false, resourceDownloadingMessage(kind))
	if err := m.copyDownloadWithProgress(ctx, kind, resource, file, resp.Body, totalBytes); err != nil {
		file.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("写入资源文件失败")
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if resource.SizeBytes > 0 {
		if info, err := os.Stat(tmpPath); err == nil && info.Size() != resource.SizeBytes {
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("下载资源大小不匹配：%s", resource.Artifact)
		}
	}
	match, err := fileMatchesSHA256(tmpPath, resource.SHA256)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if !match {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("下载资源校验失败：%s", resource.Artifact)
	}
	m.setResourceProgress(kind, resource, ResourcePhaseVerifying, totalBytes, totalBytes, 0, false, resourceVerifyingMessage(kind))
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return targetPath, nil
}

func (m *Manager) copyDownloadWithProgress(ctx context.Context, kind ResourceKind, resource ResourceDescriptor, target *os.File, source io.Reader, totalBytes int64) error {
	buffer := make([]byte, 32*1024)
	startedAt := time.Now()
	lastReportedAt := startedAt
	var downloaded int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := source.Read(buffer)
		if n > 0 {
			if _, err := target.Write(buffer[:n]); err != nil {
				return err
			}
			downloaded += int64(n)
			now := time.Now()
			if now.Sub(lastReportedAt) >= 150*time.Millisecond || downloaded == totalBytes {
				m.setResourceProgress(kind, resource, ResourcePhaseDownloading, downloaded, totalBytes, downloadSpeed(downloaded, startedAt, now), false, resourceDownloadingMessage(kind))
				lastReportedAt = now
			}
		}
		if readErr == io.EOF {
			m.setResourceProgress(kind, resource, ResourcePhaseDownloading, downloaded, totalBytes, downloadSpeed(downloaded, startedAt, time.Now()), false, resourceDownloadingMessage(kind))
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func downloadSpeed(downloaded int64, startedAt time.Time, now time.Time) int64 {
	elapsed := now.Sub(startedAt).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return int64(float64(downloaded) / elapsed)
}

func (m *Manager) setResourceReady(kind ResourceKind, resource ResourceDescriptor, message string) {
	m.setResourceProgress(kind, resource, ResourcePhaseReady, resource.SizeBytes, resource.SizeBytes, 0, true, message)
}

func (m *Manager) setResourceError(kind ResourceKind, resource ResourceDescriptor, err error) {
	message := "资源准备失败"
	if err != nil {
		message = err.Error()
	}
	m.setResourceProgress(kind, resource, ResourcePhaseError, 0, resource.SizeBytes, 0, false, message)
}

func (m *Manager) setResourceProgress(kind ResourceKind, resource ResourceDescriptor, phase ResourcePhase, downloaded int64, total int64, bytesPerSecond int64, ready bool, message string) {
	if total <= 0 {
		total = resource.SizeBytes
	}
	if ready && downloaded <= 0 {
		downloaded = total
	}
	percent := 0
	if total > 0 && downloaded > 0 {
		percent = int(downloaded * 100 / total)
		if percent > 100 {
			percent = 100
		}
	}
	m.progressMu.Lock()
	defer m.progressMu.Unlock()
	m.progress[kind] = ResourceProgress{
		Kind:           string(kind),
		Artifact:       resource.Artifact,
		Version:        resource.Version,
		Phase:          phase,
		Ready:          ready,
		Downloaded:     downloaded,
		Total:          total,
		Percent:        percent,
		BytesPerSecond: bytesPerSecond,
		Message:        message,
		UpdatedAt:      m.now().UTC().Format(time.RFC3339),
	}
}

func resourceCheckingMessage(kind ResourceKind) string {
	return resourceLabel(kind) + "检查中"
}

func resourceDownloadingMessage(kind ResourceKind) string {
	return "正在下载" + resourceLabel(kind)
}

func resourceVerifyingMessage(kind ResourceKind) string {
	return resourceLabel(kind) + "校验中"
}

func resourceExtractingMessage(kind ResourceKind) string {
	return resourceLabel(kind) + "解压中"
}

func resourceCachedMessage(kind ResourceKind) string {
	return resourceLabel(kind) + "下载包已缓存"
}

func resourceReadyMessage(kind ResourceKind) string {
	return resourceLabel(kind) + "已就绪"
}

func resourceLabel(kind ResourceKind) string {
	switch kind {
	case ResourcePlatformTools:
		return "安装工具"
	case ResourceWatchAPK:
		return "手表安装包"
	case ResourceCloudflared:
		return "托管隧道组件"
	default:
		return "运行时资源"
	}
}

func resolveChannelManifestURL(appRoot string) string {
	if envValue := strings.TrimSpace(os.Getenv("OPENWATCHER_RUNTIME_CHANNEL_MANIFEST_URL")); envValue != "" {
		return envValue
	}
	for _, root := range settings.BundledResourceRoots(appRoot) {
		for _, relative := range []string{
			filepath.Join("runtime", "channel-manifest-url.txt"),
			filepath.Join("runtime", "manifest-url.txt"),
		} {
			data, err := os.ReadFile(filepath.Join(root, relative))
			if err == nil {
				if value := strings.TrimSpace(string(data)); value != "" {
					return value
				}
			}
		}
	}
	return defaultChannelManifestURL
}

func ensureDesktopVersion(required string, current string) error {
	required = strings.TrimSpace(required)
	if required == "" || current == "" {
		return nil
	}
	if compareVersion(current, required) < 0 {
		return fmt.Errorf("当前 Desktop 版本过旧，需要至少 %s 才能使用当前运行时资源", required)
	}
	return nil
}

func compareVersion(left string, right string) int {
	leftParts := versionParts(left)
	rightParts := versionParts(right)
	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}
	for i := 0; i < maxLen; i++ {
		leftValue := 0
		rightValue := 0
		if i < len(leftParts) {
			leftValue = leftParts[i]
		}
		if i < len(rightParts) {
			rightValue = rightParts[i]
		}
		switch {
		case leftValue < rightValue:
			return -1
		case leftValue > rightValue:
			return 1
		}
	}
	return 0
}

func versionParts(raw string) []int {
	trimmed := strings.TrimPrefix(strings.TrimSpace(raw), "v")
	chunks := strings.Split(trimmed, ".")
	parts := make([]int, 0, len(chunks))
	for _, chunk := range chunks {
		value := 0
		for _, r := range chunk {
			if r < '0' || r > '9' {
				break
			}
			value = value*10 + int(r-'0')
		}
		parts = append(parts, value)
	}
	return parts
}

func archiveStripPrefix(binRelativePath string) string {
	cleaned := path.Clean(strings.TrimSpace(binRelativePath))
	if cleaned == "." || !strings.Contains(cleaned, "/") {
		return ""
	}
	return strings.Split(cleaned, "/")[0]
}

func resourceFilesReady(root string, resource ResourceDescriptor) bool {
	required := []string{resource.BinRelativePath}
	required = append(required, resource.ExtraFiles...)
	for _, relative := range required {
		relative = strings.TrimSpace(relative)
		if relative == "" {
			continue
		}
		normalized := stripArchiveRoot(relative, archiveStripPrefix(resource.BinRelativePath))
		path := filepath.Join(root, filepath.FromSlash(normalized))
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

func stripArchiveRoot(relative string, root string) string {
	cleaned := path.Clean(strings.TrimSpace(relative))
	if root == "" {
		return cleaned
	}
	prefix := root + "/"
	return strings.TrimPrefix(cleaned, prefix)
}

func stateMatches(statePath string, kind ResourceKind, platform string, resource ResourceDescriptor) bool {
	data, err := os.ReadFile(statePath)
	if err != nil {
		return false
	}
	var state resourceState
	if err := json.Unmarshal(data, &state); err != nil {
		return false
	}
	return state.Kind == string(kind) &&
		state.Platform == platform &&
		state.Artifact == resource.Artifact &&
		state.SHA256 == resource.SHA256 &&
		state.URL == resource.URL &&
		state.Version == resource.Version &&
		state.VersionName == resource.VersionName &&
		state.VersionCode == resource.VersionCode &&
		state.BinRelativePath == resource.BinRelativePath
}

func writeResourceState(statePath string, kind ResourceKind, platform string, resource ResourceDescriptor, now time.Time) error {
	state := resourceState{
		Kind:            string(kind),
		Platform:        platform,
		Artifact:        resource.Artifact,
		SHA256:          resource.SHA256,
		URL:             resource.URL,
		Version:         resource.Version,
		VersionName:     resource.VersionName,
		VersionCode:     resource.VersionCode,
		ArchiveKind:     resource.ArchiveKind,
		BinRelativePath: resource.BinRelativePath,
		ExtraFiles:      append([]string(nil), resource.ExtraFiles...),
		UpdatedAt:       now.UTC().Format(time.RFC3339),
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomically(statePath, payload, 0o644)
}

func writeFileAtomically(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func fileMatchesSHA256(path string, expected string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return false, err
	}
	return strings.EqualFold(hex.EncodeToString(sum.Sum(nil)), strings.TrimSpace(expected)), nil
}

func copyFile(sourcePath string, targetPath string, mode os.FileMode) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return err
	}
	if err := target.Close(); err != nil {
		return err
	}
	return os.Chmod(targetPath, mode)
}

func extractZip(sourcePath string, targetDir string, stripRoot string) error {
	reader, err := zip.OpenReader(sourcePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, entry := range reader.File {
		relative, skip := archiveEntryPath(entry.Name, stripRoot)
		if skip {
			continue
		}
		targetPath := filepath.Join(targetDir, filepath.FromSlash(relative))
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		target, err := os.Create(targetPath)
		if err != nil {
			source.Close()
			return err
		}
		if _, err := io.Copy(target, source); err != nil {
			target.Close()
			source.Close()
			return err
		}
		source.Close()
		if err := target.Close(); err != nil {
			return err
		}
		if entry.Mode()&0o111 != 0 {
			_ = os.Chmod(targetPath, 0o755)
		}
	}
	return nil
}

func extractTGZ(sourcePath string, targetDir string, stripRoot string) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		relative, skip := archiveEntryPath(header.Name, stripRoot)
		if skip {
			continue
		}
		targetPath := filepath.Join(targetDir, filepath.FromSlash(relative))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			target, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(target, reader); err != nil {
				target.Close()
				return err
			}
			if err := target.Close(); err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if header.FileInfo().Mode()&0o111 != 0 {
				mode = 0o755
			}
			_ = os.Chmod(targetPath, mode)
		}
	}
}

func archiveEntryPath(name string, stripRoot string) (string, bool) {
	cleaned := path.Clean(strings.TrimSpace(name))
	if cleaned == "." || cleaned == "/" || path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", true
	}
	if stripRoot != "" {
		prefix := stripRoot + "/"
		if cleaned == stripRoot {
			return "", true
		}
		if !strings.HasPrefix(cleaned, prefix) {
			return "", true
		}
		cleaned = strings.TrimPrefix(cleaned, prefix)
	}
	if cleaned == "." || cleaned == "" || path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", true
	}
	return cleaned, false
}
