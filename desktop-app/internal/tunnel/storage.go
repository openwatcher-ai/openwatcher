package tunnel

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var ErrBindingNotFound = errors.New("managed tunnel binding not found")

type Store struct {
	configDir string
	rootName  string
}

type bindingFile struct {
	PublicBaseURL string `json:"publicBaseUrl"`
	TunnelID      string `json:"tunnelId"`
	TokenVersion  int    `json:"tokenVersion"`
	RedeemedAt    string `json:"redeemedAt"`
}

func NewStore(configDir string) *Store {
	return NewNamedStore(configDir, "managed-tunnel")
}

func NewNamedStore(configDir string, rootName string) *Store {
	trimmedRoot := strings.TrimSpace(rootName)
	if trimmedRoot == "" {
		trimmedRoot = "managed-tunnel"
	}
	return &Store{
		configDir: filepath.Clean(configDir),
		rootName:  trimmedRoot,
	}
}

func (s *Store) EnsureIdentity() (Identity, error) {
	identity, err := s.LoadIdentity()
	if err == nil {
		return identity, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Identity{}, err
	}

	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return Identity{}, err
	}
	host, _ := os.Hostname()
	sum := sha256.Sum256(append([]byte(strings.TrimSpace(host)+"|"+runtime.GOOS+"|"+runtime.GOARCH+"|"), buf...))
	identity = Identity{
		InstallID:              "ins_" + hex.EncodeToString(buf[:10]),
		MachineFingerprintHash: "mf_" + hex.EncodeToString(sum[:16]),
	}
	if err := s.writeJSON(s.identityPath(), identity); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

func (s *Store) LoadIdentity() (Identity, error) {
	data, err := os.ReadFile(s.identityPath())
	if err != nil {
		return Identity{}, err
	}
	var identity Identity
	if err := json.Unmarshal(data, &identity); err != nil {
		return Identity{}, err
	}
	if strings.TrimSpace(identity.InstallID) == "" || strings.TrimSpace(identity.MachineFingerprintHash) == "" {
		return Identity{}, errors.New("managed tunnel identity is incomplete")
	}
	return identity, nil
}

func (s *Store) SaveBinding(binding Binding, response RedeemResponse) error {
	file := bindingFile{
		PublicBaseURL: strings.TrimRight(strings.TrimSpace(binding.PublicBaseURL), "/"),
		TunnelID:      strings.TrimSpace(binding.TunnelID),
		TokenVersion:  binding.TokenVersion,
		RedeemedAt:    binding.RedeemedAt,
	}
	if file.PublicBaseURL == "" || file.TunnelID == "" {
		return errors.New("managed tunnel binding is incomplete")
	}
	if strings.TrimSpace(response.TunnelToken) == "" && response.TunnelCredentials == nil {
		return errors.New("managed tunnel binding is incomplete")
	}
	if err := s.writeJSON(s.bindingPath(), file); err != nil {
		return err
	}
	if response.TunnelCredentials != nil {
		if err := s.writeJSON(s.credentialsPath(), response.TunnelCredentials); err != nil {
			return err
		}
		_ = os.Remove(s.tokenPath())
	} else {
		if err := s.writeSecretFile(s.tokenPath(), strings.TrimSpace(response.TunnelToken)+"\n"); err != nil {
			return err
		}
		_ = os.Remove(s.credentialsPath())
	}
	return s.ensureRunnerConfig()
}

