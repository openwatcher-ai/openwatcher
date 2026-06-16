package main

import (
	"testing"

	"openwatcher/testsupport/contracts"
)

func TestBootstrapContractFixtureForWatchProtocol(t *testing.T) {
	uri, err := buildBootstrapURI(
		[]BootstrapEndpointRequest{
			{ID: "lan", Label: "局域网", URL: "http://192.168.1.12:8787", Priority: 0},
			{ID: "public", Label: "公网", URL: "https://watch.example.com", Priority: 1},
		},
		"contract-token-0123456789abcdef0123456789",
		"Contract Watch",
	)
	if err != nil {
		t.Fatalf("build bootstrap URI: %v", err)
	}
	data := []byte(uri + "\n")
	contracts.AssertFixture(t, "bootstrap-uri.txt", data)
}
