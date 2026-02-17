package exitcodes

import "testing"

func TestForValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value string
		want  int32
	}{
		{"ok", OK},
		{"restart", OK},
		{"", OK},
		{"defer", Deferred},
		{"defer_4h", Deferred},
		{"defer_1d", Deferred},
		{"timeout:restart", Timeout},
		{"timeout", Timeout},
		{"timeout:", Timeout},
		{"cancelled", Error},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			if got := ForValue(tt.value); got != tt.want {
				t.Errorf("ForValue(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}
