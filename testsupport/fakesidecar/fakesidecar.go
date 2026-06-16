package fakesidecar

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type State struct {
	HealthHTTPCode int      `json:"healthHttpCode"`
	Malformed      bool     `json:"malformed"`
	Crash          bool     `json:"crash"`
	CrashMessage   string   `json:"crashMessage,omitempty"`
	StdoutLines    []string `json:"stdoutLines,omitempty"`
	StderrLines    []string `json:"stderrLines,omitempty"`
}

type CommandRecord struct {
	At            string   `json:"at"`
	Args          []string `json:"args"`
	Listen        string   `json:"listen,omitempty"`
	PublicBaseURL string   `json:"publicBaseUrl,omitempty"`
	PairingSlot   string   `json:"pairingSlot,omitempty"`
}

func BinaryName() string {
	if runtime.GOOS == "windows" {
		return "openwatcher.exe"
	}
	return "openwatcher"
}

func StatePath(binaryPath string) string {
	return filepath.Join(filepath.Dir(binaryPath), "fakesidecar-state.json")
}

func CommandsPath(binaryPath string) string {
	return filepath.Join(filepath.Dir(binaryPath), "fakesidecar-commands.jsonl")
}

func InstallBinary(t testing.TB, targetPath string) {
	t.Helper()
	source, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatalf("open test binary: %v", err)
	}
	defer source.Close()
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("mkdir fake sidecar target: %v", err)
	}
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatalf("create fake sidecar binary: %v", err)
	}
	if _, err := target.ReadFrom(source); err != nil {
		target.Close()
		t.Fatalf("copy fake sidecar binary: %v", err)
	}
	if err := target.Close(); err != nil {
		t.Fatalf("close fake sidecar binary: %v", err)
	}
}

func WriteState(t testing.TB, path string, state State) {
	t.Helper()
	writeState(path, state)
}

func ReadCommands(t testing.TB, path string) []CommandRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fake sidecar commands: %v", err)
	}
	records := []CommandRecord{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var record CommandRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("parse fake sidecar command %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func MaybeRunProcess() bool {
	base := strings.ToLower(filepath.Base(os.Args[0]))
	if base != "openwatcher" && base != "openwatcher.exe" {
		return false
	}
	os.Exit(runProcess(os.Args[1:]))
	return true
}

func runProcess(args []string) int {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake sidecar cannot resolve executable")
		return 1
	}
	state, err := readState(StatePath(exePath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake sidecar cannot read state: %v\n", err)
		return 1
	}
	cfg := parseArgs(args)
	appendRecord(CommandsPath(exePath), CommandRecord{
		At:            time.Now().UTC().Format(time.RFC3339Nano),
		Args:          append([]string(nil), args...),
		Listen:        cfg.listen,
		PublicBaseURL: cfg.publicBaseURL,
		PairingSlot:   cfg.pairingSlot,
	})
	writeConfiguredOutput(state)

	if state.Crash {
		message := strings.TrimSpace(state.CrashMessage)
		if message == "" {
			message = "address already in use"
		}
		fmt.Fprintln(os.Stderr, message)
		waitForLogCapture()
		return 1
	}

	listener, err := net.Listen("tcp", cfg.listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		waitForLogCapture()
		return 1
	}
	defer listener.Close()

	fmt.Fprintf(os.Stdout, "fake sidecar listening on %s\n", listener.Addr().String())
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		code := state.HealthHTTPCode
		if code == 0 {
			code = http.StatusOK
		}
		w.WriteHeader(code)
		if state.Malformed {
			_, _ = w.Write([]byte("{not-json"))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": code >= 200 && code < 300,
			"build": map[string]any{
				"version": "fake-sidecar",
				"commit":  "test",
				"builtAt": "2026-06-10T00:00:00Z",
			},
			"config": map[string]any{
				"listen":        cfg.listen,
				"publicBaseUrl": cfg.publicBaseURL,
				"pairingSlot":   cfg.pairingSlot,
				"paired":        true,
				"noAuth":        true,
			},
			"codex": map[string]any{
				"homeDetected":     true,
				"authDetected":     true,
				"sessionsDetected": true,
			},
		})
	})
	server := &http.Server{Handler: mux}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	return 0
}

func writeConfiguredOutput(state State) {
	for _, line := range state.StdoutLines {
		if strings.TrimSpace(line) != "" {
			fmt.Fprintln(os.Stdout, line)
		}
	}
	for _, line := range state.StderrLines {
		if strings.TrimSpace(line) != "" {
			fmt.Fprintln(os.Stderr, line)
		}
	}
}

func waitForLogCapture() {
	time.Sleep(50 * time.Millisecond)
}

type serveConfig struct {
	listen        string
	publicBaseURL string
	pairingSlot   string
}

func parseArgs(args []string) serveConfig {
	cfg := serveConfig{
		listen:        "127.0.0.1:8787",
		publicBaseURL: "http://127.0.0.1:8787",
		pairingSlot:   "beta",
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--listen":
			if i+1 < len(args) {
				cfg.listen = args[i+1]
				i++
			}
		case "--public-base-url":
			if i+1 < len(args) {
				cfg.publicBaseURL = strings.TrimRight(args[i+1], "/")
				i++
			}
		case "--pairing-slot":
			if i+1 < len(args) {
				cfg.pairingSlot = args[i+1]
				i++
			}
		}
	}
	return cfg
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
