package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/TsekNet/hermes/internal/config"
)

type mockRespondClient struct {
	cfg            *config.NotificationConfig
	deferAllowed   bool
	getErr         error
	reportAccepted bool
	reportErr      error
	reportedID     string
	reportedValue  string
}

func (m *mockRespondClient) GetUIConfig(_ context.Context, _ string) (*config.NotificationConfig, bool, error) {
	if m.getErr != nil {
		return nil, false, m.getErr
	}
	return m.cfg, m.deferAllowed, nil
}

func (m *mockRespondClient) ReportChoice(_ context.Context, id, value string) (bool, error) {
	m.reportedID = id
	m.reportedValue = value
	return m.reportAccepted, m.reportErr
}

func newTestConfig() *config.NotificationConfig {
	return &config.NotificationConfig{
		Heading: "Restart Required",
		Message: "Please restart.",
		Buttons: []config.Button{
			{Label: "Restart now", Value: "restart"},
			{Label: "Defer 1 hour", Value: "defer_1h"},
			{Label: "Defer 4 hours", Value: "defer_4h"},
		},
	}
}

func TestRespond_AcceptedChoice(t *testing.T) {
	t.Parallel()
	c := &mockRespondClient{
		cfg:            newTestConfig(),
		deferAllowed:   true,
		reportAccepted: true,
	}

	var buf bytes.Buffer
	code, err := runRespond(&buf, nil, c, "abc123", "restart", false)
	if err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if c.reportedID != "abc123" || c.reportedValue != "restart" {
		t.Errorf("ReportChoice called with (%q, %q), want (abc123, restart)", c.reportedID, c.reportedValue)
	}
	if !strings.Contains(buf.String(), "Sent: restart") {
		t.Errorf("output missing 'Sent: restart'\ngot: %s", buf.String())
	}
}

func TestRespond_DeferChoice(t *testing.T) {
	t.Parallel()
	c := &mockRespondClient{
		cfg:            newTestConfig(),
		deferAllowed:   true,
		reportAccepted: true,
	}

	var buf bytes.Buffer
	code, err := runRespond(&buf, nil, c, "abc123", "defer_1h", false)
	if err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	if code != 200 {
		t.Errorf("exit code = %d, want 200", code)
	}
}

func TestRespond_NotFound(t *testing.T) {
	t.Parallel()
	c := &mockRespondClient{getErr: fmt.Errorf("notification not found")}

	var buf bytes.Buffer
	_, err := runRespond(&buf, nil, c, "missing", "restart", false)
	if err == nil {
		t.Fatal("expected error for not-found notification")
	}
}

func TestRespond_InvalidValue(t *testing.T) {
	t.Parallel()
	c := &mockRespondClient{
		cfg:          newTestConfig(),
		deferAllowed: true,
	}

	var buf bytes.Buffer
	_, err := runRespond(&buf, nil, c, "abc123", "bogus_value", false)
	if err == nil {
		t.Fatal("expected error for invalid value")
	}
	if c.reportedID != "" {
		t.Error("ReportChoice should not have been called for invalid value")
	}
}

func TestRespond_NoValueNoTTY(t *testing.T) {
	t.Parallel()
	c := &mockRespondClient{
		cfg:          newTestConfig(),
		deferAllowed: true,
	}

	var buf bytes.Buffer
	code, err := runRespond(&buf, strings.NewReader(""), c, "abc123", "", false)
	if err == nil {
		t.Fatal("expected error for no-value no-TTY")
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	out := buf.String()
	if !strings.Contains(out, "no value provided") {
		t.Errorf("expected usage hint\ngot: %s", out)
	}
	if !strings.Contains(out, "restart") {
		t.Errorf("expected available actions in output\ngot: %s", out)
	}
}

func TestRespond_DeferFiltered(t *testing.T) {
	t.Parallel()
	c := &mockRespondClient{
		cfg:          newTestConfig(),
		deferAllowed: false,
	}

	var buf bytes.Buffer
	_, err := runRespond(&buf, nil, c, "abc123", "defer_1h", false)
	if err == nil {
		t.Fatal("expected error: defer value should be rejected when deferral_allowed=false")
	}
}

func TestRespond_InteractiveValidChoice(t *testing.T) {
	t.Parallel()
	c := &mockRespondClient{
		cfg:            newTestConfig(),
		deferAllowed:   true,
		reportAccepted: true,
	}

	var buf bytes.Buffer
	input := strings.NewReader("1\n")
	code, err := runRespond(&buf, input, c, "abc123", "", true)
	if err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if c.reportedValue != "restart" {
		t.Errorf("reported value = %q, want restart", c.reportedValue)
	}
}

func TestRespond_InteractiveDropdownChoice(t *testing.T) {
	t.Parallel()
	c := &mockRespondClient{
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
		deferAllowed:   true,
		reportAccepted: true,
	}

	var buf bytes.Buffer
	input := strings.NewReader("2\n")
	code, err := runRespond(&buf, input, c, "abc123", "", true)
	if err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	if code != 200 {
		t.Errorf("exit code = %d, want 200 (defer)", code)
	}
	if c.reportedValue != "defer_30s" {
		t.Errorf("reported value = %q, want defer_30s", c.reportedValue)
	}
}

func TestRespond_InteractiveInvalidThenValid(t *testing.T) {
	t.Parallel()
	c := &mockRespondClient{
		cfg:            newTestConfig(),
		deferAllowed:   true,
		reportAccepted: true,
	}

	var buf bytes.Buffer
	input := strings.NewReader("0\nabc\n9\n2\n")
	code, err := runRespond(&buf, input, c, "abc123", "", true)
	if err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	if code != 200 {
		t.Errorf("exit code = %d, want 200 (defer)", code)
	}
	if c.reportedValue != "defer_1h" {
		t.Errorf("reported value = %q, want defer_1h", c.reportedValue)
	}
	out := buf.String()
	if strings.Count(out, "Invalid choice") != 3 {
		t.Errorf("expected 3 invalid-choice messages\ngot:\n%s", out)
	}
}

func TestRespond_InteractiveMaxRetries(t *testing.T) {
	t.Parallel()
	c := &mockRespondClient{
		cfg:          newTestConfig(),
		deferAllowed: true,
	}

	var buf bytes.Buffer
	input := strings.NewReader("x\nx\nx\nx\nx\n")
	code, err := runRespond(&buf, input, c, "abc123", "", true)
	if err == nil {
		t.Fatal("expected error after max retries")
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(buf.String(), "Too many invalid attempts") {
		t.Errorf("expected 'Too many invalid attempts' message\ngot:\n%s", buf.String())
	}
}

func TestRespond_InteractiveEOF(t *testing.T) {
	t.Parallel()
	c := &mockRespondClient{
		cfg:          newTestConfig(),
		deferAllowed: true,
	}

	var buf bytes.Buffer
	input := strings.NewReader("")
	code, _ := runRespond(&buf, input, c, "abc123", "", true)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 on EOF", code)
	}
}
