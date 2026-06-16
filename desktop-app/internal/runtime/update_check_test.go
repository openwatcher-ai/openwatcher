package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagerCheckForUpdates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/changelog/beta-2026.06.13.1.json" {
			payload := manifestJSON(t, map[string]any{
				"schemaVersion": 1,
				"channel":       "beta",
				"id":            "beta-2026.06.13.1",
				"publishedAt":   "2026-06-13T00:00:00Z",
				"components": map[string]any{
					"desktop": map[string]any{"status": "updated"},
					"runtime": map[string]any{"status": "reused"},
				},
				"notes": map[string]any{
					"fixes": []map[string]any{
						{"component": "桌面应用", "text": "修复桌面更新检查与交互反馈"},
					},
				},
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(payload)
			return
		}
		if r.URL.Path != "/channels/beta.json" {
			http.NotFound(w, r)
			return
		}
		payload := manifestJSON(t, map[string]any{
			"schemaVersion": 1,
			"channel":       "beta",
			"updatedAt":     "2026-06-13T00:00:00Z",
			"release": map[string]any{
				"tag":        "beta-2026.06.13.1",
				"summary":    "修复桌面更新检查与交互反馈",
				"releaseUrl": "https://github.com/openwatcher-ai/openwatcher/releases/tag/beta-2026.06.13.1",
				"notesUrl":   serverURL(r, "/changelog/beta-2026.06.13.1.json"),
			},
			"desktop": map[string]any{
				"version": "0.2.1",
				"platforms": map[string]any{
					"darwin-arm64": map[string]any{
						"artifact":    "OpenWatcher-Desktop-darwin-arm64.zip",
						"downloadUrl": "https://example.com/OpenWatcher-Desktop-darwin-arm64.zip",
						"sha256":      "fixture-sha",
						"sizeBytes":   12345,
					},
				},
			},
			"watch": map[string]any{
				"versionName": "1.0.1",
				"downloadUrl": "https://example.com/openwatcher-watchapp.apk",
			},
			"runtime": map[string]any{
				"manifestUrl":    "https://example.com/runtime-manifest.json",
				"manifestSha256": "fixture-sha",
			},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	manager := NewManager(t.TempDir(), "0.2.0")
	manager.platform = "darwin-arm64"
	manager.channelManifestURL = server.URL + "/channels/beta.json"
	manager.httpClient = server.Client()

	got, err := manager.CheckForUpdates(context.Background(), "1.0.0")
	if err != nil {
		t.Fatalf("CheckForUpdates err = %v", err)
	}
	if !got.DesktopUpdateAvailable {
		t.Fatalf("DesktopUpdateAvailable = false, want true")
	}
	if !got.WatchUpdateAvailable {
		t.Fatalf("WatchUpdateAvailable = false, want true")
	}
	if got.LatestDesktopVersion != "0.2.1" {
		t.Fatalf("LatestDesktopVersion = %q", got.LatestDesktopVersion)
	}
	if got.DesktopDownloadURL != "https://example.com/OpenWatcher-Desktop-darwin-arm64.zip" {
		t.Fatalf("DesktopDownloadURL = %q", got.DesktopDownloadURL)
	}
	if !got.DesktopInstallable || got.DesktopArchiveKind != "zip" || got.DesktopSizeBytes != 12345 {
		t.Fatalf("desktop installable fields = %+v", got)
	}
	if got.ReleaseSummary != "修复桌面更新检查与交互反馈" {
		t.Fatalf("ReleaseSummary = %q", got.ReleaseSummary)
	}
	if len(got.ReleaseNotes) != 1 || got.ReleaseNotes[0].Component != "桌面应用" {
		t.Fatalf("ReleaseNotes = %+v", got.ReleaseNotes)
	}
}
