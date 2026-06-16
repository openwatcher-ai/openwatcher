package server

import (
	"errors"
	"io"
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
	screenshotMaxBytes     = 1 << 20
	screenshotFilenameTime = "20060102T150405Z"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

type screenshotUploadResponse struct {
	OK       bool   `json:"ok"`
	Filename string `json:"filename"`
}

func (a *App) handleScreenshotUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !isPNGContentType(r.Header.Get("Content-Type")) {
		writeError(w, http.StatusUnsupportedMediaType, "content type must be image/png")
		return
	}

	body := http.MaxBytesReader(w, r.Body, screenshotMaxBytes)
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "screenshot is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "failed to read screenshot")
		return
	}
	if !hasPNGSignature(data) {
		writeError(w, http.StatusUnsupportedMediaType, "body must be png")
		return
	}

	dir := a.screenshotUploadDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create screenshot directory")
		return
	}

	baseFilename := buildScreenshotFilename(
		a.clock().UTC(),
		watcherHeaderValue(r.Header, "Device-Name"),
		watcherHeaderValue(r.Header, "App-Version"),
	)
	file, filename, err := createScreenshotFile(dir, baseFilename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save screenshot")
		return
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(filepath.Join(dir, filename))
		writeError(w, http.StatusInternalServerError, "failed to save screenshot")
		return
	}

	writeJSON(w, http.StatusOK, screenshotUploadResponse{
		OK:       true,
		Filename: filename,
	})
}

func createScreenshotFile(dir, filename string) (*os.File, string, error) {
	for attempt := 0; attempt < 100; attempt++ {
		candidate := filename
		if attempt > 0 {
			candidate = strings.TrimSuffix(filename, ".png") + "-" + strconv.Itoa(attempt+1) + ".png"
		}
		file, err := os.OpenFile(filepath.Join(dir, candidate), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return file, candidate, nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return nil, "", err
	}
	return nil, "", os.ErrExist
}

func (a *App) screenshotUploadDir() string {
	a.mu.RLock()
	dir := strings.TrimSpace(a.cfg.ScreenshotUploadDir)
	a.mu.RUnlock()
	if dir == "" {
		dir = config.DefaultScreenshotUploadDir
	}
	return expandHomePath(dir)
}

func isPNGContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "image/png")
}

func hasPNGSignature(data []byte) bool {
	if len(data) < len(pngSignature) {
		return false
	}
	for index, expected := range pngSignature {
		if data[index] != expected {
			return false
		}
	}
	return true
}

func buildScreenshotFilename(now time.Time, deviceName, appVersion string) string {
	device := sanitizeFilenameComponent(deviceName, "watch", false)
	version := sanitizeFilenameComponent(appVersion, "app", true)
	return "watch-" + now.Format(screenshotFilenameTime) + "-" + device + "-" + version + ".png"
}

func sanitizeFilenameComponent(value, fallback string, allowDot bool) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, char := range value {
		allowed := (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') ||
			(allowDot && char == '.')
		if allowed {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-.")
	if result == "" {
		return fallback
	}
	if len(result) > 48 {
		result = strings.Trim(result[:48], "-.")
	}
	if result == "" {
		return fallback
	}
	return result
}

func expandHomePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(trimmed, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
		}
	}
	return trimmed
}
