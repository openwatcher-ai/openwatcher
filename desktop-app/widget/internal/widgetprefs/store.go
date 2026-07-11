package widgetprefs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Position struct {
	MonitorID  string  `json:"monitorId"`
	Edge       string  `json:"edge"`
	Normalized float64 `json:"normalized"`
}

type Store interface {
	Load() Position
	Save(Position) error
}

type memoryStore struct{ p Position }

func NewMemoryStore() Store                  { return &memoryStore{} }
func (m *memoryStore) Load() Position        { return m.p }
func (m *memoryStore) Save(p Position) error { m.p = p; return nil }

type fileStore struct{ path string }

func NewFileStore(path string) Store { return &fileStore{path: path} }
func DefaultPath(home string) string {
	return filepath.Join(home, ".openwatcher", "widget-position.json")
}
func (s *fileStore) Load() Position {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return Position{}
	}
	var p Position
	if json.Unmarshal(b, &p) != nil || !valid(p) {
		return Position{}
	}
	return p
}
func (s *fileStore) Save(p Position) error {
	if !valid(p) {
		return errors.New("invalid widget position")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".widget-position-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.path)
}
func valid(p Position) bool {
	return p.Edge == "left" || p.Edge == "right" || p.Edge == "top" || p.Edge == "bottom" || p.Edge == ""
}
