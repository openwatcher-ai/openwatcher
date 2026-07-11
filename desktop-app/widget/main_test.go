package main

import (
	"strings"
	"testing"

	"openwatcher/desktop-app/internal/widgetauth"
)

func TestReadWidgetTokenAcceptsSinglePipePayload(t *testing.T) {
	token, err := widgetauth.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	got, err := readWidgetToken(strings.NewReader(token + "\n"))
	if err != nil || got != token {
		t.Fatalf("readWidgetToken = %q, %v", got, err)
	}
}

func TestReadWidgetTokenRejectsMissingMalformedAndOversizedInput(t *testing.T) {
	for _, input := range []string{"", "not-a-token\n", strings.Repeat("x", 129), strings.Repeat("A", 43) + "\nextra\n"} {
		if token, err := readWidgetToken(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %d-byte input as %q", len(input), token)
		}
	}
}
