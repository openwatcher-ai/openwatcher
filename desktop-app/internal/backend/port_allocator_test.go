package backend

import (
	"net"
	"strconv"
	"testing"
)

func TestResolveLoopbackListenSkipsOccupiedPort(t *testing.T) {
	first, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen first: %v", err)
	}
	defer first.Close()

	preferred := first.Addr().String()
	resolved, err := ResolveLoopbackListen(preferred)
	if err != nil {
		t.Fatalf("ResolveLoopbackListen err = %v", err)
	}
	if resolved == preferred {
		t.Fatalf("resolved listen should move away from occupied port: %q", resolved)
	}

	_, preferredPortText, _ := net.SplitHostPort(preferred)
	_, resolvedPortText, _ := net.SplitHostPort(resolved)
	preferredPort, _ := strconv.Atoi(preferredPortText)
	resolvedPort, _ := strconv.Atoi(resolvedPortText)
	if resolvedPort <= preferredPort {
		t.Fatalf("resolved port = %d, want > %d", resolvedPort, preferredPort)
	}
}

func TestResolveLoopbackListenLeavesNonLoopbackUntouched(t *testing.T) {
	resolved, err := ResolveLoopbackListen("192.168.1.8:8787")
	if err != nil {
		t.Fatalf("ResolveLoopbackListen err = %v", err)
	}
	if resolved != "192.168.1.8:8787" {
		t.Fatalf("resolved listen = %q", resolved)
	}
}
