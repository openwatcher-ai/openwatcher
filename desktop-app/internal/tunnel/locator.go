package tunnel

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
	binaryName := "cloudflared"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	platformDir := runtime.GOOS + "-" + runtime.GOARCH
	candidates := make([]ResolvedBinary, 0, 4)
	for _, root := range settings.RuntimeResourceRoots(l.appRoot) {
		candidates = append(candidates, ResolvedBinary{
			Label: filepath.ToSlash(filepath.Join(filepath.Base(root), "cloudflared", platformDir)),
			Path:  filepath.Join(root, "cloudflared", platformDir, binaryName),
		})
	}
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
	if fromPath, err := exec.LookPath("cloudflared"); err == nil {
		return ResolvedBinary{Label: "system PATH", Path: fromPath}, nil
	}
	return ResolvedBinary{}, ErrBinaryNotFound
}

func (l *BinaryLocator) FriendlyError() string {
	return "未找到 cloudflared。请检查网络后稍等片刻让 Desktop 自动下载，或在 bundled/cloudflared/<platform>/ 提供可执行文件，或在开发环境安装 cloudflared。"
}
