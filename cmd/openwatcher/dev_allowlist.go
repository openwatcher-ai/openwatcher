package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"openwatcher/internal/config"
	"openwatcher/internal/pairing"
)

type devAllowlistCommand struct {
	stdout io.Writer
	stderr io.Writer
}

func newDevAllowlistCommand() devAllowlistCommand {
	return devAllowlistCommand{
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

func (c devAllowlistCommand) maybeRun(args []string) (bool, int) {
	if len(args) < 2 || args[1] != "dev-allowlist" {
		return false, 0
	}
	if len(args) < 3 {
		c.printUsage()
		return true, 2
	}
	switch args[2] {
	case "list":
		return true, c.runList(args[3:])
	case "add":
		return true, c.runAdd(args[3:])
	default:
		c.printUsage()
		return true, 2
	}
}

func (c devAllowlistCommand) runList(args []string) int {
	fs := flag.NewFlagSet("dev-allowlist list", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	var configPath string
	fs.StringVar(&configPath, "config", "", "OpenWatcher 配置文件路径")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, resolvedConfigPath, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(c.stderr, "读取配置失败：%v\n", err)
		return 1
	}
	records, err := c.knownBindings(resolvedConfigPath, cfg)
	if err != nil {
		fmt.Fprintf(c.stderr, "读取绑定历史失败：%v\n", err)
		return 1
	}
	allowlistPath := pairing.ResolveRelativeToConfig(resolvedConfigPath, cfg.DevUpdateAllowlist)
	allowlist, err := pairing.LoadAllowlist(allowlistPath)
	if err != nil {
		fmt.Fprintf(c.stderr, "读取 dev 白名单失败：%v\n", err)
		return 1
	}
	fmt.Fprintf(c.stdout, "dev 白名单文件：%s\n", allowlistPath)
	if len(records) == 0 {
		fmt.Fprintln(c.stdout, "暂无已记录的绑定设备。")
		return 0
	}
	for index, record := range records {
		status := "未加入白名单"
		if containsHash(allowlist, record.TokenHash) {
			status = "已在白名单"
		}
		pairedAt := strings.TrimSpace(record.PairedAt)
		if pairedAt == "" {
			pairedAt = "--"
		}
		deviceName := strings.TrimSpace(record.DeviceName)
		if deviceName == "" {
			deviceName = "未命名设备"
		}
		source := strings.TrimSpace(record.Source)
		if source == "" {
			source = "unknown"
		}
		fmt.Fprintf(
			c.stdout,
			"%d. %s | %s | %s | %s | %s\n",
			index+1,
			deviceName,
			record.TokenHash,
			pairedAt,
			source,
			status,
		)
	}
	return 0
}

func (c devAllowlistCommand) runAdd(args []string) int {
	fs := flag.NewFlagSet("dev-allowlist add", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	var configPath string
	var index int
	var hash string
	fs.StringVar(&configPath, "config", "", "OpenWatcher 配置文件路径")
	fs.IntVar(&index, "index", 0, "通过 list 输出中的序号加入白名单")
	fs.StringVar(&hash, "hash", "", "直接加入指定的 token hash")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if index <= 0 && strings.TrimSpace(hash) == "" {
		fmt.Fprintln(c.stderr, "需要提供 --index 或 --hash")
		return 2
	}
	cfg, resolvedConfigPath, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(c.stderr, "读取配置失败：%v\n", err)
		return 1
	}
	selectedHash := strings.TrimSpace(hash)
	if index > 0 {
		records, err := c.knownBindings(resolvedConfigPath, cfg)
		if err != nil {
			fmt.Fprintf(c.stderr, "读取绑定历史失败：%v\n", err)
			return 1
		}
		if index > len(records) {
			fmt.Fprintf(c.stderr, "序号超出范围：%d\n", index)
			return 2
		}
		selectedHash = records[index-1].TokenHash
	}
	allowlistPath := pairing.ResolveRelativeToConfig(resolvedConfigPath, cfg.DevUpdateAllowlist)
	if err := pairing.AddAllowlistTokenHash(allowlistPath, selectedHash); err != nil {
		fmt.Fprintf(c.stderr, "写入 dev 白名单失败：%v\n", err)
		return 1
	}
	fmt.Fprintf(c.stdout, "已加入 dev 白名单：%s\n", selectedHash)
	fmt.Fprintf(c.stdout, "白名单文件：%s\n", allowlistPath)
	return 0
}

func (c devAllowlistCommand) knownBindings(resolvedConfigPath string, cfg config.Config) ([]pairing.BindingRecord, error) {
	historyPath := pairing.HistoryPath(resolvedConfigPath)
	records, err := pairing.LoadHistory(historyPath)
	if err != nil {
		return nil, err
	}
	currentBindings := []struct {
		slot    config.PairingSlot
		binding config.PairingBinding
	}{
		{slot: config.PairingSlotBeta, binding: cfg.PairingForSlot(config.PairingSlotBeta)},
		{slot: config.PairingSlotDev, binding: cfg.PairingForSlot(config.PairingSlotDev)},
	}
	for _, item := range currentBindings {
		currentHash := strings.TrimSpace(item.binding.TokenHash)
		if currentHash == "" {
			continue
		}
		found := false
		for _, record := range records {
			if record.TokenHash == currentHash {
				found = true
				break
			}
		}
		if !found {
			records = append([]pairing.BindingRecord{{
				TokenHash:  currentHash,
				DeviceName: strings.TrimSpace(item.binding.DeviceName),
				PairedAt:   strings.TrimSpace(item.binding.PairedAt),
				Source:     "current-config-" + string(item.slot),
			}}, records...)
		}
	}
	return records, nil
}

func (c devAllowlistCommand) printUsage() {
	fmt.Fprintln(c.stderr, "用法：")
	fmt.Fprintln(c.stderr, "  openwatcher dev-allowlist list [--config /path/to/config.json]")
	fmt.Fprintln(c.stderr, "  openwatcher dev-allowlist add --index N [--config /path/to/config.json]")
	fmt.Fprintln(c.stderr, "  openwatcher dev-allowlist add --hash <token-hash> [--config /path/to/config.json]")
}

func containsHash(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}
