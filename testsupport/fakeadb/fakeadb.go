package fakeadb

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type State struct {
	Serial      string        `json:"serial"`
	Product     string        `json:"product"`
	Model       string        `json:"model"`
	Device      string        `json:"device"`
	TransportID string        `json:"transportId"`
	Devices     []DeviceEntry `json:"devices,omitempty"`

	Connected    bool   `json:"connected"`
	Installed    bool   `json:"installed"`
	PackageName  string `json:"packageName"`
	VersionName  string `json:"versionName"`
	VersionCode  int    `json:"versionCode"`
	APKPath      string `json:"apkPath,omitempty"`
	LastDeepLink string `json:"lastDeepLink,omitempty"`

	FailInstall bool `json:"failInstall"`
	FailLaunch  bool `json:"failLaunch"`

	PairFailure          string `json:"pairFailure,omitempty"`
	ConnectFailure       string `json:"connectFailure,omitempty"`
	ConnectWithoutDevice bool   `json:"connectWithoutDevice,omitempty"`
	InstallFailure       string `json:"installFailure,omitempty"`
	MonkeyFailure        string `json:"monkeyFailure,omitempty"`
	DeepLinkFailure      string `json:"deepLinkFailure,omitempty"`
	DumpsysFailure       string `json:"dumpsysFailure,omitempty"`
}

type DeviceEntry struct {
	Serial      string `json:"serial"`
	State       string `json:"state"`
	Product     string `json:"product,omitempty"`
	Model       string `json:"model,omitempty"`
	Device      string `json:"device,omitempty"`
	TransportID string `json:"transportId,omitempty"`
}

type CommandRecord struct {
	At            string   `json:"at"`
	Args          []string `json:"args"`
	Operation     string   `json:"operation"`
	Serial        string   `json:"serial,omitempty"`
	APKPath       string   `json:"apkPath,omitempty"`
	DeepLink      string   `json:"deepLink,omitempty"`
	StdinProvided bool     `json:"stdinProvided"`
}

func BinaryName() string {
	if runtime.GOOS == "windows" {
		return "adb.exe"
	}
	return "adb"
}

func StatePath(binaryPath string) string {
	return filepath.Join(filepath.Dir(binaryPath), "fakeadb-state.json")
}

func CommandsPath(binaryPath string) string {
	return filepath.Join(filepath.Dir(binaryPath), "fakeadb-commands.jsonl")
}

func InstallBinary(t testing.TB, targetPath string) {
	t.Helper()
	source, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatalf("open test binary: %v", err)
	}
	defer source.Close()
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("mkdir fake adb target: %v", err)
	}
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatalf("create fake adb binary: %v", err)
	}
	if _, err := target.ReadFrom(source); err != nil {
		target.Close()
		t.Fatalf("copy fake adb binary: %v", err)
	}
	if err := target.Close(); err != nil {
		t.Fatalf("close fake adb binary: %v", err)
	}
}

func WriteState(t testing.TB, path string, state State) {
	t.Helper()
	writeState(path, state)
}

func ReadState(t testing.TB, path string) State {
	t.Helper()
	state, err := readState(path)
	if err != nil {
		t.Fatalf("read fake adb state: %v", err)
	}
	return state
}

func ReadCommands(t testing.TB, path string) []CommandRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fake adb commands: %v", err)
	}
	records := []CommandRecord{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var record CommandRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("parse fake adb command %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func MaybeRunProcess() bool {
	base := strings.ToLower(filepath.Base(os.Args[0]))
	if base != "adb" && base != "adb.exe" {
		return false
	}
	os.Exit(runProcess(os.Args[1:]))
	return true
}

func runProcess(args []string) int {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake adb cannot resolve executable")
		return 1
	}
	statePath := StatePath(exePath)
	state, err := readState(statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake adb cannot read state: %v\n", err)
		return 1
	}
	stdin, _ := io.ReadAll(os.Stdin)
	record := commandRecord(args, strings.TrimSpace(string(stdin)) != "")
	appendRecord(CommandsPath(exePath), record)

	if len(args) == 0 {
		return fail("missing adb args")
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(os.Stdout, "Android Debug Bridge version 1.0.41")
		fmt.Fprintln(os.Stdout, "Version 35.0.2-12147458")
		return 0
	case "devices":
		printDevices(state)
		return 0
	case "pair":
		if state.PairFailure != "" {
			return handlePairFailure(state.PairFailure, lastArg(args))
		}
		fmt.Fprintf(os.Stdout, "Successfully paired to %s [guid=fake-guid]\n", lastArg(args))
		return 0
	case "connect":
		if state.ConnectFailure != "" {
			return handleConnectFailure(state.ConnectFailure, lastArg(args))
		}
		state.Connected = !state.ConnectWithoutDevice
		if state.ConnectWithoutDevice {
			state.Devices = nil
		}
		writeState(statePath, state)
		fmt.Fprintf(os.Stdout, "connected to %s\n", lastArg(args))
		return 0
	}

	serial, rest := consumeSerial(args)
	if len(rest) == 0 {
		return fail("missing adb command after serial")
	}
	switch rest[0] {
	case "install":
		return handleInstall(statePath, state, serial, rest)
	case "shell":
		return handleShell(statePath, state, serial, rest[1:])
	default:
		return fail("unsupported adb command: " + strings.Join(args, " "))
	}
}

