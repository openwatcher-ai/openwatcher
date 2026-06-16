package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"openwatcher/desktop-app/internal/settings"
)

const ManifestSchemaVersion = 1

type ResourceKind string

const (
	ResourceWatchAPK      ResourceKind = "watchApk"
	ResourcePlatformTools ResourceKind = "platformTools"
	ResourceCloudflared   ResourceKind = "cloudflared"
)

type Manifest struct {
	SchemaVersion     int               `json:"schemaVersion"`
	Channel           string            `json:"channel"`
	GeneratedAt       string            `json:"generatedAt"`
	ReleaseSlug       string            `json:"releaseSlug"`
	DesktopMinVersion string            `json:"desktopMinVersion"`
	SourceCommit      string            `json:"sourceCommit,omitempty"`
	Resources         ManifestResources `json:"resources"`
	Notes             string            `json:"notes"`
}

type ManifestResources struct {
	WatchAPK      ResourceDescriptor            `json:"watchApk"`
	PlatformTools map[string]ResourceDescriptor `json:"platformTools"`
	Cloudflared   map[string]ResourceDescriptor `json:"cloudflared"`
}

type ResourceDescriptor struct {
	Version         string   `json:"version"`
	VersionName     string   `json:"versionName,omitempty"`
	VersionCode     int      `json:"versionCode,omitempty"`
	Artifact        string   `json:"artifact"`
	URL             string   `json:"url"`
	SHA256          string   `json:"sha256"`
	SizeBytes       int64    `json:"sizeBytes"`
	ArchiveKind     string   `json:"archiveKind,omitempty"`
	BinRelativePath string   `json:"binRelativePath,omitempty"`
	ExtraFiles      []string `json:"extraFiles,omitempty"`
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != ManifestSchemaVersion {
		return fmt.Errorf("不支持的 runtime manifest schemaVersion：%d", m.SchemaVersion)
	}
	if strings.TrimSpace(m.Channel) == "" {
		return fmt.Errorf("runtime manifest 缺少 channel")
	}
	if strings.TrimSpace(m.GeneratedAt) == "" {
		return fmt.Errorf("runtime manifest 缺少 generatedAt")
	}
	if strings.TrimSpace(m.ReleaseSlug) == "" {
		return fmt.Errorf("runtime manifest 缺少 releaseSlug")
	}
	if err := validateResource(m.Resources.WatchAPK, ResourceWatchAPK); err != nil {
		return err
	}
	for platform, resource := range m.Resources.PlatformTools {
		if err := validateResource(resource, ResourcePlatformTools); err != nil {
			return fmt.Errorf("%s platformTools: %w", platform, err)
		}
	}
	for platform, resource := range m.Resources.Cloudflared {
		if err := validateResource(resource, ResourceCloudflared); err != nil {
			return fmt.Errorf("%s cloudflared: %w", platform, err)
		}
	}
	return nil
}

func validateResource(resource ResourceDescriptor, kind ResourceKind) error {
	if strings.TrimSpace(resource.Version) == "" {
		return fmt.Errorf("%s 缺少 version", kind)
	}
	if strings.TrimSpace(resource.Artifact) == "" {
		return fmt.Errorf("%s 缺少 artifact", kind)
	}
	if strings.TrimSpace(resource.URL) == "" {
		return fmt.Errorf("%s 缺少 url", kind)
	}
	if strings.TrimSpace(resource.SHA256) == "" {
		return fmt.Errorf("%s 缺少 sha256", kind)
	}
	if resource.SizeBytes <= 0 {
		return fmt.Errorf("%s 缺少 sizeBytes", kind)
	}
	switch kind {
	case ResourcePlatformTools, ResourceCloudflared:
		if strings.TrimSpace(resource.ArchiveKind) == "" {
			return fmt.Errorf("%s 缺少 archiveKind", kind)
		}
		if strings.TrimSpace(resource.BinRelativePath) == "" {
			return fmt.Errorf("%s 缺少 binRelativePath", kind)
		}
	}
	return nil
}

func (m Manifest) Resource(kind ResourceKind, platform string) (ResourceDescriptor, error) {
	switch kind {
	case ResourceWatchAPK:
		return m.Resources.WatchAPK, nil
	case ResourcePlatformTools:
		resource, ok := m.Resources.PlatformTools[platform]
		if !ok {
			return ResourceDescriptor{}, fmt.Errorf("manifest 缺少平台 %s 的 platform-tools 资源", platform)
		}
		return resource, nil
	case ResourceCloudflared:
		resource, ok := m.Resources.Cloudflared[platform]
		if !ok {
			return ResourceDescriptor{}, fmt.Errorf("manifest 缺少平台 %s 的 cloudflared 资源", platform)
		}
		return resource, nil
	default:
		return ResourceDescriptor{}, fmt.Errorf("未知 runtime 资源类型：%s", kind)
	}
}

func LoadCachedManifest() (Manifest, error) {
	runtimeDir, err := settings.RuntimeDir()
	if err != nil {
		return Manifest{}, err
	}
	return loadManifestFromPath(filepath.Join(runtimeDir, "manifests", "current.json"))
}

func loadManifestFromPath(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("解析 runtime manifest 失败：%w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
