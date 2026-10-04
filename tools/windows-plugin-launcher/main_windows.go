//go:build windows

// cc-connect-plugin is built with -H windowsgui so MCP startup has no console.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	path, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(launch(filepath.Join(filepath.Dir(path), "cc-connect.exe")))
}

func launch(binary string) int {
	cmd := exec.Command(binary, "codex-plugin")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "cc-connect plugin:", err)
		return 1
	}
	return 0
}
