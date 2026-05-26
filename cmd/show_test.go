package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/config"
)

type mockShowClient struct {
	cfg          *config.NotificationConfig
	deferAllowed bool
	err          error
}

func (m *mockShowClient) GetUIConfig(_ context.Context, _ string) (*config.NotificationConfig, bool, error) {
	if m.err != nil {
		return nil, false, m.err
	}
	return m.cfg, m.deferAllowed, nil
}

func (m *mockShowClient) ReportChoice(_ context.Context, _, _ string) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func TestShow_FullNotification(t *testing.T) {
	t.Parallel()
	c := &mockShowClient{
		cfg: &config.NotificationConfig{
			Heading: "Restart Required",
			Message: "Your workstation requires a restart.",
			Buttons: []config.Button{
				{Label: "Restart now", Value: "restart"},
				{Label: "Defer 1 hour", Value: "defer_1h"},
			},
		},
		deferAllowed: true,
	}

	var buf bytes.Buffer
	err := runShowByEntry(&buf, c, client.ListEntry{ID: "abc123"})
	if err != nil {
		t.Fatalf("runShowByEntry: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Restart Required", "---", "restart", "Actions:", "[1]", "[2]", "Defer 1 hour", "hermes respond <#>"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestShow_NotFound(t *testing.T) {
	t.Parallel()
	c := &mockShowClient{err: fmt.Errorf("not found")}

	var buf bytes.Buffer
	err := runShowByEntry(&buf, c, client.ListEntry{ID: "missing"})
	if err == nil {
		t.Fatal("expected error for not-found notification")
	}
}

func TestShow_DropdownFlattened(t *testing.T) {
	t.Parallel()
	c := &mockShowClient{
		cfg: &config.NotificationConfig{
			Heading: "Restart Required",
			Message: "Please restart.",
			Buttons: []config.Button{
				{Label: "Defer", Dropdown: []config.DropdownOption{
					{Label: "10 Seconds", Value: "defer_10s"},
					{Label: "30 Seconds", Value: "defer_30s"},
				}},
				{Label: "Restart Now", Value: "restart"},
			},
		},
		deferAllowed: true,
	}

	var buf bytes.Buffer
	err := runShowByEntry(&buf, c, client.ListEntry{ID: "abc123"})
	if err != nil {
		t.Fatalf("runShowByEntry: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "()") {
		t.Errorf("output contains empty parens for dropdown container\ngot:\n%s", out)
	}
	for _, want := range []string{"[1]", "10 Seconds", "defer_10s", "[2]", "30 Seconds", "defer_30s", "[3]", "Restart Now", "restart"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestShow_DeferFiltered(t *testing.T) {
	t.Parallel()
	c := &mockShowClient{
		cfg: &config.NotificationConfig{
			Heading: "Restart Required",
			Message: "Restart now.",
			Buttons: []config.Button{
				{Label: "Restart now", Value: "restart"},
				{Label: "Defer 1 hour", Value: "defer_1h"},
				{Label: "Defer 4 hours", Value: "defer_4h"},
			},
		},
		deferAllowed: false,
	}

	var buf bytes.Buffer
	err := runShowByEntry(&buf, c, client.ListEntry{ID: "abc123"})
	if err != nil {
		t.Fatalf("runShowByEntry: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "defer_1h") {
		t.Errorf("output should not contain defer buttons when deferral_allowed=false\ngot:\n%s", out)
	}
	if !strings.Contains(out, "restart") {
		t.Errorf("output should still contain non-defer buttons\ngot:\n%s", out)
	}
}
