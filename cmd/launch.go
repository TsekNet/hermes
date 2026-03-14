//go:build !windows

package cmd

import (
	"os/exec"
	"syscall"
)

// launchSubprocess starts a detached child process with the given binary
// and arguments. Setsid puts the child in its own session so parent signals
// (e.g. SIGINT on daemon stop) don't kill in-flight UI subprocesses.
func launchSubprocess(binary string, args []string) error {
	cmd := exec.Command(binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
