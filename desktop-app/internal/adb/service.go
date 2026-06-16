package adb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"openwatcher/desktop-app/internal/logging"
	"openwatcher/desktop-app/internal/processutil"
)

var ErrBinaryNotFound = errors.New("adb not found")

type Status struct {
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
	Message   string `json:"message"`
}

type Device struct {
	Serial      string `json:"serial"`
	State       string `json:"state"`
	Product     string `json:"product,omitempty"`
	Model       string `json:"model,omitempty"`
	Device      string `json:"device,omitempty"`
	TransportID string `json:"transportId,omitempty"`
	IsEmulator  bool   `json:"isEmulator"`
	HostAlias   string `json:"hostAlias,omitempty"`
	IsWatch     bool   `json:"isWatch"`
	DisplayName string `json:"displayName"`
}

type CommandResult struct {
	Command  string `json:"command"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Combined string `json:"combined"`
	ExitCode int    `json:"exitCode"`
}

type PackageInfo struct {
	Installed   bool   `json:"installed"`
	VersionName string `json:"versionName,omitempty"`
	VersionCode int    `json:"versionCode,omitempty"`
	Path        string `json:"path,omitempty"`
}

type Service struct {
	locator  *BinaryLocator
	redactor *logging.Redactor
}

func NewService(locator *BinaryLocator, redactor *logging.Redactor) *Service {
	return &Service{locator: locator, redactor: redactor}
}

func (s *Service) Status(ctx context.Context) Status {
	resolved, err := s.locator.Resolve()
	if err != nil {
		return Status{
			Available: false,
			Message:   s.locator.FriendlyError(),
		}
	}
	version, versionErr := s.Version(ctx)
	message := "ADB 可用"
	if versionErr != nil {
		message = "ADB 已找到，但读取版本失败"
	}
	return Status{
		Available: true,
		Path:      resolved.Path,
		Version:   version,
		Message:   message,
	}
}

func (s *Service) Version(ctx context.Context) (string, error) {
	result, err := s.run(ctx, nil, "version")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Android Debug Bridge version") {
			return strings.TrimPrefix(trimmed, "Android Debug Bridge version "), nil
		}
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (s *Service) Devices(ctx context.Context) ([]Device, CommandResult, error) {
	result, err := s.run(ctx, nil, "devices", "-l")
	if err != nil {
		return nil, result, err
	}
	return parseDevices(result.Stdout), result, nil
}

func (s *Service) Pair(ctx context.Context, host string, port int, code string) (CommandResult, error) {
	return s.run(ctx, strings.NewReader(strings.TrimSpace(code)+"\n"), "pair", fmt.Sprintf("%s:%d", strings.TrimSpace(host), port))
}

func (s *Service) Connect(ctx context.Context, host string, port int) (CommandResult, error) {
	return s.run(ctx, nil, "connect", fmt.Sprintf("%s:%d", strings.TrimSpace(host), port))
}

func (s *Service) Install(ctx context.Context, serial string, apkPath string) (CommandResult, error) {
	args := []string{}
	if trimmed := strings.TrimSpace(serial); trimmed != "" {
		args = append(args, "-s", trimmed)
	}
	args = append(args, "install", "-r", apkPath)
	return s.run(ctx, nil, args...)
}

func (s *Service) StartApp(ctx context.Context, serial string, packageName string) (CommandResult, error) {
	args := []string{}
	if trimmed := strings.TrimSpace(serial); trimmed != "" {
		args = append(args, "-s", trimmed)
	}
	args = append(args, "shell", "monkey", "-p", packageName, "-c", "android.intent.category.LAUNCHER", "1")
	return s.run(ctx, nil, args...)
}

func (s *Service) StartDeepLink(ctx context.Context, serial string, deepLink string) (CommandResult, error) {
	args := []string{}
	if trimmed := strings.TrimSpace(serial); trimmed != "" {
		args = append(args, "-s", trimmed)
	}
	safeLink := strings.ReplaceAll(deepLink, `'`, `'\''`)
	args = append(args, "shell", "am start -W -a android.intent.action.VIEW -d '"+safeLink+"'")
	return s.run(ctx, nil, args...)
}

func (s *Service) InspectPackage(ctx context.Context, serial string, packageName string) (PackageInfo, CommandResult, error) {
	pathResult, err := s.Shell(ctx, serial, "pm", "path", packageName)
	if err != nil {
		if strings.Contains(strings.ToLower(pathResult.Combined), "package "+strings.ToLower(packageName)+" was not found") {
			return PackageInfo{}, pathResult, nil
		}
		return PackageInfo{}, pathResult, err
	}
	path := ""
	for _, line := range strings.Split(pathResult.Stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package:") {
			path = strings.TrimPrefix(trimmed, "package:")
			break
		}
	}

	dumpResult, dumpErr := s.Shell(ctx, serial, "dumpsys", "package", packageName)
	if dumpErr != nil {
		return PackageInfo{
			Installed: path != "",
			Path:      path,
		}, dumpResult, nil
	}

	info := PackageInfo{
		Installed: path != "",
		Path:      path,
	}
	for _, line := range strings.Split(dumpResult.Stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "versionName=") {
			info.VersionName = strings.TrimPrefix(trimmed, "versionName=")
		}
		if strings.HasPrefix(trimmed, "versionCode=") {
			raw := strings.TrimPrefix(trimmed, "versionCode=")
			raw = strings.Fields(raw)[0]
			info.VersionCode, _ = strconv.Atoi(raw)
		}
	}
	return info, dumpResult, nil
}

func (s *Service) Shell(ctx context.Context, serial string, shellArgs ...string) (CommandResult, error) {
	args := []string{}
	if trimmed := strings.TrimSpace(serial); trimmed != "" {
		args = append(args, "-s", trimmed)
	}
	args = append(args, "shell")
	args = append(args, shellArgs...)
	return s.run(ctx, nil, args...)
}

func (s *Service) run(ctx context.Context, stdin *strings.Reader, args ...string) (CommandResult, error) {
	if stdin == nil {
		return s.runReader(ctx, nil, args...)
	}
	return s.runReader(ctx, stdin, args...)
}

func (s *Service) runReader(ctx context.Context, stdin *strings.Reader, args ...string) (CommandResult, error) {
	resolved, err := s.locator.Resolve()
	if err != nil {
		return CommandResult{}, err
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 25*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, resolved.Path, args...)
	processutil.HideConsoleWindow(cmd)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	result := CommandResult{
		Command:  redactCommand(append([]string{resolved.Path}, args...), s.redactor),
		Stdout:   s.redactor.RedactLine(stdout.String()),
		Stderr:   s.redactor.RedactLine(stderr.String()),
		Combined: s.redactor.RedactLine(strings.TrimSpace(stdout.String() + "\n" + stderr.String())),
		ExitCode: exitCode,
	}
	if err != nil {
		return result, classifyADBError(result)
	}
	return result, nil
}

func parseDevices(raw string) []Device {
	lines := strings.Split(raw, "\n")
	devices := make([]Device, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "List of devices attached") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		device := Device{
			Serial: fields[0],
			State:  fields[1],
		}
		for _, token := range fields[2:] {
			if key, value, ok := strings.Cut(token, ":"); ok {
				switch key {
				case "product":
					device.Product = value
				case "model":
					device.Model = strings.ReplaceAll(value, "_", " ")
				case "device":
					device.Device = value
				case "transport_id":
					device.TransportID = value
				}
			}
		}
		device.IsEmulator = strings.HasPrefix(device.Serial, "emulator-")
		device.HostAlias = detectEmulatorHostAlias(device)
		lower := strings.ToLower(strings.Join([]string{device.Product, device.Model, device.Device}, " "))
		device.IsWatch = strings.Contains(lower, "wear") || strings.Contains(lower, "gwear") || strings.Contains(lower, "watch")
		displayName := strings.TrimSpace(device.Model)
		if displayName == "" {
			displayName = device.Serial
		}
		device.DisplayName = displayName
		devices = append(devices, device)
	}
	return devices
}

func detectEmulatorHostAlias(device Device) string {
	if !strings.HasPrefix(device.Serial, "emulator-") {
		return ""
	}
	lower := strings.ToLower(strings.Join([]string{device.Product, device.Model, device.Device, device.Serial}, " "))
	if strings.Contains(lower, "genymotion") || strings.Contains(lower, "vbox") {
		return "10.0.3.2"
	}
	return "10.0.2.2"
}

func classifyADBError(result CommandResult) error {
	combined := strings.ToLower(result.Combined)
	switch {
	case strings.Contains(combined, "adb: command not found"):
		return errors.New("ADB 不可用")
	case strings.Contains(combined, "more than one device"):
		return errors.New("检测到多个 ADB 设备，请先选择目标手表")
	case strings.Contains(combined, "failed to authenticate"):
		return errors.New("无线调试认证失败，请重新确认配对码")
	case strings.Contains(combined, "unable to connect"):
		return errors.New("连接设备失败，请检查 IP、端口和同一 Wi‑Fi")
	case strings.Contains(combined, "device offline"):
		return errors.New("设备当前离线，请重新连接")
	case strings.Contains(combined, "unauthorized"):
		return errors.New("设备未授权 ADB 调试")
	case strings.Contains(combined, "install_failed_version_downgrade"):
		return errors.New("安装失败：设备上已有更高版本")
	case strings.Contains(combined, "install_failed_no_matching_abis"):
		return errors.New("安装失败：APK ABI 与设备不兼容")
	case strings.Contains(combined, "install_parse_failed_no_certificates"):
		return errors.New("安装失败：APK 签名异常或文件损坏")
	case strings.Contains(combined, "install_failed_update_incompatible"):
		return errors.New("安装失败：设备上已存在签名不一致的包")
	case strings.Contains(combined, "error: no devices/emulators found"):
		return errors.New("未检测到已连接的设备或模拟器")
	default:
		message := strings.TrimSpace(result.Combined)
		if message == "" {
			message = "ADB 命令执行失败"
		}
		return errors.New(message)
	}
}

func redactCommand(parts []string, redactor *logging.Redactor) string {
	joined := strings.Join(parts, " ")
	return redactor.RedactLine(joined)
}

var serialSuffixPattern = regexp.MustCompile(`:\d+$`)

func DeviceLabelForSerial(devices []Device, serial string) string {
	for _, device := range devices {
		if device.Serial == serial {
			if device.DisplayName != "" {
				return device.DisplayName
			}
			break
		}
	}
	if serialSuffixPattern.MatchString(serial) {
		return serial
	}
	return strings.TrimSpace(serial)
}

func PortFromSerial(serial string) int {
	if !serialSuffixPattern.MatchString(serial) {
		return 0
	}
	raw := serial[strings.LastIndex(serial, ":")+1:]
	port, _ := strconv.Atoi(raw)
	return port
}
