package backend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbePublicHealthReportsCloudflare1033(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(530)
		_, _ = w.Write([]byte("error code: 1033"))
	}))
	defer server.Close()

	status, err := ProbePublicHealth(t.Context(), server.URL, "开发环境托管隧道公网地址")
	if err == nil {
		t.Fatalf("expected JSON parse error")
	}
	if status.OK {
		t.Fatalf("expected failed health status")
	}
	if status.HTTPCode != 530 {
		t.Fatalf("HTTPCode = %d, want 530", status.HTTPCode)
	}
	if !strings.Contains(status.Message, "Cloudflare 1033") {
		t.Fatalf("message = %q", status.Message)
	}
}
