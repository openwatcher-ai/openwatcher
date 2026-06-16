package main

import "testing"

func TestMaybeRunCodexCompactHookIgnoresOtherCommands(t *testing.T) {
	handled, exitCode := maybeRunCodexCompactHook([]string{"openwatcher", "serve"})
	if handled {
		t.Fatalf("handled = true, want false")
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}
