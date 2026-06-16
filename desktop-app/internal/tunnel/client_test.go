package tunnel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientRedeemMapsStructuredErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"ok":false,"code":"code_expired","message":"该配置码已过期"}`))
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL, server.Client())
	_, err := client.Redeem(t.Context(), "OW-ABCD-1234", Identity{
		InstallID:              "ins_demo",
		MachineFingerprintHash: "mf_demo",
	}, "0.1.0")
	if err == nil {
		t.Fatalf("expected redeem error")
	}
	if !strings.Contains(err.Error(), "配置码已过期") {
		t.Fatalf("unexpected redeem error: %v", err)
	}
}

func TestClientSubmitWatchBootstrapConfigTrimsRequestAndParsesResponse(t *testing.T) {
	var received WatchBootstrapConfigRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/watch-bootstrap/config" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"bootstrapCode":"ABCD2345","status":"ready","config":{"environment":"dev","apiBase":"https://dev.example.com","source":"desktop-remote-bootstrap","configuredAt":"2026-06-12T00:00:00Z"}}`))
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL, server.Client())
	response, err := client.SubmitWatchBootstrapConfig(t.Context(), WatchBootstrapConfigRequest{
		BootstrapCode: " ABCD2345 ",
		Environment:   " dev ",
		APIBase:       " https://dev.example.com/ ",
		Source:        " desktop-remote-bootstrap ",
	})
	if err != nil {
		t.Fatalf("SubmitWatchBootstrapConfig err = %v", err)
	}
	if received.BootstrapCode != "ABCD2345" || received.Environment != "dev" || received.APIBase != "https://dev.example.com" {
		t.Fatalf("request was not normalized: %#v", received)
	}
	if response.Config.Environment != "dev" || response.Config.APIBase != "https://dev.example.com" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestClientSubmitWatchBootstrapConfigMapsStructuredErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"ok":false,"code":"bootstrap_code_consumed","message":"server message"}`))
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL, server.Client())
	_, err := client.SubmitWatchBootstrapConfig(t.Context(), WatchBootstrapConfigRequest{
		BootstrapCode: "ABCD2345",
		Environment:   "beta",
		APIBase:       "https://beta.example.com",
	})
	if err == nil {
		t.Fatalf("expected submit error")
	}
	if !strings.Contains(err.Error(), "临时配置码已被手表取走") {
		t.Fatalf("unexpected submit error: %v", err)
	}
}
