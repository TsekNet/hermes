package dnd

import "testing"

func TestActive_ReturnsWithoutPanic(t *testing.T) {
	t.Parallel()
	// Active() should never panic regardless of platform or environment.
	// On CI/headless systems it should return false (fail-open).
	got := Active()
	t.Logf("dnd.Active() = %v", got)
}
