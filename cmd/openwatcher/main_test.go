package main

import (
	"net"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"openwatcher/internal/config"
	"openwatcher/internal/server"
)

func TestServeAliasDropsLeadingSubcommand(t *testing.T) {
	input := []string{"openwatcher", "serve", "--listen", "127.0.0.1:8787"}
	got := normalizeArgsForServeAlias(input)
	want := []string{"openwatcher", "--listen", "127.0.0.1:8787"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeArgsForServeAlias() = %#v, want %#v", got, want)
	}
}

func TestServeAliasLeavesOtherArgsUntouched(t *testing.T) {
	input := []string{"openwatcher", "--listen", "127.0.0.1:8787"}
	got := normalizeArgsForServeAlias(input)
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("normalizeArgsForServeAlias() changed args: %#v", got)
	}
}

func TestWidgetEndpointLineIsStableAndCredentialFree(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	line := formatWidgetEndpointLine(listener.Addr())
	if !strings.HasPrefix(line, widgetEndpointLinePrefix) || strings.Contains(line, "token") {
		t.Fatalf("endpoint line = %q", line)
	}
	endpoint, ok := parseWidgetEndpointLine(line)
	if !ok || endpoint != "http://"+listener.Addr().String() {
		t.Fatalf("parsed endpoint = %q, ok=%v", endpoint, ok)
	}
}

func TestWidgetListenAddressMustBeLoopback(t *testing.T) {
	for _, address := range []string{"127.0.0.1:0", "[::1]:0"} {
		if err := validateWidgetListenAddress(address); err != nil {
			t.Fatalf("validate %q: %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:0", ":0", "192.168.1.2:0", "localhost:0"} {
		if err := validateWidgetListenAddress(address); err == nil {
			t.Fatalf("validate %q unexpectedly succeeded", address)
		}
	}
}

func TestValidWidgetTokenHashRequiresSHA256Hex(t *testing.T) {
	if !validWidgetTokenHash(strings.Repeat("ab", 32)) {
		t.Fatal("valid SHA-256 hex was rejected")
	}
	for _, value := range []string{"", "short", strings.Repeat("z", 64), strings.Repeat("ab", 31)} {
		if validWidgetTokenHash(value) {
			t.Fatalf("invalid widget token hash %q was accepted", value)
		}
	}
}

func TestWidgetServerBindsPortZeroAndShutsDown(t *testing.T) {
	cfg := config.Config{WidgetTokenHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	app := server.New(filepath.Join(t.TempDir(), "config.json"), cfg, false, nil, nil)
	httpServer, listener, err := startWidgetServer(app, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start widget server: %v", err)
	}
	endpoint, ok := parseWidgetEndpointLine(formatWidgetEndpointLine(listener.Addr()))
	if !ok || !strings.HasPrefix(endpoint, "http://127.0.0.1:") {
		t.Fatalf("endpoint = %q, ok=%v", endpoint, ok)
	}
	if err := httpServer.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown widget server: %v", err)
	}
	if _, err := http.Get(endpoint + "/api/status"); err == nil {
		t.Fatal("widget server still accepted connections after shutdown")
	}
}
