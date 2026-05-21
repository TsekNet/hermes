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
		// uri: with allowed schemes
		{"https any platform", "uri:https://example.com", "windows", true},
		{"http any platform", "uri:http://intranet/kb", "darwin", true},
		{"ms-settings on windows", "uri:ms-settings:windowsupdate", "windows", true},
		{"ms-settings on mac", "uri:ms-settings:windowsupdate", "darwin", true},
		{"ms-settings on linux", "uri:ms-settings:windowsupdate", "linux", true},
		{"apple prefs on darwin", "uri:x-apple.systempreferences:com.apple.preference.security", "darwin", true},
		{"apple prefs on windows", "uri:x-apple.systempreferences:com.apple.preference.security", "windows", true},
		{"companyportal scheme", "uri:companyportal://apps", "windows", true},
		{"slack scheme", "uri:slack://channel?team=T1&id=C1", "linux", true},
		{"msteams scheme", "uri:msteams://teams.microsoft.com/l/chat", "windows", true},
		{"codex scheme", "uri:codex://context?repo=example", "linux", true},

		// uri: schemes NOT in allowlist are rejected
		{"custom scheme rejected", "uri:myapp://open", "linux", false},
		{"smb rejected", "uri:smb://server/share", "windows", false},
		{"file rejected", "uri:file:///etc/passwd", "linux", false},
		{"javascript rejected", "uri:javascript:alert(1)", "darwin", false},
		{"data rejected", "uri:data:text/html,<script>alert(1)</script>", "linux", false},
		{"vbscript rejected", "uri:vbscript:MsgBox(1)", "windows", false},
		{"ms-msdt rejected", "uri:ms-msdt:/id PCWDiagnostic", "windows", false},
		{"ftp rejected", "uri:ftp://evil.com/payload", "linux", false},
		{"ldap rejected", "uri:ldap://attacker.com", "windows", false},
		{"ssh rejected", "uri:ssh://attacker.com", "linux", false},
		{"telnet rejected", "uri:telnet://attacker.com", "windows", false},
		{"mailto rejected", "uri:mailto:user@example.com", "linux", false},
		{"tel rejected", "uri:tel:+15551234567", "darwin", false},

		// uri: allowlist is case-insensitive (scheme matching)
		{"HTTPS allowed uppercase", "uri:HTTPS://example.com", "linux", true},
		{"Http allowed mixed", "uri:Http://example.com", "darwin", true},

		// uri: path-like values rejected (no valid scheme)
		{"windows drive path rejected", "uri:C:\\Windows\\System32\\calc.exe", "windows", false},
		{"unc path rejected", "uri:\\\\server\\share", "windows", false},
		{"unix absolute path rejected", "uri:/etc/passwd", "linux", false},
		{"percent-encoded scheme rejected", "uri:%66ile:///etc/passwd", "linux", false},

		// uri: case-insensitive prefix
		{"URI prefix uppercase", "URI:https://example.com", "linux", true},
		{"Uri prefix mixed", "Uri:https://example.com", "darwin", true},

		// uri: edge cases
		{"uri: empty body", "uri:", "linux", false},
		{"uri: whitespace only", "uri:   ", "linux", false},

		// action: valid verbs
		{"action:reboot on windows", "action:reboot", "windows", true},
		{"action:reboot on darwin", "action:reboot", "darwin", true},
		{"action:reboot on linux", "action:reboot", "linux", true},
		{"action:shutdown on windows", "action:shutdown", "windows", true},
		{"action:shutdown on darwin", "action:shutdown", "darwin", true},
		{"action:shutdown on linux", "action:shutdown", "linux", true},
		{"action:lock on windows", "action:lock", "windows", true},
		{"action:lock on darwin", "action:lock", "darwin", true},
		{"action:lock on linux", "action:lock", "linux", true},

		// action: case-insensitive prefix and verb
		{"ACTION prefix uppercase", "ACTION:reboot", "linux", true},
		{"Action prefix mixed", "Action:Reboot", "windows", true},

		// action: invalid verbs
		{"action:unknown verb", "action:restart", "linux", false},
		{"action:empty verb", "action:", "linux", false},
		{"action:arbitrary string", "action:rm -rf /", "linux", false},

		// cmd: rejected everywhere
		{"cmd: on windows", "cmd:shutdown /r /t 0", "windows", false},
		{"cmd: on darwin", "cmd:osascript -e 'restart'", "darwin", false},
		{"cmd: on linux", "cmd:systemctl reboot", "linux", false},
		{"CMD: uppercase", "CMD:echo hello", "linux", false},

		// bare schemes (no uri: prefix) rejected
		{"bare https", "https://example.com", "linux", false},
		{"bare http", "http://example.com", "windows", false},
		{"bare ms-settings", "ms-settings:windowsupdate", "windows", false},

		// plain values
		{"plain value", "restart", "linux", false},
		{"empty string", "", "windows", false},
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
		{"uri:https is URI", "uri:https://example.com", "linux", KindURI},
		{"uri:http is URI", "uri:http://example.com", "windows", KindURI},
		{"uri:ms-settings is URI", "uri:ms-settings:windowsupdate", "windows", KindURI},
		{"uri:unknown scheme is unknown", "uri:myapp://open", "linux", KindUnknown},
		{"uri:file is unknown", "uri:file:///etc/passwd", "linux", KindUnknown},
		{"action:reboot is builtin", "action:reboot", "linux", KindBuiltin},
		{"action:shutdown is builtin", "action:shutdown", "windows", KindBuiltin},
		{"action:lock is builtin", "action:lock", "darwin", KindBuiltin},
		{"action:unknown is unknown", "action:restart", "linux", KindUnknown},
		{"cmd: is unknown", "cmd:echo hi", "linux", KindUnknown},
		{"plain value is unknown", "restart", "windows", KindUnknown},
		{"bare https is unknown", "https://example.com", "linux", KindUnknown},
		{"empty is unknown", "", "linux", KindUnknown},
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

