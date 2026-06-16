package adb

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"openwatcher/desktop-app/internal/settings"
)

type ResolvedBinary struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

type BinaryLocator struct {
	appRoot string
}

func NewBinaryLocator(appRoot string) *BinaryLocator {
	return &BinaryLocator{appRoot: appRoot}
}

func (l *BinaryLocator) Candidates() []ResolvedBinary {
	binaryName := "adb"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	homeDir, _ := os.UserHomeDir()
	candidates := make([]ResolvedBinary, 0, 6)
	for _, root := range settings.RuntimeResourceRoots(l.appRoot) {
		candidates = append(candidates, ResolvedBinary{
			Label: filepath.ToSlash(filepath.Join(filepath.Base(root), "platform-tools", runtime.GOOS+"-"+runtime.GOARCH)),
			Path:  filepath.Join(root, "platform-tools", runtime.GOOS+"-"+runtime.GOARCH, binaryName),
		})
	}
	candidates = append(candidates,
		ResolvedBinary{
			Label: "ANDROID_HOME/platform-tools",
			Path:  filepath.Join(stringsOrEmpty(os.Getenv("ANDROID_HOME")), "platform-tools", binaryName),
		},
		ResolvedBinary{
			Label: "ANDROID_SDK_ROOT/platform-tools",
			Path:  filepath.Join(stringsOrEmpty(os.Getenv("ANDROID_SDK_ROOT")), "platform-tools", binaryName),
		},
		ResolvedBinary{
			Label: "macOS Android SDK default",
			Path:  filepath.Join(homeDir, "Library", "Android", "sdk", "platform-tools", binaryName),
		},
	)
	return candidates
}

func (l *BinaryLocator) Resolve() (ResolvedBinary, error) {
	for _, candidate := range l.Candidates() {
		if candidate.Path == "" {
			continue
		}
		info, err := os.Stat(candidate.Path)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	if fromPath, err := exec.LookPath("adb"); err == nil {
		return ResolvedBinary{Label: "system PATH", Path: fromPath}, nil
	}
	return ResolvedBinary{}, ErrBinaryNotFound
}

func (l *BinaryLocator) FriendlyError() string {
	return "未找到 ADB。请检查网络后稍等片刻让 Desktop 自动下载，或在 bundled 中提供 platform-tools，或安装 Android SDK。"
}

func stringsOrEmpty(value string) string {
	return value
}
