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
		{"zero", 0, "Hermes — no notifications need your attention"},
		{"one", 1, "Hermes — 1 notification needs your attention"},
		{"plural", 3, "Hermes — 3 notifications need your attention"},
		{"nine", 9, "Hermes — 9 notifications need your attention"},
		{"at capacity", 10, "Hermes — at capacity (10 need attention)"},
		{"over capacity", 15, "Hermes — at capacity (15 need attention)"},
		{"negative clamped", -1, "Hermes — no notifications need your attention"},
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
		{"zero", 0, "Notification History"},
		{"one", 1, "Notification History (1)"},
		{"many", 5, "Notification History (5)"},
		{"negative", -1, "Notification History"},
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
	got := InboxArgs()
	want := []string{"inbox"}
	if len(got) != len(want) {
		t.Fatalf("InboxArgs() len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("InboxArgs()[%d] = %q, want %q", i, got[i], want[i])
		}
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
