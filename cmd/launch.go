package cmd

import "os/exec"

// launchSubprocess starts a detached child process with the given binary
// and arguments. The process inherits the current user's session (because
// the service daemon already runs per-user).
func launchSubprocess(binary string, args []string) error {
	cmd := exec.Command(binary, args...)
	return cmd.Start()
}
