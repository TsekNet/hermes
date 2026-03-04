package urischeme

import "testing"

func TestAllowedOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		goos string
		want bool
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := AllowedOn(tt.url, tt.goos); got != tt.want {
				t.Errorf("AllowedOn(%q, %q) = %v, want %v", tt.url, tt.goos, got, tt.want)
			}
		})
	}
}

func TestClassifyOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		goos string
		want Kind
	}{
		{"https is web", "https://example.com", "linux", KindWeb},
		{"http is web", "http://example.com", "windows", KindWeb},
		{"ms-settings is settings", "ms-settings:windowsupdate", "windows", KindSettings},
		{"apple prefs is settings", "x-apple.systempreferences:com.apple.preference.security", "darwin", KindSettings},
		{"ms-settings wrong platform", "ms-settings:windowsupdate", "darwin", KindUnknown},
		{"unknown scheme", "ftp://x.com", "linux", KindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ClassifyOn(tt.url, tt.goos); got != tt.want {
				t.Errorf("ClassifyOn(%q, %q) = %v, want %v", tt.url, tt.goos, got, tt.want)
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

	if len(win) != 3 {
		t.Errorf("windows schemes = %d, want 3 (https, http, ms-settings)", len(win))
	}
	if len(mac) != 3 {
		t.Errorf("darwin schemes = %d, want 3 (https, http, x-apple.systempreferences)", len(mac))
	}
	if len(lin) != 2 {
		t.Errorf("linux schemes = %d, want 2 (https, http)", len(lin))
	}
}
