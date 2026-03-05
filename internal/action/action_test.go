package action

import (
	"runtime"
	"strings"
	"testing"
)

func TestAllowedOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		goos  string
		want  bool
	}{
		{"https any platform", "https://example.com", "windows", true},
		{"http any platform", "http://intranet/kb", "darwin", true},

		{"ms-settings on windows", "ms-settings:windowsupdate", "windows", true},
		{"ms-settings on mac blocked", "ms-settings:windowsupdate", "darwin", false},
		{"ms-settings on linux blocked", "ms-settings:windowsupdate", "linux", false},

		{"apple prefs on mac", "x-apple.systempreferences:com.apple.preference.security", "darwin", true},
		{"apple prefs on windows blocked", "x-apple.systempreferences:com.apple.preference.security", "windows", false},
		{"apple prefs on linux blocked", "x-apple.systempreferences:com.apple.preference.security", "linux", false},

		{"apple prefs with anchor", "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles", "darwin", true},
		{"ms-settings with path", "ms-settings:windowsupdate-action", "windows", true},

		{"ftp blocked", "ftp://evil.com/payload", "windows", false},
		{"file blocked", "file:///etc/passwd", "linux", false},
		{"javascript blocked", "javascript:alert(1)", "darwin", false},
		{"empty string", "", "windows", false},
		{"bare path", "/tmp/foo", "linux", false},

		{"case insensitive https", "HTTPS://EXAMPLE.COM", "linux", true},
		{"case insensitive ms-settings", "MS-SETTINGS:windowsupdate", "windows", true},
		{"case insensitive apple prefs", "X-Apple.SystemPreferences:com.apple.preference.security", "darwin", true},

		{"cmd on windows", "cmd:shutdown /r /t 0", "windows", true},
		{"cmd on mac", "cmd:osascript -e 'tell app \"Finder\" to restart'", "darwin", true},
		{"cmd on linux", "cmd:systemctl reboot", "linux", true},
		{"cmd case insensitive", "CMD:echo hello", "windows", true},
		{"cmd empty command", "cmd:", "windows", false},
		{"cmd whitespace only", "cmd:   ", "linux", false},
		{"cmd with args", "cmd:powershell.exe -Command Get-Process", "windows", true},
		{"cmd with pipe", "cmd:echo hello | grep hello", "linux", true},
		{"cmd with quotes", `cmd:bash -c "echo test"`, "darwin", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := AllowedOn(tt.value, tt.goos); got != tt.want {
				t.Errorf("AllowedOn(%q, %q) = %v, want %v", tt.value, tt.goos, got, tt.want)
			}
		})
	}
}

func TestClassifyOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		goos  string
		want  Kind
	}{
		{"https is web", "https://example.com", "linux", KindWeb},
		{"http is web", "http://example.com", "windows", KindWeb},
		{"ms-settings is settings", "ms-settings:windowsupdate", "windows", KindSettings},
		{"apple prefs is settings", "x-apple.systempreferences:com.apple.preference.security", "darwin", KindSettings},
		{"ms-settings wrong platform", "ms-settings:windowsupdate", "darwin", KindUnknown},
		{"cmd is command", "cmd:shutdown /r /t 0", "windows", KindCommand},
		{"cmd any platform", "cmd:echo hi", "linux", KindCommand},
		{"cmd on mac", "cmd:open /Applications", "darwin", KindCommand},
		{"unknown scheme", "ftp://x.com", "linux", KindUnknown},
		{"plain value", "restart", "windows", KindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ClassifyOn(tt.value, tt.goos); got != tt.want {
				t.Errorf("ClassifyOn(%q, %q) = %v, want %v", tt.value, tt.goos, got, tt.want)
			}
		})
	}
}

func TestSettingsSchemes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos      string
		wantCount int
		wantFirst string
	}{
		{"windows", 1, "ms-settings:"},
		{"darwin", 1, "x-apple.systempreferences:"},
		{"linux", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			t.Parallel()
			got := SettingsSchemes(tt.goos)
			if len(got) != tt.wantCount {
				t.Fatalf("len = %d, want %d", len(got), tt.wantCount)
			}
			if tt.wantCount > 0 && got[0].Prefix != tt.wantFirst {
				t.Errorf("prefix = %q, want %q", got[0].Prefix, tt.wantFirst)
			}
		})
	}
}

func TestAllSchemes(t *testing.T) {
	t.Parallel()

	win := AllSchemes("windows")
	mac := AllSchemes("darwin")
	lin := AllSchemes("linux")

	if len(win) != 4 {
		t.Errorf("windows schemes = %d, want 4 (https, http, ms-settings, cmd)", len(win))
	}
	if len(mac) != 4 {
		t.Errorf("darwin schemes = %d, want 4 (https, http, x-apple.systempreferences, cmd)", len(mac))
	}
	if len(lin) != 3 {
		t.Errorf("linux schemes = %d, want 3 (https, http, cmd)", len(lin))
	}
}

func TestIsCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  bool
	}{
		{"cmd:echo hi", true},
		{"CMD:echo hi", true},
		{"Cmd:echo hi", true},
		{"url:https://x.com", false},
		{"restart", false},
		{"", false},
		{"cmd:", true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			if got := IsCommand(tt.value); got != tt.want {
				t.Errorf("IsCommand(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestCommandString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  string
	}{
		{"cmd:echo hi", "echo hi"},
		{"cmd: shutdown /r /t 0", "shutdown /r /t 0"},
		{"cmd:  spaces  ", "spaces"},
		{"cmd:powershell.exe -Command Get-Process", "powershell.exe -Command Get-Process"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			if got := CommandString(tt.value); got != tt.want {
				t.Errorf("CommandString(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestRunCommand(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Run("echo on windows", func(t *testing.T) {
			t.Parallel()
			out, err := RunCommand("cmd:echo hello")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, "hello") {
				t.Errorf("output = %q, want contains 'hello'", out)
			}
		})

		t.Run("powershell on windows", func(t *testing.T) {
			t.Parallel()
			out, err := RunCommand("cmd:powershell.exe -Command \"Write-Output 'test123'\"")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, "test123") {
				t.Errorf("output = %q, want contains 'test123'", out)
			}
		})
	} else {
		t.Run("echo on unix", func(t *testing.T) {
			t.Parallel()
			out, err := RunCommand("cmd:echo hello")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.TrimSpace(out) != "hello" {
				t.Errorf("output = %q, want 'hello'", out)
			}
		})

		t.Run("pipe on unix", func(t *testing.T) {
			t.Parallel()
			out, err := RunCommand("cmd:echo abc123 | grep abc")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, "abc123") {
				t.Errorf("output = %q, want contains 'abc123'", out)
			}
		})

		t.Run("args with spaces", func(t *testing.T) {
			t.Parallel()
			out, err := RunCommand("cmd:printf '%s %s' hello world")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.TrimSpace(out) != "hello world" {
				t.Errorf("output = %q, want 'hello world'", out)
			}
		})
	}

	t.Run("bad command fails", func(t *testing.T) {
		t.Parallel()
		_, err := RunCommand("cmd:__nonexistent_binary_xyz__")
		if err == nil {
			t.Fatal("expected error for nonexistent command")
		}
	})
}

func TestRunCommandOn(t *testing.T) {
	t.Parallel()

	// RunCommandOn with current OS should behave the same as RunCommand.
	out, err := RunCommandOn("cmd:echo crossplatform", runtime.GOOS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "crossplatform") {
		t.Errorf("output = %q, want contains 'crossplatform'", out)
	}
}
