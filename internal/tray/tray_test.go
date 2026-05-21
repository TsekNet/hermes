package tray

import (
	"testing"
)

func TestFormatTooltip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		count int
		want  string
	}{
		{"zero", 0, "Hermes — no pending notifications"},
		{"one", 1, "Hermes — 1 pending notification"},
		{"plural", 3, "Hermes — 3 pending notifications"},
		{"nine", 9, "Hermes — 9 pending notifications"},
		{"at capacity", 10, "Hermes — at capacity (10 pending)"},
		{"over capacity", 15, "Hermes — at capacity (15 pending)"},
		{"negative clamped", -1, "Hermes — no pending notifications"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := FormatTooltip(tt.count); got != tt.want {
				t.Errorf("FormatTooltip(%d) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

func TestFormatInboxLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		count int
		want  string
	}{
		{"zero", 0, "Open Inbox"},
		{"one", 1, "Open Inbox (Pending: 1)"},
		{"many", 5, "Open Inbox (Pending: 5)"},
		{"negative", -1, "Open Inbox"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := FormatInboxLabel(tt.count); got != tt.want {
				t.Errorf("FormatInboxLabel(%d) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

func TestShouldEnable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		goos string
		env  map[string]string
		want bool
	}{
		{"windows always", "windows", nil, true},
		{"darwin always", "darwin", nil, true},
		{"linux with DISPLAY", "linux", map[string]string{"DISPLAY": ":0"}, true},
		{"linux with WAYLAND_DISPLAY", "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, true},
		{"linux with both", "linux", map[string]string{"DISPLAY": ":0", "WAYLAND_DISPLAY": "wayland-0"}, true},
		{"linux headless", "linux", map[string]string{}, false},
		{"linux empty DISPLAY", "linux", map[string]string{"DISPLAY": ""}, false},
		{"linux empty WAYLAND", "linux", map[string]string{"WAYLAND_DISPLAY": ""}, false},
		{"freebsd with DISPLAY", "freebsd", map[string]string{"DISPLAY": ":0"}, true},
		{"freebsd headless", "freebsd", map[string]string{}, false},
		{"unknown os", "plan9", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			envLookup := func(key string) string {
				if tt.env == nil {
					return ""
				}
				return tt.env[key]
			}
			if got := ShouldEnable(tt.goos, envLookup); got != tt.want {
				t.Errorf("ShouldEnable(%q, ...) = %v, want %v", tt.goos, got, tt.want)
			}
		})
	}
}

func TestInboxArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		selfPath string
		port     int
		want     []string
	}{
		{"default port", "/usr/local/bin/hermes", 4770, []string{"inbox", "--port", "4770"}},
		{"custom port", "/opt/hermes/hermes", 5000, []string{"inbox", "--port", "5000"}},
		{"windows path", `C:\Program Files\Hermes\hermes.exe`, 4770, []string{"inbox", "--port", "4770"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := InboxArgs(tt.port)
			if len(got) != len(tt.want) {
				t.Fatalf("InboxArgs(%d) len = %d, want %d", tt.port, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("InboxArgs(%d)[%d] = %q, want %q", tt.port, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIconEmbedded(t *testing.T) {
	t.Parallel()
	data := IconBytes()
	if len(data) == 0 {
		t.Fatal("IconBytes() returned empty data")
	}
	// On Linux/macOS: PNG magic (0x89 P N G)
	// On Windows: ICO magic (0x00 0x00 0x01 0x00)
	// Just verify non-empty and has a known magic prefix.
	if data[0] == 0x89 && data[1] == 'P' {
		// PNG
	} else if data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x01 {
		// ICO
	} else {
		t.Errorf("IconBytes() has unknown magic: %x %x %x %x", data[0], data[1], data[2], data[3])
	}
}
