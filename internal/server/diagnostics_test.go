package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"openwatcher/internal/config"
	"openwatcher/internal/pairing"
)

func TestDiagnosticUploadRejectsNonPost(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token), DiagnosticUploadDir: t.TempDir()}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("diagnostic upload get status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestDiagnosticUploadRequiresToken(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token), DiagnosticUploadDir: t.TempDir()}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics", bytes.NewReader(validDiagnosticGZIPFixture(t, `{"event":"snapshot"}`)))
	req.Header.Set("Content-Type", diagnosticContentType)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("diagnostic upload without token = %d body=%s", res.Code, res.Body.String())
	}
}

func TestDiagnosticUploadRejectsNonGZIPContentType(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token), DiagnosticUploadDir: t.TempDir()}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics", strings.NewReader("not gzip"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("diagnostic upload content type status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestDiagnosticUploadRejectsOversizedBody(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token), DiagnosticUploadDir: t.TempDir()}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics", bytes.NewReader(bytes.Repeat([]byte{'x'}, diagnosticMaxBytes+1)))
	req.Header.Set("Content-Type", diagnosticContentType)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("diagnostic upload oversized status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestDiagnosticUploadRejectsUnreadableGZIP(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token), DiagnosticUploadDir: t.TempDir()}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics", strings.NewReader("not gzip"))
	req.Header.Set("Content-Type", diagnosticContentType)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("diagnostic upload invalid gzip status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestDiagnosticUploadStoresFilesAndReturnsMetadata(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	uploadDir := filepath.Join(t.TempDir(), "diagnostics")
	cfg := config.Config{TokenHash: pairing.HashToken(token), DiagnosticUploadDir: uploadDir}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})
	now := time.Date(2026, 6, 6, 17, 2, 15, 0, time.UTC)
	app.clock = func() time.Time { return now }

	body := validDiagnosticGZIPFixture(t, `{"event":"network_request"}`, `{"event":"snapshot_home_dashboard"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics", bytes.NewReader(body))
	req.RemoteAddr = "192.168.0.2:4567"
	req.Header.Set("Content-Type", "application/gzip; charset=binary")
	req.Header.Set("X-OpenWatcher-Token", token)
	req.Header.Set("X-OpenWatcher-Device-Name", "Xiaomi Watch 5")
	req.Header.Set("X-OpenWatcher-App-Version", "0.13.2")
	req.Header.Set("X-OpenWatcher-Diagnostic-Hours", "24")
	req.Header.Set("X-OpenWatcher-Diagnostic-Started-At", "2026-06-06T16:00:00Z")
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("diagnostic upload success status = %d body=%s", res.Code, res.Body.String())
	}

	var payload diagnosticUploadResponse
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode diagnostic payload: %v body=%s", err, res.Body.String())
	}
	if !payload.OK {
		t.Fatalf("diagnostic payload ok = false: %#v", payload)
	}
	if payload.ReceivedAt != now.Format(time.RFC3339) {
		t.Fatalf("receivedAt = %q want %q", payload.ReceivedAt, now.Format(time.RFC3339))
	}
	wantPrefix := "diag-20260606T170215Z-"
	if !strings.HasPrefix(payload.DiagnosticID, wantPrefix) || len(payload.DiagnosticID) != len(wantPrefix)+8 {
		t.Fatalf("diagnosticId = %q", payload.DiagnosticID)
	}

	dayDir := filepath.Join(uploadDir, "2026-06-06")
	archivePath := filepath.Join(dayDir, payload.DiagnosticID+".jsonl.gz")
	metadataPath := filepath.Join(dayDir, payload.DiagnosticID+".json")

	archiveBytes, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read diagnostic archive: %v", err)
	}
	if !bytes.Equal(archiveBytes, body) {
		t.Fatalf("saved archive mismatch")
	}
	if got := mustFileMode(t, archivePath); got != 0o600 {
		t.Fatalf("archive mode = %o want 0600", got)
	}

	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read diagnostic metadata: %v", err)
	}
	if got := mustFileMode(t, metadataPath); got != 0o600 {
		t.Fatalf("metadata mode = %o want 0600", got)
	}

	var metadata diagnosticUploadMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("decode diagnostic metadata: %v body=%s", err, string(metadataBytes))
	}
	if metadata.DiagnosticID != payload.DiagnosticID {
		t.Fatalf("metadata diagnosticId = %q want %q", metadata.DiagnosticID, payload.DiagnosticID)
	}
	if metadata.ReceivedAt != payload.ReceivedAt {
		t.Fatalf("metadata receivedAt = %q want %q", metadata.ReceivedAt, payload.ReceivedAt)
	}
	if metadata.DeviceName != "Xiaomi Watch 5" || metadata.AppVersion != "0.13.2" {
		t.Fatalf("metadata identity = %#v", metadata)
	}
	if metadata.Hours != 24 {
		t.Fatalf("metadata hours = %d want 24", metadata.Hours)
	}
	if metadata.ContentType != diagnosticContentType {
		t.Fatalf("metadata content type = %q want %q", metadata.ContentType, diagnosticContentType)
	}
	if metadata.CompressedBytes != int64(len(body)) {
		t.Fatalf("metadata compressed bytes = %d want %d", metadata.CompressedBytes, len(body))
	}
	if metadata.RemoteAddr != "192.168.0.2:4567" {
		t.Fatalf("metadata remote addr = %q", metadata.RemoteAddr)
	}
}

func TestDiagnosticUploadCleansUpFilesOlderThanSevenDays(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	uploadDir := filepath.Join(t.TempDir(), "diagnostics")
	cfg := config.Config{TokenHash: pairing.HashToken(token), DiagnosticUploadDir: uploadDir}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})
	now := time.Date(2026, 6, 6, 17, 2, 15, 0, time.UTC)
	app.clock = func() time.Time { return now }

	oldTime := now.Add(-diagnosticRetention - time.Hour)
	oldDir := filepath.Join(uploadDir, "2026-05-29")
	writeDiagnosticFixtureFile(t, filepath.Join(oldDir, "diag-old.jsonl.gz"), []byte("old-archive"), oldTime)
	writeDiagnosticFixtureFile(t, filepath.Join(oldDir, "diag-old.json"), []byte(`{"diagnosticId":"diag-old"}`), oldTime)

	recentTime := now.Add(-6 * 24 * time.Hour)
	recentDir := filepath.Join(uploadDir, "2026-05-31")
	writeDiagnosticFixtureFile(t, filepath.Join(recentDir, "diag-recent.jsonl.gz"), []byte("recent-archive"), recentTime)
	writeDiagnosticFixtureFile(t, filepath.Join(recentDir, "diag-recent.json"), []byte(`{"diagnosticId":"diag-recent"}`), recentTime)

	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics", bytes.NewReader(validDiagnosticGZIPFixture(t, `{"event":"diagnostic_upload_requested"}`)))
	req.Header.Set("Content-Type", diagnosticContentType)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("diagnostic upload cleanup status = %d body=%s", res.Code, res.Body.String())
	}

	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("old diagnostic dir still exists err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(recentDir, "diag-recent.jsonl.gz")); err != nil {
		t.Fatalf("recent archive missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(recentDir, "diag-recent.json")); err != nil {
		t.Fatalf("recent metadata missing: %v", err)
	}
}

func validDiagnosticGZIPFixture(t *testing.T, lines ...string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	for _, line := range lines {
		if _, err := writer.Write([]byte(line + "\n")); err != nil {
			t.Fatalf("write gzip fixture: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip fixture: %v", err)
	}
	return buffer.Bytes()
}

func writeDiagnosticFixtureFile(t *testing.T, path string, data []byte, modTime time.Time) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes fixture file: %v", err)
	}
}

func mustFileMode(t *testing.T, path string) os.FileMode {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
