//go:build windows

package processutil

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func HideConsoleWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
