package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	rootconfig "openwatcher/internal/config"
)

const (
	openWatcherHookSubcommand = "codex-compact-hook"
	hookMatcher               = "manual|auto"
	hookStatusMessage         = "OpenWatcher 正在记录 Codex 压缩状态"
)

type HookStatus struct {
	CodexHome      string `json:"codexHome"`
	HooksPath      string `json:"hooksPath"`
	HooksPathLabel string `json:"hooksPathLabel"`
	ReviewLocation string `json:"reviewLocation"`
	Installed      bool   `json:"installed"`
	Changed        bool   `json:"changed"`
	BackupPath     string `json:"backupPath,omitempty"`
	Command        string `json:"command,omitempty"`
	Message        string `json:"message"`
}

func InspectOpenWatcherHooks(binaryPath string) (HookStatus, error) {
	return inspectOpenWatcherHooks(binaryPath, time.Now())
}

func InstallOpenWatcherHooks(binaryPath string) (HookStatus, error) {
	return installOpenWatcherHooks(binaryPath, time.Now())
}

func HooksPath() (string, error) {
	codexHome, err := rootconfig.ResolveCodexHome("")
	if err != nil {
		return "", err
	}
	return filepath.Join(codexHome, "hooks.json"), nil
}

func inspectOpenWatcherHooks(binaryPath string, now time.Time) (HookStatus, error) {
	status, path, err := baseHookStatus(binaryPath)
	if err != nil {
		return status, err
	}
	_ = now

	root, err := readHooksRoot(path)
	if errors.Is(err, os.ErrNotExist) {
		status.Message = "尚未安装 OpenWatcher Codex hooks。"
		return status, nil
	}
	if err != nil {
		status.Message = "读取 Codex hooks.json 失败。"
		return status, err
	}

	status.Installed = hasOpenWatcherHook(root, "PreCompact") && hasOpenWatcherHook(root, "PostCompact")
	if status.Installed {
		status.Message = "OpenWatcher hooks 已写入；信任状态请在 Codex App 中确认。"
	} else {
		status.Message = "尚未安装完整的 OpenWatcher Codex hooks。"
	}
	return status, nil
}

func installOpenWatcherHooks(binaryPath string, now time.Time) (HookStatus, error) {
	status, path, err := baseHookStatus(binaryPath)
	if err != nil {
		return status, err
	}
	if strings.TrimSpace(binaryPath) == "" {
		status.Message = "未找到 OpenWatcher 本机服务二进制，暂不能安装 hooks。"
		return status, errors.New(status.Message)
	}

	root, err := readHooksRoot(path)
	if errors.Is(err, os.ErrNotExist) {
		root = map[string]any{}
	} else if err != nil {
		status.Message = "读取 Codex hooks.json 失败，未改动配置。"
		return status, err
	}

	next := cloneJSONMap(root)
	command := HookCommand(binaryPath)
	ensureOpenWatcherEvent(next, "PreCompact", command)
	ensureOpenWatcherEvent(next, "PostCompact", command)

	if jsonEqual(root, next) {
		status.Installed = true
		status.Message = "OpenWatcher hooks 已存在；信任状态请在 Codex App 中确认。"
		return status, nil
	}

	if _, err := os.Stat(path); err == nil {
		backupPath, err := backupHooksFile(path, now)
		if err != nil {
			status.Message = "备份 Codex hooks.json 失败，未改动配置。"
			return status, err
		}
		status.BackupPath = backupPath
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		status.Message = "检查 Codex hooks.json 失败，未改动配置。"
		return status, err
	}

	if err := writeHooksRoot(path, next); err != nil {
		status.Message = "写入 Codex hooks.json 失败。"
		return status, err
	}

	status.Installed = true
	status.Changed = true
	status.Message = "已写入 OpenWatcher hooks；信任状态请在 Codex App 中确认。"
	return status, nil
}

func baseHookStatus(binaryPath string) (HookStatus, string, error) {
	path, err := HooksPath()
	if err != nil {
		return HookStatus{
			HooksPathLabel: "~/.codex/hooks.json",
			ReviewLocation: "Codex App → 设置 → 钩子 → 来自用户配置",
			Message:        "无法解析 Codex Home。",
		}, "", err
	}
	codexHome := filepath.Dir(path)
	command := ""
	if strings.TrimSpace(binaryPath) != "" {
		command = HookCommand(binaryPath)
	}
	return HookStatus{
		CodexHome:      codexHome,
		HooksPath:      path,
		HooksPathLabel: hooksPathLabel(path),
		ReviewLocation: "Codex App → 设置 → 钩子 → 来自用户配置",
		Command:        command,
	}, path, nil
}

