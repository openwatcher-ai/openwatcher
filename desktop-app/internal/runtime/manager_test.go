package runtime

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagerEnsureAll(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)

	assetsDir := filepath.Join(configRoot, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}

	platformToolsPath := filepath.Join(assetsDir, "platform-tools-windows-amd64.zip")
	writeZipFile(t, platformToolsPath, map[string]string{
		"platform-tools/adb.exe":          "adb-binary",
		"platform-tools/AdbWinApi.dll":    "dll-a",
		"platform-tools/AdbWinUsbApi.dll": "dll-b",
	})
	cloudflaredPath := filepath.Join(assetsDir, "cloudflared-windows-amd64.tgz")
	writeTGZFile(t, cloudflaredPath, map[string]string{
		"cloudflared": "cloudflared-binary",
	})
	watchAPKPath := filepath.Join(assetsDir, "openwatcher-watchapp.apk")
	if err := os.WriteFile(watchAPKPath, []byte("watch-apk"), 0o644); err != nil {
		t.Fatalf("write apk: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/channels/beta.json":
			runtimePayload := runtimePayloadForTest(t, serverURL(r, ""), watchAPKPath, platformToolsPath, cloudflaredPath)
			payload := manifestJSON(t, map[string]any{
				"schemaVersion": 1,
				"channel":       "beta",
				"runtime": map[string]any{
					"manifestUrl":    serverURL(r, "/runtime-manifest.json"),
					"manifestSha256": payloadSHA256(runtimePayload),
				},
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(payload)
		case "/runtime-manifest.json":
			payload := runtimePayloadForTest(t, serverURL(r, ""), watchAPKPath, platformToolsPath, cloudflaredPath)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(payload)
		case "/" + filepath.Base(platformToolsPath):
			http.ServeFile(w, r, platformToolsPath)
		case "/" + filepath.Base(cloudflaredPath):
			http.ServeFile(w, r, cloudflaredPath)
		case "/" + filepath.Base(watchAPKPath):
			http.ServeFile(w, r, watchAPKPath)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager := NewManager(t.TempDir(), "0.1.0")
	manager.platform = "windows-amd64"
	manager.channelManifestURL = server.URL + "/channels/beta.json"
	manager.httpClient = server.Client()
	manager.fetchTTL = time.Hour

	if err := manager.EnsureAll(context.Background()); err != nil {
		t.Fatalf("EnsureAll err = %v", err)
	}

	runtimeRoot, err := manager.RuntimeRoot()
	if err != nil {
		t.Fatalf("RuntimeRoot err = %v", err)
	}
	for _, path := range []string{
		filepath.Join(runtimeRoot, "platform-tools", "windows-amd64", "adb.exe"),
		filepath.Join(runtimeRoot, "platform-tools", "windows-amd64", "AdbWinApi.dll"),
		filepath.Join(runtimeRoot, "cloudflared", "windows-amd64", "cloudflared"),
		filepath.Join(runtimeRoot, "watch-apk", "openwatcher-watch-release.apk"),
		filepath.Join(runtimeRoot, "manifests", "current.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}

	manifest, err := LoadCachedManifest()
	if err != nil {
		t.Fatalf("LoadCachedManifest err = %v", err)
	}
	if manifest.Resources.WatchAPK.VersionName != "0.19.3" {
		t.Fatalf("unexpected cached manifest watch version: %+v", manifest.Resources.WatchAPK)
	}

	status := manager.Status()
	for _, kind := range []ResourceKind{ResourcePlatformTools, ResourceWatchAPK, ResourceCloudflared} {
		progress, ok := status.Resources[string(kind)]
		if !ok {
			t.Fatalf("missing runtime progress for %s: %+v", kind, status.Resources)
		}
		if !progress.Ready || progress.Phase != ResourcePhaseReady {
			t.Fatalf("progress for %s = %+v, want ready", kind, progress)
		}
	}
}

func TestEnsureUsesCachedManifestWhenRemoteUnavailable(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)

	manager := NewManager(t.TempDir(), "0.1.0")
	manager.platform = "windows-amd64"
	manager.channelManifestURL = "http://127.0.0.1.invalid/channels/beta.json"
	manager.httpClient = &http.Client{Timeout: 200 * time.Millisecond}

	runtimeRoot, err := manager.RuntimeRoot()
	if err != nil {
		t.Fatalf("RuntimeRoot err = %v", err)
	}
	manifestDir := filepath.Join(runtimeRoot, "manifests")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, "current.json"), manifestJSON(t, map[string]any{
		"schemaVersion":     1,
		"channel":           "beta",
		"generatedAt":       "2026-06-09T12:00:00Z",
		"releaseSlug":       "cached",
		"desktopMinVersion": "0.1.0",
		"resources": map[string]any{
			"watchApk": map[string]any{
				"version":     "0.19.3+72",
				"versionName": "0.19.3",
				"versionCode": 72,
				"artifact":    "cached.apk",
				"url":         "https://example.invalid/cached.apk",
				"sha256":      "abc",
				"sizeBytes":   1,
			},
			"platformTools": map[string]any{
				"windows-amd64": map[string]any{
					"version":         "35.0.2",
					"artifact":        "cached.zip",
					"url":             "https://example.invalid/cached.zip",
					"sha256":          "abc",
					"sizeBytes":       1,
					"archiveKind":     "zip",
					"binRelativePath": "platform-tools/adb.exe",
					"extraFiles": []string{
						"platform-tools/AdbWinApi.dll",
						"platform-tools/AdbWinUsbApi.dll",
					},
				},
			},
			"cloudflared": map[string]any{
				"windows-amd64": map[string]any{
					"version":         "2026.6.0",
					"artifact":        "cached.tgz",
					"url":             "https://example.invalid/cached.tgz",
					"sha256":          "abc",
					"sizeBytes":       1,
					"archiveKind":     "tgz",
					"binRelativePath": "cloudflared",
				},
			},
		},
		"notes": "cached",
	}), 0o644); err != nil {
		t.Fatalf("write cached manifest: %v", err)
	}

	manifest, err := manager.currentManifest(context.Background())
	if err != nil {
		t.Fatalf("currentManifest err = %v", err)
	}
	if manifest.ReleaseSlug != "cached" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
}

func TestCurrentManifestReturnsErrorWhenRuntimeManifestSHA256Mismatch(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/channels/beta.json":
			payload := manifestJSON(t, map[string]any{
				"schemaVersion": 1,
				"channel":       "beta",
				"runtime": map[string]any{
					"manifestUrl":    serverURL(r, "/runtime-manifest.json"),
					"manifestSha256": strings.Repeat("0", 64),
				},
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(payload)
		case "/runtime-manifest.json":
			payload := manifestJSON(t, map[string]any{
				"schemaVersion":     1,
				"channel":           "beta",
				"generatedAt":       "2026-06-09T12:00:00Z",
				"releaseSlug":       "preview",
				"desktopMinVersion": "0.1.0",
				"resources": map[string]any{
					"watchApk": map[string]any{
						"version":     "0.19.3+72",
						"versionName": "0.19.3",
						"versionCode": 72,
						"artifact":    "watch.apk",
						"url":         "https://example.invalid/watch.apk",
						"sha256":      "abc",
						"sizeBytes":   1,
					},
					"platformTools": map[string]any{
						"windows-amd64": map[string]any{
							"version":         "35.0.2",
							"artifact":        "platform-tools.zip",
							"url":             "https://example.invalid/platform-tools.zip",
							"sha256":          "abc",
							"sizeBytes":       1,
							"archiveKind":     "zip",
							"binRelativePath": "platform-tools/adb.exe",
							"extraFiles": []string{
								"platform-tools/AdbWinApi.dll",
								"platform-tools/AdbWinUsbApi.dll",
							},
						},
					},
					"cloudflared": map[string]any{
						"windows-amd64": map[string]any{
							"version":         "2026.6.0",
							"artifact":        "cloudflared.tgz",
							"url":             "https://example.invalid/cloudflared.tgz",
							"sha256":          "abc",
							"sizeBytes":       1,
							"archiveKind":     "tgz",
							"binRelativePath": "cloudflared",
						},
					},
				},
				"notes": "beta runtime",
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager := NewManager(t.TempDir(), "0.1.0")
	manager.platform = "windows-amd64"
	manager.channelManifestURL = server.URL + "/channels/beta.json"
	manager.httpClient = server.Client()
	manager.fetchTTL = time.Hour

	_, err := manager.currentManifest(context.Background())
	if err == nil {
		t.Fatal("currentManifest err = nil, want sha256 mismatch")
	}
	if got := err.Error(); got != "runtime manifest sha256 校验失败" {
		t.Fatalf("currentManifest err = %q", got)
	}
}

func runtimePayloadForTest(t *testing.T, serverBaseURL string, watchAPKPath string, platformToolsPath string, cloudflaredPath string) []byte {
	t.Helper()
	return manifestJSON(t, map[string]any{
		"schemaVersion":     1,
		"channel":           "beta",
		"generatedAt":       "2026-06-09T12:00:00Z",
		"releaseSlug":       "preview",
		"desktopMinVersion": "0.1.0",
		"resources": map[string]any{
			"watchApk": map[string]any{
				"version":     "0.19.3+72",
				"versionName": "0.19.3",
				"versionCode": 72,
				"artifact":    filepath.Base(watchAPKPath),
				"url":         serverBaseURL + "/" + filepath.Base(watchAPKPath),
				"sha256":      fileSHA256(t, watchAPKPath),
				"sizeBytes":   fileSize(t, watchAPKPath),
			},
			"platformTools": map[string]any{
				"windows-amd64": map[string]any{
					"version":         "35.0.2",
					"artifact":        filepath.Base(platformToolsPath),
					"url":             serverBaseURL + "/" + filepath.Base(platformToolsPath),
					"sha256":          fileSHA256(t, platformToolsPath),
					"sizeBytes":       fileSize(t, platformToolsPath),
					"archiveKind":     "zip",
					"binRelativePath": "platform-tools/adb.exe",
					"extraFiles": []string{
						"platform-tools/AdbWinApi.dll",
						"platform-tools/AdbWinUsbApi.dll",
					},
				},
			},
			"cloudflared": map[string]any{
				"windows-amd64": map[string]any{
					"version":         "2026.6.0",
					"artifact":        filepath.Base(cloudflaredPath),
					"url":             serverBaseURL + "/" + filepath.Base(cloudflaredPath),
					"sha256":          fileSHA256(t, cloudflaredPath),
					"sizeBytes":       fileSize(t, cloudflaredPath),
					"archiveKind":     "tgz",
					"binRelativePath": "cloudflared",
				},
			},
		},
		"notes": "beta runtime",
	})
}

func writeZipFile(t *testing.T, target string, files map[string]string) {
	t.Helper()
	file, err := os.Create(target)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
}

func writeTGZFile(t *testing.T, target string, files map[string]string) {
	t.Helper()
	file, err := os.Create(target)
	if err != nil {
		t.Fatalf("create tgz: %v", err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range files {
		payload := []byte(content)
		header := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(payload)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write(payload); err != nil {
			t.Fatalf("write tar payload: %v", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close tgz file: %v", err)
	}
}

func manifestJSON(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return data
}

func payloadSHA256(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file for sha: %v", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file size: %v", err)
	}
	return info.Size()
}

func serverURL(r *http.Request, path string) string {
	return "http://" + r.Host + path
}
