package cmd

import "testing"

func TestVersionCmd_NoPanic(t *testing.T) {
	t.Parallel()
	cmd := versionCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command: %v", err)
	}
}
