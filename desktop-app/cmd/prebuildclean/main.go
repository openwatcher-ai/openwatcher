package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fatal("resolve cwd: %v", err)
	}

	targets := []string{
		"OpenWatcher.app",
		"OpenWatcher Desktop.app",
		"openwatcher",
		"openwatcher-desktop",
		"openwatcher.exe",
		"openwatcher-desktop.exe",
		"bundled",
	}

	for _, target := range targets {
		path := filepath.Join(cwd, target)
		if err := os.RemoveAll(path); err != nil {
			fatal("remove %s: %v", path, err)
		}
	}

	fmt.Println("cleaned build/bin leftovers")
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
