package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"openwatcher/desktop-app/internal/settings"
)

const defaultRuntimeChannelManifestURL = "https://openwatcher.ai/channels/beta.json"

func main() {
	if len(os.Args) != 2 {
		fatal("usage: syncbundle <compiled-binary-path>")
	}

	binPath, err := filepath.Abs(os.Args[1])
	if err != nil {
		fatal("resolve binary path: %v", err)
	}

	desktopRoot := settings.AppRoot()
	repoRoot := filepath.Clean(filepath.Join(desktopRoot, ".."))
	bundledRoot, appPath, err := targetBundledRoot(binPath)
	if err != nil {
		fatal("%v", err)
	}

	if err := os.RemoveAll(bundledRoot); err != nil {
		fatal("remove existing bundled dir: %v", err)
	}
	if err := os.MkdirAll(bundledRoot, 0o755); err != nil {
		fatal("create bundled dir: %v", err)
	}

	goosName, goarchName := targetPlatform()
	platformDir := goosName + "-" + goarchName
	sidecarName := "openwatcher"
	if goosName == "windows" {
		sidecarName += ".exe"
	}

	gitCommit := gitShortCommit(repoRoot)
	builtAt := time.Now().UTC().Format(time.RFC3339)
	buildVersion := sidecarBuildVersion(repoRoot)
	sidecarTarget := filepath.Join(bundledRoot, "openwatcher", platformDir, sidecarName)
	if err := os.MkdirAll(filepath.Dir(sidecarTarget), 0o755); err != nil {
		fatal("create sidecar dir: %v", err)
	}
	buildCmd := exec.Command("go", "build",
		"-trimpath",
		"-ldflags", sidecarLDFlags(goosName, buildVersion, gitCommit, builtAt),
		"-o", sidecarTarget,
		"./cmd/openwatcher",
	)
	buildCmd.Env = append(os.Environ(), "GOOS="+goosName, "GOARCH="+goarchName)
	buildCmd.Dir = repoRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fatal("build sidecar: %v", err)
	}
	if err := os.Chmod(sidecarTarget, 0o755); err != nil {
		fatal("chmod sidecar: %v", err)
	}

	updaterName := "openwatcher-updater"
	if goosName == "windows" {
		updaterName += ".exe"
	}
	updaterTarget := filepath.Join(bundledRoot, "updater", platformDir, updaterName)
	if err := os.MkdirAll(filepath.Dir(updaterTarget), 0o755); err != nil {
		fatal("create updater dir: %v", err)
	}
	updaterCmd := exec.Command("go", "build",
		"-trimpath",
		"-o", updaterTarget,
		"./desktop-app/cmd/updater",
	)
	updaterCmd.Env = append(os.Environ(), "GOOS="+goosName, "GOARCH="+goarchName)
	updaterCmd.Dir = repoRoot
	updaterCmd.Stdout = os.Stdout
	updaterCmd.Stderr = os.Stderr
	if err := updaterCmd.Run(); err != nil {
		fatal("build updater: %v", err)
	}
	if err := os.Chmod(updaterTarget, 0o755); err != nil {
		fatal("chmod updater: %v", err)
	}

	if err := buildWidget(repoRoot, bundledRoot, appPath, goosName, goarchName, widgetBundleVersion()); err != nil {
		fatal("build widget: %v", err)
	}

	channelManifestURLPath := filepath.Join(bundledRoot, "runtime", "channel-manifest-url.txt")
	if err := os.MkdirAll(filepath.Dir(channelManifestURLPath), 0o755); err != nil {
		fatal("create runtime dir: %v", err)
	}
	if err := os.WriteFile(channelManifestURLPath, []byte(runtimeChannelManifestURL()+"\n"), 0o644); err != nil {
		fatal("write runtime channel manifest url: %v", err)
	}

	if runtime.GOOS == "darwin" && appPath != "" {
		helperPath := widgetTargets(bundledRoot, appPath, goosName, goarchName).AppBundle
		helperSignCmd := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", helperPath)
		helperSignCmd.Stdout = os.Stdout
		helperSignCmd.Stderr = os.Stderr
		if err := helperSignCmd.Run(); err != nil {
			fatal("codesign widget app bundle: %v", err)
		}
		signCmd := exec.Command("/usr/bin/codesign", "--force", "--deep", "--sign", "-", appPath)
		signCmd.Stdout = os.Stdout
		signCmd.Stderr = os.Stderr
		if err := signCmd.Run(); err != nil {
			fatal("codesign app bundle: %v", err)
		}
	}

	fmt.Printf("synced bundled resources into %s\n", bundledRoot)
}

type widgetBuildTargets struct {
	Executable string
	AppBundle  string
}

func widgetTargets(bundledRoot, appPath, goosName, goarchName string) widgetBuildTargets {
	if goosName == "darwin" && appPath != "" {
		appBundle := filepath.Join(appPath, "Contents", "Library", "Helpers", "OpenWatcher Widget.app")
		return widgetBuildTargets{
			Executable: filepath.Join(appBundle, "Contents", "MacOS", "openwatcher-widget"),
			AppBundle:  appBundle,
		}
	}
	name := "openwatcher-widget"
	if goosName == "windows" {
		name += ".exe"
	}
	return widgetBuildTargets{
		Executable: filepath.Join(bundledRoot, "widget", goosName+"-"+goarchName, name),
	}
}

