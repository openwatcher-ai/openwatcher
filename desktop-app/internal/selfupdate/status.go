package selfupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Status struct {
	Phase      string `json:"phase"`
	Message    string `json:"message"`
	Version    string `json:"version,omitempty"`
	Artifact   string `json:"artifact,omitempty"`
	BackupPath string `json:"backupPath,omitempty"`
	UpdatedAt  string `json:"updatedAt"`
}

func NewStatus(phase string, message string, version string, artifact string, backupPath string) Status {
	return Status{
		Phase:      phase,
		Message:    message,
		Version:    version,
		Artifact:   artifact,
		BackupPath: backupPath,
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
}

func ReadStatus(path string) (Status, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Status{}, err
	}
	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		return Status{}, err
	}
	return status, nil
}

func WriteStatus(path string, status Status) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