func HookCommand(binaryPath string) string {
	return quoteCommandPath(binaryPath) + " " + openWatcherHookSubcommand
}

func readHooksRoot(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if root == nil {
		root = map[string]any{}
	}
	return root, nil
}

func writeHooksRoot(path string, root map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(path, 0o600)
}

func ensureOpenWatcherEvent(root map[string]any, eventName string, command string) {
	hooksRoot, ok := root["hooks"].(map[string]any)
	if !ok || hooksRoot == nil {
		hooksRoot = map[string]any{}
		root["hooks"] = hooksRoot
	}

	groups := asSlice(hooksRoot[eventName])
	targetIndex := -1
	for i, rawGroup := range groups {
		group, ok := rawGroup.(map[string]any)
		if !ok {
			continue
		}
		group["hooks"] = removeManagedHooks(asSlice(group["hooks"]))
		if strings.TrimSpace(fmt.Sprint(group["matcher"])) == hookMatcher {
			targetIndex = i
		}
	}

	managedHook := map[string]any{
		"type":          "command",
		"command":       command,
		"timeout":       float64(5),
		"statusMessage": hookStatusMessage,
	}

	if targetIndex >= 0 {
		group := groups[targetIndex].(map[string]any)
		group["hooks"] = append(asSlice(group["hooks"]), managedHook)
	} else {
		groups = append(groups, map[string]any{
			"matcher": hookMatcher,
			"hooks":   []any{managedHook},
		})
	}
	hooksRoot[eventName] = groups
}

func hasOpenWatcherHook(root map[string]any, eventName string) bool {
	hooksRoot, ok := root["hooks"].(map[string]any)
	if !ok || hooksRoot == nil {
		return false
	}
	for _, rawGroup := range asSlice(hooksRoot[eventName]) {
		group, ok := rawGroup.(map[string]any)
		if !ok {
			continue
		}
		for _, rawHook := range asSlice(group["hooks"]) {
			hook, ok := rawHook.(map[string]any)
			if !ok {
				continue
			}
			if strings.Contains(fmt.Sprint(hook["command"]), openWatcherHookSubcommand) {
				return true
			}
		}
	}
	return false
}

func removeManagedHooks(hooks []any) []any {
	next := make([]any, 0, len(hooks))
	for _, rawHook := range hooks {
		hook, ok := rawHook.(map[string]any)
		if !ok {
			next = append(next, rawHook)
			continue
		}
		if strings.Contains(fmt.Sprint(hook["command"]), openWatcherHookSubcommand) {
			continue
		}
		next = append(next, rawHook)
	}
	return next
}

func asSlice(value any) []any {
	if value == nil {
		return nil
	}
	if items, ok := value.([]any); ok {
		return items
	}
	return nil
}

func backupHooksFile(path string, now time.Time) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	backupPath := fmt.Sprintf("%s.openwatcher-backup-%s", path, now.UTC().Format("20060102-150405"))
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return "", err
	}
	return backupPath, os.Chmod(backupPath, 0o600)
}

func cloneJSONMap(value map[string]any) map[string]any {
	data, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var cloned map[string]any
	if err := json.Unmarshal(data, &cloned); err != nil || cloned == nil {
		return map[string]any{}
	}
	return cloned
}

func jsonEqual(a map[string]any, b map[string]any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}

func quoteCommandPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if runtime.GOOS == "windows" {
		return `"` + strings.ReplaceAll(trimmed, `"`, `\"`) + `"`
	}
	return `'` + strings.ReplaceAll(trimmed, `'`, `'\''`) + `'`
}

func hooksPathLabel(path string) string {
	codexHome, err := rootconfig.ResolveCodexHome("")
	if err == nil {
		if rel, relErr := filepath.Rel(codexHome, path); relErr == nil && rel == "hooks.json" {
			if _, ok := os.LookupEnv("CODEX_HOME"); ok {
				return filepath.Join("CODEX_HOME", "hooks.json")
			}
			return "~/.codex/hooks.json"
		}
	}
	return path
}