func (s *Store) LoadBinding() (Binding, string, error) {
	data, err := os.ReadFile(s.bindingPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Binding{}, "", ErrBindingNotFound
		}
		return Binding{}, "", err
	}
	var file bindingFile
	if err := json.Unmarshal(data, &file); err != nil {
		return Binding{}, "", err
	}
	token := ""
	tokenData, err := os.ReadFile(s.tokenPath())
	if err == nil {
		token = strings.TrimSpace(string(tokenData))
	} else if !errors.Is(err, os.ErrNotExist) {
		return Binding{}, "", err
	}
	hasCredentials := false
	if info, statErr := os.Stat(s.credentialsPath()); statErr == nil && !info.IsDir() {
		hasCredentials = true
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return Binding{}, "", statErr
	}
	if strings.TrimSpace(file.PublicBaseURL) == "" || strings.TrimSpace(file.TunnelID) == "" || (token == "" && !hasCredentials) {
		return Binding{}, "", ErrBindingNotFound
	}
	return Binding{
		PublicBaseURL: file.PublicBaseURL,
		TunnelID:      file.TunnelID,
		TokenVersion:  file.TokenVersion,
		RedeemedAt:    file.RedeemedAt,
	}, token, nil
}

func (s *Store) ClearBinding() error {
	for _, path := range []string{s.bindingPath(), s.tokenPath(), s.credentialsPath(), s.runnerConfigPath()} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *Store) RootDir() string {
	return filepath.Join(s.configDir, s.rootName)
}

func (s *Store) TokenPath() string {
	return s.tokenPath()
}

func (s *Store) CredentialsPath() string {
	return s.credentialsPath()
}

func (s *Store) RunnerConfigPath() string {
	return s.runnerConfigPath()
}

func (s *Store) WriteRunnerConfig(binding Binding, originURL string) error {
	hostname, err := publicHostname(binding.PublicBaseURL)
	if err != nil {
		return err
	}
	originURL = strings.TrimRight(strings.TrimSpace(originURL), "/")
	if originURL == "" {
		return errors.New("managed tunnel origin url is empty")
	}
	credentialsPath := strings.TrimSpace(s.credentialsPath())
	if credentialsPath == "" {
		return errors.New("managed tunnel credentials path is empty")
	}
	content := fmt.Sprintf(`# OpenWatcher 托管隧道运行时配置
tunnel: %s
credentials-file: "%s"
ingress:
  - hostname: %s
    service: "%s"
  - service: http_status:404
`, strings.TrimSpace(binding.TunnelID), credentialsPath, hostname, originURL)
	return s.writeSecretFile(s.runnerConfigPath(), content)
}

func (s *Store) identityPath() string {
	return filepath.Join(s.RootDir(), "identity.json")
}

func (s *Store) bindingPath() string {
	return filepath.Join(s.RootDir(), "binding.json")
}

func (s *Store) tokenPath() string {
	return filepath.Join(s.RootDir(), "tunnel-token.txt")
}

func (s *Store) credentialsPath() string {
	return filepath.Join(s.RootDir(), "tunnel-credentials.json")
}

func (s *Store) runnerConfigPath() string {
	return filepath.Join(s.RootDir(), "cloudflared-config.yml")
}

func (s *Store) ensureRootDir() error {
	if err := os.MkdirAll(s.RootDir(), 0o700); err != nil {
		return err
	}
	return os.Chmod(s.RootDir(), 0o700)
}

func (s *Store) ensureRunnerConfig() error {
	if err := s.ensureRootDir(); err != nil {
		return err
	}
	if _, err := os.Stat(s.runnerConfigPath()); err == nil {
		return os.Chmod(s.runnerConfigPath(), 0o600)
	}
	content := []byte("# OpenWatcher 托管隧道运行时配置\n")
	return s.writeSecretFile(s.runnerConfigPath(), string(content))
}

func publicHostname(publicBaseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(publicBaseURL))
	if err != nil {
		return "", err
	}
	hostname := strings.TrimSpace(parsed.Hostname())
	if hostname == "" {
		return "", errors.New("managed tunnel public hostname is empty")
	}
	return hostname, nil
}

func (s *Store) writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return s.writeSecretFile(path, string(data))
}

func (s *Store) writeSecretFile(path string, content string) error {
	if err := s.ensureRootDir(); err != nil {
		return err
	}
	tmpPath := path + "." + time.Now().UTC().Format("20060102150405") + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Chmod(path, 0o600)
}
