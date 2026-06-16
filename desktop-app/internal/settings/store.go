package settings

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	rootconfig "openwatcher/internal/config"
)

func AppRoot() string {
	wd, err := os.Getwd()
	if err == nil {
		if root := findProjectRoot(wd); root != "" {
			return root
		}
	}
	exePath, err := os.Executable()
	if err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		if root := findProjectRoot(exeDir); root != "" {
			return root
		}
		return exeDir
	}
	if err == nil {
		return wd
	}
	return "."
}

func findProjectRoot(start string) string {
	current := filepath.Clean(start)
	for {
		if fileExists(filepath.Join(current, "wails.json")) {
			return current
		}
		if fileExists(filepath.Join(current, "desktop-app", "wails.json")) {
			return filepath.Join(current, "desktop-app")
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func BundledResourceRoots(appRoot string) []string {
	roots := make([]string, 0, 2)
	add := func(path string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			return
		}
		cleaned := filepath.Clean(trimmed)
		for _, existing := range roots {
			if existing == cleaned {
				return
			}
		}
		roots = append(roots, cleaned)
	}

	exePath, err := os.Executable()
	if err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		add(filepath.Join(exeDir, "bundled"))
		add(filepath.Join(exeDir, "..", "Resources", "bundled"))
	}

	add(filepath.Join(appRoot, "bundled"))

	return roots
}

func RuntimeResourceRoots(appRoot string) []string {
	roots := make([]string, 0, 3)
	add := func(path string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			return
		}
		cleaned := filepath.Clean(trimmed)
		for _, existing := range roots {
			if existing == cleaned {
				return
			}
		}
		roots = append(roots, cleaned)
	}

	if runtimeDir, err := RuntimeDir(); err == nil {
		add(runtimeDir)
	}
	for _, root := range BundledResourceRoots(appRoot) {
		add(root)
	}
	return roots
}

func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(base, "OpenWatcher"), nil
	case "darwin":
		return filepath.Join(base, "OpenWatcher"), nil
	default:
		return filepath.Join(base, "openwatcher"), nil
	}
}

func RuntimeDir() (string, error) {
	configDir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "runtime"), nil
}

func ConfigDirLabel() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "~/Library/Application Support/OpenWatcher", nil
	case "windows":
		return "%APPDATA%/OpenWatcher", nil
	default:
		return "~/.config/openwatcher", nil
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func BackendConfigPath() string {
	path, err := rootconfig.ResolvePath("")
	if err != nil {
		return ""
	}
	return path
}
