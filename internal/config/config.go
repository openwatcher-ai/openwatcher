package config

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultListen              = "127.0.0.1:8787"
	DefaultQuotaRefreshSeconds = 60
	DefaultActiveSessionLimit  = 5
	DefaultApkDistDir          = "dist"
	DefaultDevUpdateAllowlist  = ".tmp/openwatcher-dev-update-allowlist.txt"
	DefaultScreenshotUploadDir = "~/.openwatcher/screenshots"
	DefaultDiagnosticUploadDir = "~/.openwatcher/diagnostics"
)

type PairingSlot string

const (
	PairingSlotBeta PairingSlot = "beta"
	PairingSlotDev  PairingSlot = "dev"
)

type PairingBinding struct {
	TokenHash  string `json:"tokenHash,omitempty"`
	DeviceName string `json:"deviceName,omitempty"`
	PairedAt   string `json:"pairedAt,omitempty"`
}

type Config struct {
	Listen              string          `json:"listen"`
	PublicBaseURL       string          `json:"publicBaseUrl,omitempty"`
	CodexHome           string          `json:"codexHome,omitempty"`
	ApkDistDir          string          `json:"apkDistDir,omitempty"`
	DevUpdateAllowlist  string          `json:"devUpdateAllowlist,omitempty"`
	ScreenshotUploadDir string          `json:"screenshotUploadDir,omitempty"`
	DiagnosticUploadDir string          `json:"diagnosticUploadDir,omitempty"`
	WidgetTokenHash     string          `json:"widgetTokenHash,omitempty"`
	BetaPairing         *PairingBinding `json:"betaPairing,omitempty"`
	DevPairing          *PairingBinding `json:"devPairing,omitempty"`
	TokenHash           string          `json:"tokenHash,omitempty"`
	DeviceName          string          `json:"deviceName,omitempty"`
	PairedAt            string          `json:"pairedAt,omitempty"`
	QuotaRefreshSeconds int             `json:"quotaRefreshSeconds"`
	ActiveSessionLimit  int             `json:"activeSessionLimit"`
}

func Load(path string) (Config, string, error) {
	resolved, err := ResolvePath(path)
	if err != nil {
		return Config{}, "", err
	}

	cfg := Config{}
	data, err := os.ReadFile(resolved)
	if errors.Is(err, os.ErrNotExist) {
		cfg.ApplyDefaults()
		return cfg, resolved, nil
	}
	if err != nil {
		return Config{}, "", err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, "", err
	}
	cfg.ApplyDefaults()
	return cfg, resolved, nil
}

func ResolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	if envPath := os.Getenv("OPENWATCHER_CONFIG"); envPath != "" {
		return envPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".openwatcher", "config.json"), nil
}

func ResolvePathOrEmpty() string {
	path, err := ResolvePath("")
	if err != nil {
		return ""
	}
	return path
}

func (c *Config) ApplyDefaults() {
	c.syncPairingBindings()
	if c.Listen == "" {
		c.Listen = DefaultListen
	}
	c.PublicBaseURL = NormalizePublicBaseURL(c.PublicBaseURL)
	if c.QuotaRefreshSeconds <= 0 {
		c.QuotaRefreshSeconds = DefaultQuotaRefreshSeconds
	}
	if c.ActiveSessionLimit <= 0 {
		c.ActiveSessionLimit = DefaultActiveSessionLimit
	}
	if c.ApkDistDir == "" {
		c.ApkDistDir = DefaultApkDistDir
	}
	if c.DevUpdateAllowlist == "" {
		c.DevUpdateAllowlist = DefaultDevUpdateAllowlist
	}
	if c.ScreenshotUploadDir == "" {
		c.ScreenshotUploadDir = DefaultScreenshotUploadDir
	}
	if c.DiagnosticUploadDir == "" {
		c.DiagnosticUploadDir = DefaultDiagnosticUploadDir
	}
}

func NormalizePairingSlot(raw string) PairingSlot {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(PairingSlotDev):
		return PairingSlotDev
	default:
		return PairingSlotBeta
	}
}

