package backend

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"openwatcher/desktop-app/internal/settings"
)

var ErrBinaryNotFound = errors.New("backend sidecar not found")

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
	binaryName := "openwatcher"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	repoRoot := filepath.Clean(filepath.Join(l.appRoot, ".."))
	platformDir := runtime.GOOS + "-" + runtime.GOARCH
	candidates := make([]ResolvedBinary, 0, 5)
	for _, root := range settings.BundledResourceRoots(l.appRoot) {
		candidates = append(candidates, ResolvedBinary{
			Label: filepath.ToSlash(filepath.Join(filepath.Base(root), "openwatcher", platformDir)),
			Path:  filepath.Join(root, "openwatcher", platformDir, binaryName),
		})
	}
	candidates = append(candidates,
		ResolvedBinary{
			Label: "bin/openwatcher",
			Path:  filepath.Join(repoRoot, "bin", binaryName),
		},
		ResolvedBinary{
			Label: "build/bin/openwatcher",
			Path:  filepath.Join(repoRoot, "build", "bin", binaryName),
		},
	)
	return candidates
}

func (l *BinaryLocator) Resolve() (ResolvedBinary, error) {
	for _, candidate := range l.Candidates() {
		info, err := os.Stat(candidate.Path)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return ResolvedBinary{}, ErrBinaryNotFound
}

func (l *BinaryLocator) FriendlyError() string {
	return "未找到 OpenWatcher 本机服务组件。请把二进制放到 bundled/openwatcher/<platform>/，或在仓库根目录生成 bin/openwatcher。"
}