func printDevices(state State) {
	fmt.Fprintln(os.Stdout, "List of devices attached")
	for _, entry := range deviceEntries(state) {
		model := strings.ReplaceAll(firstNonBlank(entry.Model, "OpenWatcher_Headless"), " ", "_")
		fmt.Fprintf(
			os.Stdout,
			"%s %s product:%s model:%s device:%s transport_id:%s\n",
			firstNonBlank(entry.Serial, "192.168.1.33:40221"),
			firstNonBlank(entry.State, "device"),
			firstNonBlank(entry.Product, "watch"),
			model,
			firstNonBlank(entry.Device, "watch"),
			firstNonBlank(entry.TransportID, "9"),
		)
	}
}

func deviceEntries(state State) []DeviceEntry {
	if len(state.Devices) > 0 {
		return state.Devices
	}
	if !state.Connected {
		return nil
	}
	return []DeviceEntry{{
		Serial:      firstNonBlank(state.Serial, "192.168.1.33:40221"),
		State:       "device",
		Product:     firstNonBlank(state.Product, "watch"),
		Model:       firstNonBlank(state.Model, "OpenWatcher_Headless"),
		Device:      firstNonBlank(state.Device, "watch"),
		TransportID: firstNonBlank(state.TransportID, "9"),
	}}
}

func handlePairFailure(kind string, target string) int {
	switch kind {
	case "auth":
		fmt.Fprintf(os.Stderr, "failed to authenticate to %s\n", target)
	case "bad_code":
		fmt.Fprintln(os.Stderr, "Failed: Wrong pairing code")
	default:
		fmt.Fprintf(os.Stderr, "Failed: pair failed for %s\n", target)
	}
	return 1
}

func handleConnectFailure(kind string, target string) int {
	switch kind {
	case "unable_to_connect":
		fmt.Fprintf(os.Stderr, "unable to connect to %s: connection refused\n", target)
	default:
		fmt.Fprintf(os.Stderr, "failed to connect to %s\n", target)
	}
	return 1
}

func handleInstall(statePath string, state State, serial string, args []string) int {
	switch {
	case state.FailInstall || state.InstallFailure == "version_downgrade":
		fmt.Fprintln(os.Stderr, "Failure [INSTALL_FAILED_VERSION_DOWNGRADE]")
		return 1
	case state.InstallFailure == "update_incompatible":
		fmt.Fprintf(os.Stderr, "Failure [INSTALL_FAILED_UPDATE_INCOMPATIBLE: Package %s signatures do not match]\n", packageName(state))
		return 1
	case state.InstallFailure == "generic":
		fmt.Fprintln(os.Stderr, "adb: failed to install apk: Failure [INSTALL_FAILED_INTERNAL_ERROR]")
		return 1
	}
	apkPath := lastArg(args)
	state.Connected = true
	state.Installed = true
	state.APKPath = apkPath
	writeState(statePath, state)
	fmt.Fprintln(os.Stdout, "Success")
	return 0
}

