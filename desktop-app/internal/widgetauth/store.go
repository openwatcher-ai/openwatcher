// Package widgetauth owns the local-only credential used by the Desktop widget.
package widgetauth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"

	"openwatcher/internal/config"
	"openwatcher/internal/pairing"
)

var (
	ErrNotFound          = errors.New("未找到悬浮球凭据")
	ErrUnsupported       = errors.New("当前平台不支持悬浮球凭据存储")
	ErrInvalidCredential = errors.New("悬浮球凭据无效")
	configMu             sync.Mutex
)

const (
	widgetCredentialService       = "ai.openwatcher.widget"
	widgetCredentialAccount       = "widget-token"
	widgetCredentialWindowsTarget = "ai.openwatcher.widget/token"
)

// SecretStore deliberately exposes no logging or serialization hooks.
type SecretStore interface {
	Read() (string, error)
	Write(string) error
	Delete() error
}

// TokenSource supplies the Desktop supervisor without exposing a token to Wails bindings.
type TokenSource interface{ Token() (string, error) }

type StoreTokenSource struct{ Store SecretStore }

func (s StoreTokenSource) Token() (string, error) {
	if s.Store == nil {
		return "", ErrUnsupported
	}
	token, err := s.Store.Read()
	if err != nil {
		return "", err
	}
	if err := ValidateToken(token); err != nil {
		return "", err
	}
	return token, nil
}

func GenerateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成悬浮球凭据失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func ValidateToken(token string) error {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != token {
		return ErrInvalidCredential
	}
	return nil
}

// ConfigSynchronizer atomically writes only the token hash to the sidecar config.
// The package-level lock also serializes concurrent Desktop toggle/start operations.
type ConfigSynchronizer struct{}

func (s *ConfigSynchronizer) Ensure(store SecretStore, configPath string) error {
	if store == nil {
		return ErrUnsupported
	}
	configMu.Lock()
	defer configMu.Unlock()
	token, err := store.Read()
	if errors.Is(err, ErrNotFound) {
		token, err = GenerateToken()
		if err == nil {
			err = store.Write(token)
		}
	}
	if err != nil {
		return fmt.Errorf("准备悬浮球凭据失败: %w", err)
	}
	if err := ValidateToken(token); err != nil {
		return fmt.Errorf("准备悬浮球凭据失败: %w", err)
	}
	return syncTokenHash(configPath, token)
}

// Rotate replaces a missing or invalid credential only after an explicit user
// action. It restores the previous valid store value if the config update fails.
func (s *ConfigSynchronizer) Rotate(store SecretStore, configPath string) error {
	if store == nil {
		return ErrUnsupported
	}
	configMu.Lock()
	defer configMu.Unlock()
	previous, readErr := store.Read()
	previousValid := readErr == nil && ValidateToken(previous) == nil
	if readErr != nil && !errors.Is(readErr, ErrNotFound) && !errors.Is(readErr, ErrInvalidCredential) {
		return fmt.Errorf("读取待替换的悬浮球凭据失败: %w", readErr)
	}
	token, err := GenerateToken()
	if err != nil {
		return err
	}
	if err := store.Write(token); err != nil {
		return fmt.Errorf("写入新的悬浮球凭据失败: %w", err)
	}
	if err := syncTokenHash(configPath, token); err != nil {
		var restoreErr error
		if previousValid {
			restoreErr = store.Write(previous)
		} else {
			restoreErr = store.Delete()
		}
		if restoreErr != nil {
			return fmt.Errorf("%w；恢复原凭据失败: %v", err, restoreErr)
		}
		return err
	}
	return nil
}

func syncTokenHash(configPath, token string) error {
	cfg, resolved, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("读取本机服务配置失败: %w", err)
	}
	hash := pairing.HashToken(token)
	if cfg.WidgetTokenHash == hash {
		return nil
	}
	cfg.WidgetTokenHash = hash
	if err := config.Save(resolved, cfg); err != nil {
		return fmt.Errorf("保存悬浮球凭据摘要失败: %w", err)
	}
	return nil
}

// MemoryStore is intentionally only for deterministic unit tests.
type MemoryStore struct {
	mu    sync.Mutex
	token string
}

func (s *MemoryStore) Read() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token == "" {
		return "", ErrNotFound
	}
	return s.token, nil
}
func (s *MemoryStore) Write(token string) error {
	if err := ValidateToken(token); err != nil {
		return err
	}
	s.mu.Lock()
	s.token = token
	s.mu.Unlock()
	return nil
}
func (s *MemoryStore) Delete() error { s.mu.Lock(); s.token = ""; s.mu.Unlock(); return nil }