func (c Config) TokenHashForSlot(slot PairingSlot) string {
	return c.PairingForSlot(slot).TokenHash
}

func (c Config) PairingForSlot(slot PairingSlot) PairingBinding {
	switch NormalizePairingSlot(string(slot)) {
	case PairingSlotDev:
		return normalizePairingBinding(c.DevPairing)
	default:
		binding := normalizePairingBinding(c.BetaPairing)
		if binding.isEmpty() {
			return normalizePairingBinding(&PairingBinding{
				TokenHash:  c.TokenHash,
				DeviceName: c.DeviceName,
				PairedAt:   c.PairedAt,
			})
		}
		return binding
	}
}

func (c *Config) SetPairingForSlot(slot PairingSlot, tokenHash, deviceName, pairedAt string) {
	binding := normalizePairingBinding(&PairingBinding{
		TokenHash:  tokenHash,
		DeviceName: deviceName,
		PairedAt:   pairedAt,
	})
	switch NormalizePairingSlot(string(slot)) {
	case PairingSlotDev:
		c.DevPairing = binding.pointer()
	default:
		c.BetaPairing = binding.pointer()
		c.TokenHash = binding.TokenHash
		c.DeviceName = binding.DeviceName
		c.PairedAt = binding.PairedAt
	}
}

func (c *Config) ClearPairingForSlot(slot PairingSlot) {
	switch NormalizePairingSlot(string(slot)) {
	case PairingSlotDev:
		c.DevPairing = nil
	default:
		c.BetaPairing = nil
		c.TokenHash = ""
		c.DeviceName = ""
		c.PairedAt = ""
	}
}

func (c *Config) ClearAllPairings() {
	c.BetaPairing = nil
	c.DevPairing = nil
	c.TokenHash = ""
	c.DeviceName = ""
	c.PairedAt = ""
}

func (c *Config) syncPairingBindings() {
	beta := normalizePairingBinding(c.BetaPairing)
	if beta.isEmpty() {
		beta = normalizePairingBinding(&PairingBinding{
			TokenHash:  c.TokenHash,
			DeviceName: c.DeviceName,
			PairedAt:   c.PairedAt,
		})
	}
	c.BetaPairing = beta.pointer()
	c.DevPairing = normalizePairingBinding(c.DevPairing).pointer()

	if !beta.isEmpty() {
		c.TokenHash = beta.TokenHash
		c.DeviceName = beta.DeviceName
		c.PairedAt = beta.PairedAt
	}
}

func NormalizePublicBaseURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func DefaultPublicBaseURL(listen string) string {
	host, port, err := net.SplitHostPort(strings.TrimSpace(listen))
	if err != nil {
		return "http://" + DefaultListen
	}
	switch strings.Trim(host, "[]") {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func (c Config) EffectivePublicBaseURL() string {
	if trimmed := NormalizePublicBaseURL(c.PublicBaseURL); trimmed != "" {
		return trimmed
	}
	return DefaultPublicBaseURL(c.Listen)
}

func Save(path string, cfg Config) error {
	cfg.ApplyDefaults()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
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

func ResolveCodexHome(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	if envHome := os.Getenv("CODEX_HOME"); envHome != "" {
		return envHome, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}

func ResolveUserHome() (string, error) {
	return os.UserHomeDir()
}

func normalizePairingBinding(binding *PairingBinding) PairingBinding {
	if binding == nil {
		return PairingBinding{}
	}
	return PairingBinding{
		TokenHash:  strings.TrimSpace(binding.TokenHash),
		DeviceName: strings.TrimSpace(binding.DeviceName),
		PairedAt:   strings.TrimSpace(binding.PairedAt),
	}
}

func (b PairingBinding) isEmpty() bool {
	return strings.TrimSpace(b.TokenHash) == "" &&
		strings.TrimSpace(b.DeviceName) == "" &&
		strings.TrimSpace(b.PairedAt) == ""
}

func (b PairingBinding) pointer() *PairingBinding {
	if b.isEmpty() {
		return nil
	}
	copy := b
	return &copy
}
