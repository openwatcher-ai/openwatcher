package runtime

import (
	"context"
	"strings"
	"time"
)

type UpdateCheckResult struct {
	Channel                string              `json:"channel"`
	CheckedAt              string              `json:"checkedAt"`
	ReleaseTag             string              `json:"releaseTag,omitempty"`
	ReleaseSummary         string              `json:"releaseSummary,omitempty"`
	ReleaseURL             string              `json:"releaseUrl,omitempty"`
	NotesURL               string              `json:"notesUrl,omitempty"`
	CurrentDesktopVersion  string              `json:"currentDesktopVersion"`
	LatestDesktopVersion   string              `json:"latestDesktopVersion,omitempty"`
	DesktopUpdateAvailable bool                `json:"desktopUpdateAvailable"`
	DesktopDownloadURL     string              `json:"desktopDownloadUrl,omitempty"`
	DesktopArtifact        string              `json:"desktopArtifact,omitempty"`
	DesktopSHA256          string              `json:"desktopSha256,omitempty"`
	DesktopSizeBytes       int64               `json:"desktopSizeBytes,omitempty"`
	DesktopArchiveKind     string              `json:"desktopArchiveKind,omitempty"`
	DesktopInstallable     bool                `json:"desktopInstallable"`
	DesktopInstallMessage  string              `json:"desktopInstallMessage,omitempty"`
	CurrentWatchVersion    string              `json:"currentWatchVersion,omitempty"`
	LatestWatchVersion     string              `json:"latestWatchVersion,omitempty"`
	WatchUpdateAvailable   bool                `json:"watchUpdateAvailable"`
	WatchDownloadURL       string              `json:"watchDownloadUrl,omitempty"`
	ReleaseNotes           []ChangelogNoteItem `json:"releaseNotes,omitempty"`
}

func (m *Manager) CheckForUpdates(ctx context.Context, currentWatchVersion string) (UpdateCheckResult, error) {
	channelManifest, err := m.fetchChannelManifest(ctx)
	if err != nil {
		return UpdateCheckResult{}, err
	}

	result := UpdateCheckResult{
		Channel:               strings.TrimSpace(channelManifest.Channel),
		CheckedAt:             m.now().UTC().Format(time.RFC3339),
		ReleaseTag:            strings.TrimSpace(channelManifest.Release.Tag),
		ReleaseSummary:        strings.TrimSpace(channelManifest.Release.Summary),
		ReleaseURL:            strings.TrimSpace(channelManifest.Release.ReleaseURL),
		NotesURL:              strings.TrimSpace(channelManifest.Release.NotesURL),
		CurrentDesktopVersion: strings.TrimSpace(m.desktopVersion),
		LatestDesktopVersion:  strings.TrimSpace(channelManifest.Desktop.Version),
		CurrentWatchVersion:   strings.TrimSpace(currentWatchVersion),
		LatestWatchVersion:    strings.TrimSpace(channelManifest.Watch.VersionName),
		WatchDownloadURL:      strings.TrimSpace(channelManifest.Watch.DownloadURL),
	}

	if result.CurrentDesktopVersion != "" && result.LatestDesktopVersion != "" {
		result.DesktopUpdateAvailable = compareVersion(result.CurrentDesktopVersion, result.LatestDesktopVersion) < 0
	}
	if result.CurrentWatchVersion != "" && result.LatestWatchVersion != "" {
		result.WatchUpdateAvailable = compareVersion(result.CurrentWatchVersion, result.LatestWatchVersion) < 0
	}

	if target, ok := channelManifest.Desktop.Platforms[m.platform]; ok {
		result.DesktopArtifact = strings.TrimSpace(target.Artifact)
		result.DesktopDownloadURL = strings.TrimSpace(target.DownloadURL)
		result.DesktopSHA256 = strings.TrimSpace(target.SHA256)
		result.DesktopSizeBytes = target.SizeBytes
		result.DesktopArchiveKind = desktopArchiveKind(result.DesktopArtifact)
		result.DesktopInstallable = desktopTargetInstallable(result)
		if !result.DesktopInstallable && result.DesktopUpdateAvailable {
			result.DesktopInstallMessage = "当前通道未提供可自动安装的 Desktop zip 包"
		}
	} else if result.DesktopUpdateAvailable {
		result.DesktopInstallMessage = "当前通道缺少本机平台的 Desktop 更新包"
	}

	if result.NotesURL != "" {
		notes, _ := m.fetchDesktopReleaseNotes(ctx, result.NotesURL)
		result.ReleaseNotes = notes
	}

	return result, nil
}

func desktopArchiveKind(artifact string) string {
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(artifact)), ".zip") {
		return "zip"
	}
	return ""
}

func desktopTargetInstallable(result UpdateCheckResult) bool {
	return result.DesktopUpdateAvailable &&
		result.DesktopArchiveKind == "zip" &&
		strings.TrimSpace(result.DesktopDownloadURL) != "" &&
		strings.TrimSpace(result.DesktopSHA256) != "" &&
		result.DesktopSizeBytes > 0
}
