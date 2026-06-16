//go:build !windows

package processutil

import "os/exec"

func HideConsoleWindow(*exec.Cmd) {}
