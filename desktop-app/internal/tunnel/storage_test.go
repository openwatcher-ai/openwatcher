package tunnel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreSaveAndLoadBindingWithSecureModes(t *testing.T) {
	store := NewStore(t.TempDir())

	identity, err := store.EnsureIdentity()
	if err != nil {
		t.Fatalf("EnsureIdentity err = %v", err)
	}
	if identity.InstallID == "" || identity.MachineFingerprintHash == "" {
		t.Fatalf("identity = %#v", identity)
	}

	if err := store.SaveBinding(Binding{
		PublicBaseURL: "https://ow-demo.openwatcher.ai",
		TunnelID:      "cf_tunnel_demo",
		TokenVersion:  3,
		RedeemedAt:    "2026-06-08T12:00:00Z",
	}, RedeemResponse{TunnelToken: "secret-token"}); err != nil {
		t.Fatalf("SaveBinding err = %v", err)
	}

	binding, token, err := store.LoadBinding()
	if err != nil {
		t.Fatalf("LoadBinding err = %v", err)
	}
	if binding.PublicBaseURL != "https://ow-demo.openwatcher.ai" || token != "secret-token" {
		t.Fatalf("binding/token = %#v / %q", binding, token)
	}

	for _, path := range []string{
		filepath.Join(store.RootDir(), "identity.json"),
		filepath.Join(store.RootDir(), "binding.json"),
		filepath.Join(store.RootDir(), "tunnel-token.txt"),
		filepath.Join(store.RootDir(), "cloudflared-config.yml"),
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s mode = %o, want 0600", path, got)
		}
	}
}

func TestWriteRunnerConfigUsesActualOriginURL(t *testing.T) {
	store := NewStore(t.TempDir())
	binding := Binding{
		PublicBaseURL: "https://ow-demo.openwatcher.ai",
		TunnelID:      "cf_tunnel_demo",
		TokenVersion:  1,
		RedeemedAt:    "2026-06-08T12:00:00Z",
	}
	if err := store.WriteRunnerConfig(binding, "http://127.0.0.1:8788"); err != nil {
		t.Fatalf("WriteRunnerConfig err = %v", err)
	}
	data, err := os.ReadFile(store.RunnerConfigPath())
	if err != nil {
		t.Fatalf("ReadFile err = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "hostname: ow-demo.openwatcher.ai") {
		t.Fatalf("runner config missing hostname: %s", text)
	}
	if !strings.Contains(text, `service: "http://127.0.0.1:8788"`) {
		t.Fatalf("runner config missing origin: %s", text)
	}
}

func TestNamedStoreUsesDedicatedRoot(t *testing.T) {
	store := NewNamedStore(t.TempDir(), "managed-dev-tunnel")
	if !strings.Contains(store.RootDir(), "managed-dev-tunnel") {
		t.Fatalf("unexpected root dir: %s", store.RootDir())
	}
}