func handleShell(statePath string, state State, serial string, args []string) int {
	shell := strings.Join(args, " ")
	switch {
	case strings.Contains(shell, "pm path"):
		if !state.Installed {
			fmt.Fprintf(os.Stderr, "package %s was not found\n", packageName(state))
			return 1
		}
		fmt.Fprintf(os.Stdout, "package:/data/app/%s/base.apk\n", packageName(state))
		return 0
	case strings.Contains(shell, "dumpsys package"):
		if !state.Installed {
			fmt.Fprintf(os.Stderr, "Unable to find package: %s\n", packageName(state))
			return 1
		}
		if state.DumpsysFailure != "" {
			fmt.Fprintf(os.Stderr, "Unable to read package version for %s\n", packageName(state))
			return 1
		}
		fmt.Fprintf(os.Stdout, "Packages:\n  Package [%s]\n    versionName=%s\n    versionCode=%d minSdk=34 targetSdk=34\n", packageName(state), firstNonBlank(state.VersionName, "0.1.0"), firstNonZero(state.VersionCode, 10000))
		return 0
	case strings.Contains(shell, "monkey"):
		if state.MonkeyFailure == "package_not_found" {
			fmt.Fprintf(os.Stderr, "monkey: unknown package: %s\n", packageName(state))
			return 1
		}
		if state.FailLaunch {
			fmt.Fprintln(os.Stderr, "No activities found to run, monkey aborted.")
			return 1
		}
		fmt.Fprintln(os.Stdout, "Events injected: 1")
		return 0
	case strings.Contains(shell, "am start"):
		if state.DeepLinkFailure != "" {
			fmt.Fprintln(os.Stderr, "Error: Activity not started, unable to resolve Intent")
			return 1
		}
		deepLink := extractDeepLink(shell)
		state.LastDeepLink = deepLink
		writeState(statePath, state)
		fmt.Fprintln(os.Stdout, "Starting: Intent { act=android.intent.action.VIEW dat="+deepLink+" }")
		fmt.Fprintln(os.Stdout, "Status: ok")
		return 0
	default:
		return fail("unsupported shell command: " + shell)
	}
}

func commandRecord(args []string, stdinProvided bool) CommandRecord {
	serial, rest := consumeSerial(args)
	record := CommandRecord{
		At:            time.Now().UTC().Format(time.RFC3339Nano),
		Args:          append([]string(nil), args...),
		Serial:        serial,
		StdinProvided: stdinProvided,
	}
	if len(rest) == 0 {
		return record
	}
	if rest[0] == "shell" {
		shell := strings.Join(rest[1:], " ")
		switch {
		case strings.Contains(shell, "am start"):
			record.Operation = "shell am start"
			record.DeepLink = extractDeepLink(shell)
		case strings.Contains(shell, "monkey"):
			record.Operation = "shell monkey"
		case strings.Contains(shell, "pm path"):
			record.Operation = "shell pm path"
		case strings.Contains(shell, "dumpsys package"):
			record.Operation = "shell dumpsys package"
		default:
			record.Operation = "shell"
		}
		return record
	}
	record.Operation = strings.Join(rest, " ")
	if rest[0] == "install" {
		record.APKPath = lastArg(rest)
	}
	return record
}

func consumeSerial(args []string) (string, []string) {
	if len(args) >= 2 && args[0] == "-s" {
		return args[1], args[2:]
	}
	return "", args
}

func extractDeepLink(shell string) string {
	for _, marker := range []string{"-d '", "-d \""} {
		idx := strings.Index(shell, marker)
		if idx < 0 {
			continue
		}
		value := shell[idx+len(marker):]
		quote := marker[len(marker)-1]
		if end := strings.IndexRune(value, rune(quote)); end >= 0 {
			value = value[:end]
		}
		return value
	}
	return ""
}

func readState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	state := State{}
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func writeState(path string, state State) {
	if state.Serial == "" {
		state.Serial = "192.168.1.33:40221"
	}
	if state.PackageName == "" {
		state.PackageName = "ai.openwatcher.watchapp"
	}
	if state.VersionName == "" {
		state.VersionName = "0.1.0"
	}
	if state.VersionCode == 0 {
		state.VersionCode = 10000
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		panic(err)
	}
}

func appendRecord(path string, record CommandRecord) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	payload, err := json.Marshal(record)
	if err != nil {
		panic(err)
	}
	_, _ = file.Write(append(payload, '\n'))
}

func packageName(state State) string {
	return firstNonBlank(state.PackageName, "ai.openwatcher.watchapp")
}

func lastArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[len(args)-1]
}

func fail(message string) int {
	fmt.Fprintln(os.Stderr, message)
	return 1
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