func TestValidVerbs(t *testing.T) {
	t.Parallel()

	verbs := ValidVerbs()
	if len(verbs) != 3 {
		t.Fatalf("len(ValidVerbs()) = %d, want 3", len(verbs))
	}

	want := map[string]bool{"reboot": true, "shutdown": true, "lock": true}
	for _, v := range verbs {
		if !want[v] {
			t.Errorf("unexpected verb %q", v)
		}
	}
}

func TestAllowedSchemes(t *testing.T) {
	t.Parallel()

	schemes := AllowedSchemes()
	want := []string{
		"https:", "http:", "ms-settings:", "x-apple.systempreferences:",
		"slack:", "msteams:", "companyportal:", "codex:",
	}
	if len(schemes) != len(want) {
		t.Fatalf("len(AllowedSchemes()) = %d, want %d", len(schemes), len(want))
	}
	for i, s := range schemes {
		if s != want[i] {
			t.Errorf("AllowedSchemes()[%d] = %q, want %q", i, s, want[i])
		}
	}
}

func TestDispatch(t *testing.T) {
	t.Parallel()

	t.Run("plain value returns nil", func(t *testing.T) {
		t.Parallel()
		if err := Dispatch("restart"); err != nil {
			t.Errorf("Dispatch(plain) = %v, want nil", err)
		}
	})

	t.Run("empty value returns nil", func(t *testing.T) {
		t.Parallel()
		if err := Dispatch(""); err != nil {
			t.Errorf("Dispatch(empty) = %v, want nil", err)
		}
	})

	t.Run("cmd: returns error", func(t *testing.T) {
		t.Parallel()
		err := Dispatch("cmd:echo hello")
		if err == nil {
			t.Fatal("expected error for cmd: value")
		}
		if !strings.Contains(err.Error(), "cmd:") {
			t.Errorf("error = %q, want contains 'cmd:'", err)
		}
	})

	t.Run("uri: unknown scheme returns error", func(t *testing.T) {
		t.Parallel()
		err := Dispatch("uri:file:///etc/passwd")
		if err == nil {
			t.Fatal("expected error for unknown URI scheme")
		}
		if !strings.Contains(err.Error(), "not allowed") {
			t.Errorf("error = %q, want contains 'not allowed'", err)
		}
	})

	t.Run("action: invalid verb returns error", func(t *testing.T) {
		t.Parallel()
		err := Dispatch("action:rm -rf /")
		if err == nil {
			t.Fatal("expected error for unknown action verb")
		}
		if !strings.Contains(err.Error(), "unknown") {
			t.Errorf("error = %q, want contains 'unknown'", err)
		}
	})
}

func TestRunBuiltin(t *testing.T) {
	t.Parallel()

	t.Run("invalid verb returns error", func(t *testing.T) {
		t.Parallel()
		err := RunBuiltin("restart")
		if err == nil {
			t.Fatal("expected error for unknown verb")
		}
	})

	t.Run("empty verb returns error", func(t *testing.T) {
		t.Parallel()
		err := RunBuiltin("")
		if err == nil {
			t.Fatal("expected error for empty verb")
		}
	})
}

func TestOpenURI(t *testing.T) {
	t.Parallel()

	t.Run("file scheme not in allowlist", func(t *testing.T) {
		t.Parallel()
		err := OpenURI("file:///etc/passwd")
		if err == nil {
			t.Fatal("expected error for file scheme")
		}
		if !strings.Contains(err.Error(), "not allowed") {
			t.Errorf("error = %q, want contains 'not allowed'", err)
		}
	})

	t.Run("empty URI returns error", func(t *testing.T) {
		t.Parallel()
		err := OpenURI("")
		if err == nil {
			t.Fatal("expected error for empty URI")
		}
	})

	t.Run("smb not in allowlist", func(t *testing.T) {
		t.Parallel()
		err := OpenURI("smb://server/share")
		if err == nil {
			t.Fatal("expected error for smb scheme")
		}
	})

	t.Run("path-like value rejected", func(t *testing.T) {
		t.Parallel()
		err := OpenURI("C:\\Windows\\System32\\calc.exe")
		if err == nil {
			t.Fatal("expected error for path-like value")
		}
	})
}

func TestIsURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  bool
	}{
		{"uri:https://example.com", true},
		{"URI:https://example.com", true},
		{"Uri:ms-settings:windowsupdate", true},
		{"uri:", true},
		{"url:https://example.com", false},
		{"cmd:echo hi", false},
		{"action:reboot", false},
		{"restart", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			if got := IsURI(tt.value); got != tt.want {
				t.Errorf("IsURI(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestIsBuiltin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  bool
	}{
		{"action:reboot", true},
		{"ACTION:REBOOT", true},
		{"action:shutdown", true},
		{"action:lock", true},
		{"action:restart", true},
		{"action:", true},
		{"cmd:echo", false},
		{"uri:https://example.com", false},
		{"restart", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			if got := IsBuiltin(tt.value); got != tt.want {
				t.Errorf("IsBuiltin(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestNoShellExecution(t *testing.T) {
	t.Parallel()

	t.Run("cmd prefix functions removed", func(t *testing.T) {
		t.Parallel()
		if ClassifyOn("cmd:echo hello", runtime.GOOS) != KindUnknown {
			t.Error("cmd: should classify as KindUnknown")
		}
		if AllowedOn("cmd:echo hello", runtime.GOOS) {
			t.Error("cmd: should not be allowed")
		}
	})
}
