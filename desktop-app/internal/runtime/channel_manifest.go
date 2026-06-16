package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const ChannelManifestSchemaVersion = 1

type ChannelManifest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Channel       string                 `json:"channel"`
	Revision      int                    `json:"revision,omitempty"`
	UpdatedAt     string                 `json:"updatedAt,omitempty"`
	Source        ChannelManifestSource  `json:"source,omitempty"`
	Release       ChannelManifestRelease `json:"release,omitempty"`
	Desktop       ChannelManifestDesktop `json:"desktop,omitempty"`
	Watch         ChannelManifestWatch   `json:"watch,omitempty"`
	Runtime       ChannelManifestRuntime `json:"runtime"`
}

type ChannelManifestRuntime struct {
	ManifestURL    string `json:"manifestUrl"`
	ManifestSHA256 string `json:"manifestSha256"`
}

type ChannelManifestSource struct {
	Repository string `json:"repository,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Commit     string `json:"commit,omitempty"`
	ReleaseTag string `json:"releaseTag,omitempty"`
	ReleaseURL string `json:"releaseUrl,omitempty"`
}

type ChannelManifestRelease struct {
	Tag         string   `json:"tag,omitempty"`
	Scope       []string `json:"scope,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	ReleaseURL  string   `json:"releaseUrl,omitempty"`
	ManifestURL string   `json:"manifestUrl,omitempty"`
	NotesURL    string   `json:"notesUrl,omitempty"`
}

type ChannelManifestDesktop struct {
	Version   string                                  `json:"version,omitempty"`
	Platforms map[string]ChannelManifestDesktopTarget `json:"platforms,omitempty"`
}

type ChannelManifestDesktopTarget struct {
	Artifact    string `json:"artifact,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	SizeBytes   int64  `json:"sizeBytes,omitempty"`
}

type ChannelManifestWatch struct {
	VersionName  string `json:"versionName,omitempty"`
	VersionCode  int    `json:"versionCode,omitempty"`
	Artifact     string `json:"artifact,omitempty"`
	DownloadURL  string `json:"downloadUrl,omitempty"`
	SHA256       string `json:"sha256,omitempty"`
	ChangelogURL string `json:"changelogUrl,omitempty"`
}

func (m ChannelManifest) Validate() error {
	if m.SchemaVersion != ChannelManifestSchemaVersion {
		return fmt.Errorf("不支持的 channel manifest schemaVersion：%d", m.SchemaVersion)
	}
	if strings.TrimSpace(m.Channel) == "" {
		return fmt.Errorf("channel manifest 缺少 channel")
	}
	if strings.TrimSpace(m.Runtime.ManifestURL) == "" {
		return fmt.Errorf("channel manifest 缺少 runtime.manifestUrl")
	}
	if strings.TrimSpace(m.Runtime.ManifestSHA256) == "" {
		return fmt.Errorf("channel manifest 缺少 runtime.manifestSha256")
	}
	return nil
}

func (m *Manager) fetchChannelManifest(ctx context.Context) (ChannelManifest, error) {
	url := strings.TrimSpace(m.channelManifestURL)
	if url == "" {
		return ChannelManifest{}, fmt.Errorf("未配置 channel manifest 地址")
	}
	payload, err := m.fetchJSON(ctx, url, "channel manifest")
	if err != nil {
		return ChannelManifest{}, err
	}
	var manifest ChannelManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return ChannelManifest{}, fmt.Errorf("解析 channel manifest 失败：%w", err)
	}
	if err := manifest.Validate(); err != nil {
		return ChannelManifest{}, err
	}
	return manifest, nil
}

func (m *Manager) fetchRuntimeManifest(ctx context.Context, url string, expectedSHA256 string) (Manifest, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return Manifest{}, fmt.Errorf("未配置 runtime manifest 地址")
	}
	payload, err := m.fetchJSON(ctx, url, "runtime manifest")
	if err != nil {
		return Manifest{}, err
	}
	sum := sha256.Sum256(payload)
	actualSHA256 := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actualSHA256, strings.TrimSpace(expectedSHA256)) {
		return Manifest{}, fmt.Errorf("runtime manifest sha256 校验失败")
	}
	var manifest Manifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("解析 runtime manifest 失败：%w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (m *Manager) fetchJSON(ctx context.Context, url string, label string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("构造%s请求失败", label)
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载%s失败：%w", label, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载%s失败：HTTP %d", label, resp.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("读取%s失败", label)
	}
	return payload, nil
}
