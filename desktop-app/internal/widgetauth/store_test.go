package widgetauth

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"openwatcher/internal/config"
	"openwatcher/internal/pairing"
)

func TestEnsureGeneratesSecretAndOnlyPersistsHash(t *testing.T) {
	s := &MemoryStore{}
	p := filepath.Join(t.TempDir(), "config.json")
	var sync ConfigSynchronizer
	if err := sync.Ensure(s, p); err != nil {
		t.Fatal(err)
	}
	token, err := s.Read()
	if err != nil || len(token) != 43 {
		t.Fatalf("token=%q err=%v", token, err)
	}
	cfg, _, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WidgetTokenHash != pairing.HashToken(token) {
		t.Fatal("hash was not synchronized")
	}
	data, _ := os.ReadFile(p)
	if bytes.Contains(data, []byte(token)) {
		t.Fatal("raw token persisted")
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestEnsureRejectsCorruptedStoredCredentialWithoutRotation(t *testing.T) {
	store := &recordingStore{token: "not-a-valid-token"}
	var synchronizer ConfigSynchronizer
	err := synchronizer.Ensure(store, filepath.Join(t.TempDir(), "config.json"))
	if !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("Ensure err = %v, want invalid credential", err)
	}
	if store.writeCount != 0 || store.token != "not-a-valid-token" {
		t.Fatalf("corrupted credential was rotated: %+v", store)
	}
}

func TestEnsurePreservesExistingConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	existing := config.Config{
		Listen:              "0.0.0.0:9012",
		PublicBaseURL:       "https://example.test",
		CodexHome:           "/tmp/codex-home",
		QuotaRefreshSeconds: 17,
		ActiveSessionLimit:  9,
	}
	if err := config.Save(path, existing); err != nil {
		t.Fatal(err)
	}
	store := &MemoryStore{}
	var synchronizer ConfigSynchronizer
	if err := synchronizer.Ensure(store, path); err != nil {
		t.Fatal(err)
	}
	got, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Listen != existing.Listen || got.PublicBaseURL != existing.PublicBaseURL || got.CodexHome != existing.CodexHome ||
		got.QuotaRefreshSeconds != existing.QuotaRefreshSeconds || got.ActiveSessionLimit != existing.ActiveSessionLimit {
		t.Fatalf("existing config changed: got %+v, want fields from %+v", got, existing)
	}
}

func TestEnsureSerializesAcrossSynchronizerInstances(t *testing.T) {
	store := &MemoryStore{}
	path := filepath.Join(t.TempDir(), "config.json")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var synchronizer ConfigSynchronizer
			errs <- synchronizer.Ensure(store, path)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	token, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WidgetTokenHash != pairing.HashToken(token) {
		t.Fatal("final hash does not match the sole stored credential")
	}
}

func TestRotateExplicitlyReplacesCorruptedCredential(t *testing.T) {
	store := &recordingStore{token: "corrupted"}
	path := filepath.Join(t.TempDir(), "config.json")
	var synchronizer ConfigSynchronizer
	if err := synchronizer.Rotate(store, path); err != nil {
		t.Fatal(err)
	}
	if store.token == "corrupted" || ValidateToken(store.token) != nil {
		t.Fatalf("credential was not repaired: %q", store.token)
	}
	cfg, _, err := config.Load(path)
	if err != nil || cfg.WidgetTokenHash != pairing.HashToken(store.token) {
		t.Fatalf("rotated hash mismatch: %+v, %v", cfg, err)
	}
}

func TestRotateRestoresPreviousCredentialWhenConfigWriteFails(t *testing.T) {
	store := &MemoryStore{}
	previous, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Write(previous); err != nil {
		t.Fatal(err)
	}
	var synchronizer ConfigSynchronizer
	if err := synchronizer.Rotate(store, t.TempDir()); err == nil {
		t.Fatal("Rotate unexpectedly succeeded with a directory config path")
	}
	got, err := store.Read()
	if err != nil || got != previous {
		t.Fatalf("previous credential was not restored: %q, %v", got, err)
	}
}

func TestResetReplacesCredentialWithoutReadingOldValue(t *testing.T) {
	store := &deleteFirstStore{token: "inaccessible"}
	path := filepath.Join(t.TempDir(), "config.json")
	var synchronizer ConfigSynchronizer
	if err := synchronizer.Reset(store, path); err != nil {
		t.Fatal(err)
	}
	if store.readCount != 0 || store.deleteCount != 1 || ValidateToken(store.token) != nil {
		t.Fatalf("reset store = %+v", store)
	}
	cfg, _, err := config.Load(path)
	if err != nil || cfg.WidgetTokenHash != pairing.HashToken(store.token) {
		t.Fatalf("reset hash mismatch: %+v, %v", cfg, err)
	}
}

func TestResetDeletesNewCredentialWhenConfigWriteFails(t *testing.T) {
	store := &deleteFirstStore{token: "inaccessible"}
	var synchronizer ConfigSynchronizer
	if err := synchronizer.Reset(store, t.TempDir()); err == nil {
		t.Fatal("Reset unexpectedly succeeded with a directory config path")
	}
	if store.readCount != 0 || store.deleteCount != 2 || store.token != "" {
		t.Fatalf("failed reset left credential behind: %+v", store)
	}
}

type recordingStore struct {
	token      string
	writeCount int
}

func (s *recordingStore) Read() (string, error) {
	if s.token == "" {
		return "", ErrNotFound
	}
	return s.token, nil
}
func (s *recordingStore) Write(token string) error {
	s.writeCount++
	s.token = token
	return nil
}
func (s *recordingStore) Delete() error { s.token = ""; return nil }

type deleteFirstStore struct {
	token       string
	readCount   int
	deleteCount int
}

func (s *deleteFirstStore) Read() (string, error) {
	s.readCount++
	return "", errors.New("protected value must not be read during reset")
}
func (s *deleteFirstStore) Write(token string) error { s.token = token; return nil }
func (s *deleteFirstStore) Delete() error {
	s.deleteCount++
	s.token = ""
	return nil
}
