package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type DesktopSettings struct {
	AutoStartBackend bool `json:"autoStartBackend"`
}

func DefaultDesktopSettings() DesktopSettings {
	return DesktopSettings{
		AutoStartBackend: true,
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
	payload, err := json.MarshalIndent(DesktopSettings{
		AutoStartBackend: value.AutoStartBackend,
	}, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return writeFileAtomically(path, payload, 0o600)
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
