package config

import (
	"testing"
	"time"
)

func TestParseDeferValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  time.Duration
	}{
		{"defer_1h", 1 * time.Hour},
		{"defer_4h", 4 * time.Hour},
		{"defer_24h", 24 * time.Hour},
		{"defer_1d", 24 * time.Hour},
		{"defer_7d", 7 * 24 * time.Hour},
		{"defer_30m", 30 * time.Minute},
		{"defer_15m", 15 * time.Minute},
		{"defer", 0},           // plain defer, no duration
		{"defer_", 0},          // malformed
		{"defer_0h", 0},        // zero hours
		{"restart", 0},         // not a defer
		{"", 0},                // empty
		{"defer_abc", 0},       // non-numeric
		{"defer_1x", 0},         // unknown unit
		{"defer_10s", 10 * time.Second},
		{"defer_3000000h", 0},   // overflow: would wrap int64 ns
		{"defer_200000000m", 0}, // overflow: minutes
		{"defer_200000d", 0},    // overflow: days
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := ParseDeferValue(tt.input)
			if got != tt.want {
				t.Errorf("ParseDeferValue(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDeadline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  time.Duration
	}{
		{"24h", 24 * time.Hour},
		{"1h", 1 * time.Hour},
		{"30m", 30 * time.Minute},
		{"1h30m", 90 * time.Minute},
		{"7d", 7 * 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{"", 0},
		{"   ", 0},
		{"abc", 0},
		{"0d", 0},
		{"200000d", 0}, // overflow guard
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := ParseDeadline(tt.input)
			if got != tt.want {
				t.Errorf("ParseDeadline(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
