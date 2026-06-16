package server

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"openwatcher/internal/config"
)

var errLatestAPKNotFound = errors.New("latest release apk not found")

type latestAPKFile struct {
	Path    string
	Name    string
	ModTime time.Time
	Size    int64
}

func (a *App) handleLatestAPK(w http.ResponseWriter, r *http.Request) {
	a.handleDevAPKArtifact(w, r)
}

func (a *App) handleDevLatestAPK(w http.ResponseWriter, r *http.Request) {
	a.handleDevAPKArtifact(w, r)
}

func (a *App) handleDevAPKArtifact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.devUpdateAuthorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	file, err := resolveLatestReleaseAPK(a.apkDistDir())
	if err != nil {
		if errors.Is(err, errLatestAPKNotFound) {
			writeError(w, http.StatusNotFound, "latest release apk not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to locate latest release apk")
		return
	}

	handle, err := os.Open(file.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "latest release apk not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to open latest release apk")
		return
	}
	defer handle.Close()

	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": file.Name,
	}))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-OpenWatcher-Apk-Filename", file.Name)
	w.Header().Set("X-OpenWatcher-Apk-Size", strconvFormatInt(file.Size))
	http.ServeContent(w, r, file.Name, file.ModTime, handle)
}

func (a *App) handleLatestAPKMetadata(w http.ResponseWriter, r *http.Request) {
	a.handleDevAPKJSON(w, r, "latest-apk.json", "latest release apk metadata not found", "failed to open latest release apk metadata")
}

func (a *App) handleDevLatestAPKMetadata(w http.ResponseWriter, r *http.Request) {
	a.handleDevAPKJSON(w, r, "latest-apk.json", "latest release apk metadata not found", "failed to open latest release apk metadata")
}

func (a *App) handleLatestAPKChangelog(w http.ResponseWriter, r *http.Request) {
	a.handleDevAPKJSON(w, r, "latest-apk-changelog.json", "latest release apk changelog not found", "failed to open latest release apk changelog")
}

func (a *App) handleDevLatestAPKChangelog(w http.ResponseWriter, r *http.Request) {
	a.handleDevAPKJSON(w, r, "latest-apk-changelog.json", "latest release apk changelog not found", "failed to open latest release apk changelog")
}

func (a *App) handleDevAPKJSON(w http.ResponseWriter, r *http.Request, filename, notFoundMessage, openErrorMessage string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.devUpdateAuthorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	path := filepath.Join(a.apkDistDir(), filename)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, notFoundMessage)
		return
	}
	handle, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openErrorMessage)
		return
	}
	defer handle.Close()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, filename, info.ModTime(), handle)
}

func (a *App) apkDistDir() string {
	a.mu.RLock()
	dir := strings.TrimSpace(a.cfg.ApkDistDir)
	a.mu.RUnlock()
	if dir == "" {
		return config.DefaultApkDistDir
	}
	return dir
}

func resolveLatestReleaseAPK(dir string) (latestAPKFile, error) {
	if file, err := latestReleaseAPKFromMetadata(dir); err == nil {
		return file, nil
	}
	return findLatestReleaseAPK(dir)
}

func latestReleaseAPKFromMetadata(dir string) (latestAPKFile, error) {
	data, err := os.ReadFile(filepath.Join(dir, "latest-apk.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return latestAPKFile{}, errLatestAPKNotFound
		}
		return latestAPKFile{}, err
	}
	var metadata struct {
		Artifact string `json:"artifact"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil || strings.TrimSpace(metadata.Artifact) == "" {
		return latestAPKFile{}, errLatestAPKNotFound
	}
	name := filepath.Base(strings.TrimSpace(metadata.Artifact))
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil || !info.Mode().IsRegular() {
		return latestAPKFile{}, errLatestAPKNotFound
	}
	return latestAPKFile{
		Path:    filepath.Join(dir, name),
		Name:    name,
		ModTime: info.ModTime(),
		Size:    info.Size(),
	}, nil
}

func findLatestReleaseAPK(dir string) (latestAPKFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return latestAPKFile{}, errLatestAPKNotFound
		}
		return latestAPKFile{}, err
	}

	var latest latestAPKFile
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || !isReleaseAPKName(entry.Name()) {
			continue
		}

		candidate := latestAPKFile{
			Path:    filepath.Join(dir, entry.Name()),
			Name:    entry.Name(),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}
		if latest.Name == "" ||
			candidate.ModTime.After(latest.ModTime) ||
			(candidate.ModTime.Equal(latest.ModTime) && candidate.Name > latest.Name) {
			latest = candidate
		}
	}

	if latest.Name == "" {
		return latestAPKFile{}, errLatestAPKNotFound
	}
	return latest, nil
}

func isReleaseAPKName(name string) bool {
	lowerName := strings.ToLower(name)
	return !strings.HasPrefix(lowerName, ".") &&
		strings.HasSuffix(lowerName, ".apk") &&
		strings.Contains(lowerName, "release") &&
		!strings.Contains(lowerName, "debug")
}

func strconvFormatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
