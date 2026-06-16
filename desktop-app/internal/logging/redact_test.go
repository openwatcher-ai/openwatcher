package logging

import (
	"strings"
	"testing"
)

func TestRedactLineHidesSensitiveValues(t *testing.T) {
	redactor := NewRedactor()
	input := `Authorization: Bearer abc token=foo tunnelToken=bar TunnelSecret=cf-secret cloudflare_tunnel_secret=db-secret pairing code=889900 deviceToken=baz tunnelCode=OW-ABCD-1234`
	got := redactor.RedactLine(input)

	for _, forbidden := range []string{"abc", "foo", "bar", "cf-secret", "db-secret", "889900", "baz", "OW-ABCD-1234"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("redacted log leaked %q: %s", forbidden, got)
		}
	}
}

func TestPathPatternRewritesHomeDir(t *testing.T) {
	redactor := &Redactor{
		patterns: []redactionPattern{pathPattern("/tmp/openwatcher-home")},
	}

	got := redactor.RedactLine("config file: /tmp/openwatcher-home/.openwatcher/config.json")
	if strings.Contains(got, "/tmp/openwatcher-home") {
		t.Fatalf("redacted path leaked home dir: %s", got)
	}
	if !strings.Contains(got, "~/.openwatcher/config.json") {
		t.Fatalf("redacted path missing home shorthand: %s", got)
	}
}
