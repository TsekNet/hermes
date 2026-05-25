//go:build !windows

package cmd

import (
	"runtime"
	"strings"
	"testing"
)

func TestBuildUserEnv(t *testing.T) {
	t.Parallel()

	s := userSession{
		uid:      1001,
		gid:      1001,
		username: "jdoe",
		home:     "/home/jdoe",
	}

	env := buildUserEnv(s)
	envMap := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	tests := []struct {
		key  string
		want string
	}{
		{"HOME", "/home/jdoe"},
		{"USER", "jdoe"},
		{"LOGNAME", "jdoe"},
		{"SHELL", "/bin/sh"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			if got := envMap[tt.key]; got != tt.want {
				t.Errorf("%s = %q, want %q", tt.key, got, tt.want)
			}
		})
	}

	if _, ok := envMap["PATH"]; !ok {
		t.Error("PATH not set")
	}

	if runtime.GOOS == "linux" {
		t.Run("linux XDG vars", func(t *testing.T) {
			if envMap["XDG_RUNTIME_DIR"] != "/run/user/1001" {
				t.Errorf("XDG_RUNTIME_DIR = %q, want /run/user/1001", envMap["XDG_RUNTIME_DIR"])
			}
			if envMap["DISPLAY"] != ":0" {
				t.Errorf("DISPLAY = %q, want :0", envMap["DISPLAY"])
			}
			if !strings.Contains(envMap["DBUS_SESSION_BUS_ADDRESS"], "/run/user/1001/bus") {
				t.Errorf("DBUS_SESSION_BUS_ADDRESS = %q, want to contain /run/user/1001/bus", envMap["DBUS_SESSION_BUS_ADDRESS"])
			}
		})
	}
}

func TestIsPrivileged(t *testing.T) {
	t.Parallel()
	got := isPrivileged()
	// In test context, we're not root.
	if got {
		t.Skip("running as root, cannot verify non-privileged path")
	}
}
