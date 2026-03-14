package cmd

import "os/exec"

// launchSubprocess starts a detached child process with the given binary
// and arguments. On Windows, CREATE_BREAKAWAY_FROM_JOB in the broadcast
// path (sessionlaunch_windows.go) handles process group isolation.
func launchSubprocess(binary string, args []string) error {
	cmd := exec.Command(binary, args...)
	return cmd.Start()
}
