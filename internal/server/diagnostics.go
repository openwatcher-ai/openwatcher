package server

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"openwatcher/internal/config"
)

const (
	diagnosticMaxBytes          = 4 << 20
	diagnosticContentType       = "application/gzip"
	diagnosticRetention         = 7 * 24 * time.Hour
	diagnosticIDRandomByteCount = 4
)

type diagnosticUploadResponse struct {
	OK           bool   `json:"ok"`
	DiagnosticID string `json:"diagnosticId"`
	ReceivedAt   string `json:"receivedAt"`
}

type diagnosticUploadMetadata struct {
	DiagnosticID    string `json:"diagnosticId"`
	ReceivedAt      string `json:"receivedAt"`
	DeviceName      string `json:"deviceName"`
	AppVersion      string `json:"appVersion"`
	Hours           int    `json:"hours"`
	ContentType     string `json:"contentType"`
	CompressedBytes int64  `json:"compressedBytes"`
	RemoteAddr      string `json:"remoteAddr"`
}

func (a *App) handleDiagnosticUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	contentType, ok := parseGZIPContentType(r.Header.Get("Content-Type"))
	if !ok {
		writeError(w, http.StatusUnsupportedMediaType, "content type must be application/gzip")
		return
	}

	body := http.MaxBytesReader(w, r.Body, diagnosticMaxBytes)
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "diagnostic upload is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "failed to read diagnostic upload")
		return
	}
	if err := validateGZIPData(data); err != nil {
		writeError(w, http.StatusBadRequest, "body must be readable gzip")
		return
	}

	now := a.clock().UTC()
	dir := a.diagnosticUploadDir()
	metadata := diagnosticUploadMetadata{
		ReceivedAt:      now.Format(time.RFC3339),
		DeviceName:      strings.TrimSpace(watcherHeaderValue(r.Header, "Device-Name")),
		AppVersion:      strings.TrimSpace(watcherHeaderValue(r.Header, "App-Version")),
		Hours:           parseDiagnosticHours(watcherHeaderValue(r.Header, "Diagnostic-Hours")),
		ContentType:     contentType,
		CompressedBytes: int64(len(data)),
		RemoteAddr:      r.RemoteAddr,
	}

	stored, err := saveDiagnosticUpload(dir, now, data, metadata)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save diagnostic upload")
		return
	}
	if err := cleanupDiagnosticUploads(dir, now.Add(-diagnosticRetention)); err != nil {
		log.Printf("watcher_diagnostic_cleanup_failed dir=%q err=%v", dir, err)
	}

	writeJSON(w, http.StatusOK, stored)
}

func (a *App) diagnosticUploadDir() string {
	a.mu.RLock()
	dir := strings.TrimSpace(a.cfg.DiagnosticUploadDir)
	a.mu.RUnlock()
	if dir == "" {
		dir = config.DefaultDiagnosticUploadDir
	}
	return expandHomePath(dir)
}

func parseGZIPContentType(value string) (string, bool) {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}
	if !strings.EqualFold(mediaType, diagnosticContentType) {
		return "", false
	}
	return diagnosticContentType, true
}

func validateGZIPData(data []byte) error {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	_, readErr := io.Copy(io.Discard, reader)
	closeErr := reader.Close()
	if readErr != nil {
		return readErr
	}
	return closeErr
}

func parseDiagnosticHours(value string) int {
	hours, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || hours < 0 {
		return 0
	}
	return hours
}

func saveDiagnosticUpload(root string, now time.Time, compressed []byte, metadata diagnosticUploadMetadata) (diagnosticUploadResponse, error) {
	dayDir := filepath.Join(root, now.Format("2006-01-02"))
	if err := os.MkdirAll(dayDir, 0o700); err != nil {
		return diagnosticUploadResponse{}, err
	}

	for attempt := 0; attempt < 100; attempt++ {
		diagnosticID, err := buildDiagnosticID(now)
		if err != nil {
			return diagnosticUploadResponse{}, err
		}

		metadata.DiagnosticID = diagnosticID
		metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
		if err != nil {
			return diagnosticUploadResponse{}, err
		}
		metadataBytes = append(metadataBytes, '\n')

		archivePath := filepath.Join(dayDir, diagnosticID+".jsonl.gz")
		metadataPath := filepath.Join(dayDir, diagnosticID+".json")
		if err := writePrivateFileExclusive(archivePath, compressed); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return diagnosticUploadResponse{}, err
		}
		if err := writePrivateFileExclusive(metadataPath, metadataBytes); err != nil {
			_ = os.Remove(archivePath)
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return diagnosticUploadResponse{}, err
		}

		return diagnosticUploadResponse{
			OK:           true,
			DiagnosticID: diagnosticID,
			ReceivedAt:   metadata.ReceivedAt,
		}, nil
	}

	return diagnosticUploadResponse{}, os.ErrExist
}

func buildDiagnosticID(now time.Time) (string, error) {
	suffix := make([]byte, diagnosticIDRandomByteCount)
	if _, err := io.ReadFull(rand.Reader, suffix); err != nil {
		return "", err
	}
	return "diag-" + now.Format(screenshotFilenameTime) + "-" + hex.EncodeToString(suffix), nil
}

func writePrivateFileExclusive(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return nil
}

func cleanupDiagnosticUploads(root string, cutoff time.Time) error {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(root, entry.Name())
		if err := cleanupDiagnosticUploadDir(dirPath, cutoff); err != nil {
			return err
		}
	}
	return nil
}

func cleanupDiagnosticUploadDir(dir string, cutoff time.Time) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !isDiagnosticUploadFile(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	remaining, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(remaining) == 0 {
		if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func isDiagnosticUploadFile(name string) bool {
	return strings.HasSuffix(name, ".jsonl.gz") || strings.HasSuffix(name, ".json")
}
