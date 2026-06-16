package fakeruntime

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"openwatcher/testsupport/fakeadb"
)

type Runtime struct {
	AppRoot       string
	ManifestURL   string
	ChannelURL    string
	RuntimeRoot   string
	ADBBinaryPath string
	WatchAPKPath  string
	Server        *httptest.Server
}

func Start(t testing.TB, appRoot string) *Runtime {
	t.Helper()

	assetsDir := t.TempDir()
	adbArchive := filepath.Join(assetsDir, "platform-tools-"+platform()+".zip")
	writePlatformToolsZip(t, adbArchive)
	watchAPK := filepath.Join(assetsDir, "openwatcher-watch-release.apk")
	if err := os.WriteFile(watchAPK, []byte("fake release apk"), 0o644); err != nil {
		t.Fatalf("write fake watch apk: %v", err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/channels/beta.json":
			runtimeManifest := manifestPayload(t, server.URL, watchAPK, adbArchive)
			channelManifest := map[string]any{
				"schemaVersion": 1,
				"channel":       "beta",
				"runtime": map[string]any{
					"manifestUrl":    server.URL + "/runtime-manifest.json",
					"manifestSha256": payloadSHA256(t, runtimeManifest),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(channelManifest); err != nil {
				t.Fatalf("encode channel manifest: %v", err)
			}
		case "/runtime-manifest.json":
			manifest := manifestPayload(t, server.URL, watchAPK, adbArchive)
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write(manifest); err != nil {
				t.Fatalf("write runtime manifest: %v", err)
			}
		case "/manifest.json":
			manifest := manifestPayload(t, server.URL, watchAPK, adbArchive)
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write(manifest); err != nil {
				t.Fatalf("write legacy runtime manifest: %v", err)
			}
		case "/" + filepath.Base(adbArchive):
			http.ServeFile(w, r, adbArchive)
		case "/" + filepath.Base(watchAPK):
			http.ServeFile(w, r, watchAPK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("OPENWATCHER_RUNTIME_CHANNEL_MANIFEST_URL", server.URL+"/channels/beta.json")

	runtimeRoot := runtimeRootForConfig(t)
	return &Runtime{
		AppRoot:       appRoot,
		ManifestURL:   server.URL + "/runtime-manifest.json",
		ChannelURL:    server.URL + "/channels/beta.json",
		RuntimeRoot:   runtimeRoot,
		ADBBinaryPath: filepath.Join(runtimeRoot, "platform-tools", platform(), fakeadb.BinaryName()),
		WatchAPKPath:  filepath.Join(runtimeRoot, "watch-apk", "openwatcher-watch-release.apk"),
		Server:        server,
	}
}

func writePlatformToolsZip(t testing.TB, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir platform-tools zip dir: %v", err)
	}
	file, err := os.Create(target)
	if err != nil {
		t.Fatalf("create platform-tools zip: %v", err)
	}
	defer file.Close()
	archive := zip.NewWriter(file)
	defer archive.Close()

	header := &zip.FileHeader{
		Name:   filepath.ToSlash(filepath.Join("platform-tools", fakeadb.BinaryName())),
		Method: zip.Deflate,
	}
	header.SetMode(0o755)
	writer, err := archive.CreateHeader(header)
	if err != nil {
		t.Fatalf("create adb zip entry: %v", err)
	}
	source, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatalf("open test binary: %v", err)
	}
	defer source.Close()
	if _, err := io.Copy(writer, source); err != nil {
		t.Fatalf("copy test binary into adb zip entry: %v", err)
	}
}

func runtimeRootForConfig(t testing.TB) string {
	t.Helper()
	configRoot, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("resolve user config dir: %v", err)
	}
	switch runtime.GOOS {
	case "windows", "darwin":
		return filepath.Join(configRoot, "OpenWatcher", "runtime")
	default:
		return filepath.Join(configRoot, "openwatcher", "runtime")
	}
}

func platform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func manifestPayload(t testing.TB, baseURL string, watchAPK string, adbArchive string) []byte {
	t.Helper()
	manifest := map[string]any{
		"schemaVersion":     1,
		"channel":           "beta",
		"generatedAt":       "2026-06-10T00:00:00Z",
		"releaseSlug":       "headless-e2e",
		"desktopMinVersion": "0.1.0",
		"resources": map[string]any{
			"watchApk": map[string]any{
				"version":     "0.1.0+10000",
				"versionName": "0.1.0",
				"versionCode": 10000,
				"artifact":    filepath.Base(watchAPK),
				"url":         baseURL + "/" + filepath.Base(watchAPK),
				"sha256":      fileSHA256(t, watchAPK),
				"sizeBytes":   fileSize(t, watchAPK),
			},
			"platformTools": map[string]any{
				platform(): map[string]any{
					"version":         "fake-adb",
					"artifact":        filepath.Base(adbArchive),
					"url":             baseURL + "/" + filepath.Base(adbArchive),
					"sha256":          fileSHA256(t, adbArchive),
					"sizeBytes":       fileSize(t, adbArchive),
					"archiveKind":     "zip",
					"binRelativePath": filepath.ToSlash(filepath.Join("platform-tools", fakeadb.BinaryName())),
				},
			},
			"cloudflared": map[string]any{},
		},
		"notes": "fake runtime for Desktop headless E2E",
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal runtime manifest: %v", err)
	}
	return data
}

func payloadSHA256(t testing.TB, payload []byte) string {
	t.Helper()
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func fileSHA256(t testing.TB, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open for sha256: %v", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		t.Fatalf("hash file: %v", err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func fileSize(t testing.TB, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	return info.Size()
}
