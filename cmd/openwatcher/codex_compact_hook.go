package main

import (
	"fmt"
	"os"
	"time"

	"openwatcher/internal/codexcompact"
)

func maybeRunCodexCompactHook(args []string) (bool, int) {
	if len(args) < 2 || args[1] != "codex-compact-hook" {
		return false, 0
	}
	if err := codexcompact.HandleHook(os.Stdin, time.Now()); err != nil {
		fmt.Fprintf(os.Stderr, "OpenWatcher Codex compact hook failed: %v\n", err)
	}
	return true, 0
}
