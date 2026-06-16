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
		return err
	}
	switch kind {
	case ResourceWatchAPK:
		return m.ensureWatchAPK(ctx, resource)
	case ResourcePlatformTools:
		return m.ensureArchiveResource(ctx, kind, resource, filepath.Join("platform-tools", m.platform))
	case ResourceCloudflared:
		return m.ensureArchiveResource(ctx, kind, resource, filepath.Join("cloudflared", m.platform))
	default:
		return fmt.Errorf("未知 runtime 资源类型：%s", kind)
	}
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
			return nil
		}
	}

	downloadPath, err := m.ensureDownload(ctx, resource)
	if err != nil {
		return err
	}
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
	return writeResourceState(statePath, ResourceWatchAPK, m.platform, resource, m.now())
}

func (m *Manager) ensureArchiveResource(ctx context.Context, kind ResourceKind, resource ResourceDescriptor, relativeTarget string) error {
	runtimeRoot, err := m.RuntimeRoot()
	if err != nil {
		return err
	}
	targetDir := filepath.Join(runtimeRoot, filepath.FromSlash(relativeTarget))
	statePath := filepath.Join(targetDir, ".resource.json")
	if stateMatches(statePath, kind, m.platform, resource) && resourceFilesReady(targetDir, resource) {
		return nil
	}

	downloadPath, err := m.ensureDownload(ctx, resource)
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
		if err := extractZip(downloadPath, tempDir, archiveStripPrefix(resource.BinRelativePath)); err != nil {
			return err
		}
	case "tgz":
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
	return nil
}

func (m *Manager) ensureDownload(ctx context.Context, resource ResourceDescriptor) (string, error) {
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
	if _, err := io.Copy(file, resp.Body); err != nil {
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
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return targetPath, nil
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
