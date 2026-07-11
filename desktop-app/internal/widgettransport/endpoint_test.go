package widgettransport

import "testing"

func TestParseEndpointCanonicalizesNumericLoopbackOrigins(t *testing.T) {
	tests := map[string]string{
		" http://127.0.0.1:8787 ": "http://127.0.0.1:8787",
		"http://[::1]:8787/":      "http://[::1]:8787",
	}
	for raw, want := range tests {
		got, err := ParseEndpoint(raw)
		if err != nil || got != want {
			t.Fatalf("ParseEndpoint(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
}

func TestParseEndpointRejectsAnythingBeyondPrivateOrigin(t *testing.T) {
	invalid := []string{
		"",
		"https://127.0.0.1:8787",
		"http://localhost:8787",
		"http://example.test:8787",
		"http://0.0.0.0:8787",
		"http://127.0.0.1",
		"http://127.0.0.1:0",
		"http://127.0.0.1:65536",
		"http://127.000.000.001:8787",
		"http://user@127.0.0.1:8787",
		"http://127.0.0.1:8787/api/status",
		"http://127.0.0.1:8787?x=1",
		"http://127.0.0.1:8787#fragment",
		"http://127.0.0.1:8787/%2f",
	}
	for _, raw := range invalid {
		if got, err := ParseEndpoint(raw); err == nil {
			t.Fatalf("ParseEndpoint(%q) accepted %q", raw, got)
		}
	}
}
