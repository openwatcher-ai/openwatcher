package network

import "testing"

func TestNormalizePort(t *testing.T) {
	if got := NormalizePort(""); got != "8787" {
		t.Fatalf("NormalizePort empty = %q", got)
	}
	if got := NormalizePort("9191"); got != "9191" {
		t.Fatalf("NormalizePort explicit = %q", got)
	}
}

func TestBuildManagedBetaURL(t *testing.T) {
	got := BuildManagedBetaURL("ABCD-EFGH")
	if got != "https://abcd-efgh.beta.openwatcher.example.com" {
		t.Fatalf("BuildManagedBetaURL = %q", got)
	}
}

func TestBuildListen(t *testing.T) {
	if got := BuildListen("192.168.1.8", "9000", false); got != "192.168.1.8:9000" {
		t.Fatalf("BuildListen selected = %q", got)
	}
	if got := BuildListen("192.168.1.8", "9000", true); got != "0.0.0.0:9000" {
		t.Fatalf("BuildListen bindAll = %q", got)
	}
}

func TestNormalizePublicURL(t *testing.T) {
	if got := NormalizePublicURL("demo.example.com/"); got != "https://demo.example.com" {
		t.Fatalf("NormalizePublicURL = %q", got)
	}
	if got := NormalizePublicURL("http://demo.example.com/path/"); got != "http://demo.example.com/path" {
		t.Fatalf("NormalizePublicURL keep scheme = %q", got)
	}
}

func TestNormalizeModeAcceptsFrontendAliases(t *testing.T) {
	if got := NormalizeMode("public"); got != ModePublicURL {
		t.Fatalf("NormalizeMode(public) = %q", got)
	}
	if got := NormalizeMode("tunnel"); got != ModeManagedBeta {
		t.Fatalf("NormalizeMode(tunnel) = %q", got)
	}
}

func TestValidatePublicURL(t *testing.T) {
	got, err := ValidatePublicURL("demo.example.com/path/", true)
	if err != nil {
		t.Fatalf("ValidatePublicURL err = %v", err)
	}
	if got != "https://demo.example.com/path" {
		t.Fatalf("ValidatePublicURL = %q", got)
	}

	if _, err := ValidatePublicURL("http://demo.example.com", true); err == nil {
		t.Fatalf("expected https requirement error")
	}
	if _, err := ValidatePublicURL("ftp://demo.example.com", false); err == nil {
		t.Fatalf("expected scheme error")
	}
	if _, err := ValidatePublicURL("https://demo.example.com/path?mode=test", false); err == nil {
		t.Fatalf("expected query error")
	}
}
