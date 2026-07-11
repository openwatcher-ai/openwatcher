package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type DesktopSettings struct {
	AutoStartBackend      bool                         `json:"autoStartBackend"`
	FloatingWidgetEnabled bool                         `json:"floatingWidgetEnabled"`
	DeveloperEnvironment  DeveloperEnvironmentSettings `json:"developerEnvironment"`
}

type DeveloperEnvironmentSettings struct {
	Enabled              bool   `json:"enabled"`
	Mode                 string `json:"mode,omitempty"`
	RepoPath             string `json:"repoPath,omitempty"`
	BaseURL              string `json:"baseUrl,omitempty"`
	DeviceName           string `json:"deviceName,omitempty"`
	HostAlias            string `json:"hostAlias,omitempty"`
	ManagedTunnelEnabled bool   `json:"managedTunnelEnabled,omitempty"`
}

func DefaultDesktopSettings() DesktopSettings {
	return DesktopSettings{
		AutoStartBackend:      true,
		FloatingWidgetEnabled: true,
		DeveloperEnvironment: DeveloperEnvironmentSettings{
			Enabled:    false,
			Mode:       "workspace",
			BaseURL:    "http://10.0.2.2:18787",
			DeviceName: "watch",
			HostAlias:  "10.0.2.2",
		},
	}
}

func LoadDesktopSettings() (DesktopSettings, error) {
	defaults := DefaultDesktopSettings()
	path, err := desktopSettingsPath()
	if err != nil {
		return defaults, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	loaded := defaults
	if err := json.Unmarshal(data, &loaded); err != nil {
		return defaults, err
	}
	// A present file without this field is an upgrade, not a new install.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return defaults, err
	}
	if _, ok := raw["floatingWidgetEnabled"]; !ok {
		loaded.FloatingWidgetEnabled = false
	}
	return loaded, nil
}

func SaveDesktopSettings(value DesktopSettings) error {
	path, err := desktopSettingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(normalizeDesktopSettings(value), "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return writeFileAtomically(path, payload, 0o600)
}

func normalizeDesktopSettings(value DesktopSettings) DesktopSettings {
	defaults := DefaultDesktopSettings()
	if value.DeveloperEnvironment.Mode == "" {
		value.DeveloperEnvironment.Mode = defaults.DeveloperEnvironment.Mode
	}
	if value.DeveloperEnvironment.BaseURL == "" {
		value.DeveloperEnvironment.BaseURL = defaults.DeveloperEnvironment.BaseURL
	}
	if value.DeveloperEnvironment.DeviceName == "" {
		value.DeveloperEnvironment.DeviceName = defaults.DeveloperEnvironment.DeviceName
	}
	if value.DeveloperEnvironment.HostAlias == "" {
		value.DeveloperEnvironment.HostAlias = defaults.DeveloperEnvironment.HostAlias
	}
	return value
}

func desktopSettingsPath() (string, error) {
	configDir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "desktop-settings.json"), nil
}

func writeFileAtomically(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmpPath := filepath.Join(dir, "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmpPath, data, mode); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