func buildWidget(repoRoot, bundledRoot, appPath, goosName, goarchName, version string) error {
	frontendDir := filepath.Join(repoRoot, "desktop-app", "widget", "frontend-vue3")
	for _, args := range [][]string{{"ci"}, {"run", "build"}} {
		cmd := exec.Command("npm", args...)
		cmd.Dir = frontendDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("widget frontend npm %s: %w", strings.Join(args, " "), err)
		}
	}

	targets := widgetTargets(bundledRoot, appPath, goosName, goarchName)
	if goosName == "darwin" && targets.AppBundle == "" {
		return fmt.Errorf("macOS widget 缺少宿主 app bundle")
	}
	if targets.AppBundle != "" {
		if err := os.RemoveAll(targets.AppBundle); err != nil {
			return fmt.Errorf("remove old widget app: %w", err)
		}
		if err := os.MkdirAll(filepath.Join(targets.AppBundle, "Contents", "MacOS"), 0o755); err != nil {
			return fmt.Errorf("create widget app executable dir: %w", err)
		}
		if err := writeWidgetInfoPlist(filepath.Join(targets.AppBundle, "Contents", "Info.plist"), version); err != nil {
			return err
		}
	} else if err := os.MkdirAll(filepath.Dir(targets.Executable), 0o755); err != nil {
		return fmt.Errorf("create widget binary dir: %w", err)
	}

	args := []string{"build", "-trimpath", "-tags", "desktop,production"}
	if goosName == "windows" {
		args = append(args, "-ldflags", "-H windowsgui")
	}
	args = append(args, "-o", targets.Executable, "./desktop-app/widget")
	cmd := exec.Command("go", args...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GOOS="+goosName, "GOARCH="+goarchName)
	if goosName == "windows" {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build widget binary: %w", err)
	}
	if err := os.Chmod(targets.Executable, 0o755); err != nil {
		return fmt.Errorf("chmod widget binary: %w", err)
	}
	return nil
}

func writeWidgetInfoPlist(path, version string) error {
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDisplayName</key><string>OpenWatcher Widget</string>
  <key>CFBundleExecutable</key><string>openwatcher-widget</string>
  <key>CFBundleIdentifier</key><string>ai.openwatcher.widget</string>
  <key>CFBundleName</key><string>OpenWatcher Widget</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>%s</string>
  <key>CFBundleVersion</key><string>%s</string>
  <key>LSMinimumSystemVersion</key><string>10.13.0</string>
  <key>LSUIElement</key><true/>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
`, version, version)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create widget Info.plist dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		return fmt.Errorf("write widget Info.plist: %w", err)
	}
	return nil
}

var bundleVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func widgetBundleVersion() string {
	raw := strings.TrimPrefix(strings.TrimSpace(os.Getenv("OPENWATCHER_DESKTOP_VERSION")), "v")
	parts := strings.FieldsFunc(raw, func(value rune) bool { return value == '-' || value == '+' })
	if len(parts) == 0 {
		return "0.0.0"
	}
	core := parts[0]
	if bundleVersionPattern.MatchString(core) {
		return core
	}
	return "0.0.0"
}

func targetBundledRoot(binPath string) (string, string, error) {
	exeDir := filepath.Dir(binPath)
	contentsDir := filepath.Dir(exeDir)
	if filepath.Base(exeDir) == "MacOS" && filepath.Base(contentsDir) == "Contents" {
		appPath := filepath.Dir(contentsDir)
		return filepath.Join(contentsDir, "Resources", "bundled"), appPath, nil
	}
	if exeDir == "" {
		return "", "", fmt.Errorf("无法解析可执行文件目录")
	}
	return filepath.Join(exeDir, "bundled"), "", nil
}

func targetPlatform() (string, string) {
	if platform := strings.TrimSpace(os.Getenv("OPENWATCHER_BUNDLE_PLATFORM")); platform != "" {
		parts := strings.Split(platform, "-")
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return normalizeGOOS(parts[0]), normalizeGOARCH(parts[1])
		}
	}
	goosName := strings.TrimSpace(os.Getenv("GOOS"))
	goarchName := strings.TrimSpace(os.Getenv("GOARCH"))
	if goosName == "" {
		goosName = runtime.GOOS
	}
	if goarchName == "" {
		goarchName = runtime.GOARCH
	}
	return normalizeGOOS(goosName), normalizeGOARCH(goarchName)
}

func normalizeGOOS(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeGOARCH(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "x86_64" {
		return "amd64"
	}
	if value == "aarch64" {
		return "arm64"
	}
	return value
}

func gitShortCommit(repoRoot string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func runtimeChannelManifestURL() string {
	if envValue := strings.TrimSpace(os.Getenv("OPENWATCHER_RUNTIME_CHANNEL_MANIFEST_URL")); envValue != "" {
		return envValue
	}
	return defaultRuntimeChannelManifestURL
}

func sidecarBuildVersion(repoRoot string) string {
	for _, key := range []string{"OPENWATCHER_BACKEND_VERSION", "OPENWATCHER_DESKTOP_VERSION"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return "dev-" + gitShortCommit(repoRoot)
}

func sidecarLDFlags(goosName string, buildVersion string, gitCommit string, builtAt string) string {
	return strings.Join([]string{
		fmt.Sprintf("-X openwatcher/internal/buildinfo.Version=%s", buildVersion),
		fmt.Sprintf("-X openwatcher/internal/buildinfo.Commit=%s", gitCommit),
		fmt.Sprintf("-X openwatcher/internal/buildinfo.BuiltAt=%s", builtAt),
	}, " ")
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
