package respond

import (
	"context"
	"fmt"
	"testing"

	"github.com/TsekNet/hermes/internal/config"
)

type mockClient struct {
	cfg            *config.NotificationConfig
	deferAllowed   bool
	getErr         error
	reportAccepted bool
	reportErr      error
	reportedID     string
	reportedValue  string
}

func (m *mockClient) GetUIConfig(_ context.Context, _ string) (*config.NotificationConfig, bool, error) {
	if m.getErr != nil {
		return nil, false, m.getErr
	}
	return m.cfg, m.deferAllowed, nil
}

func (m *mockClient) ReportChoice(_ context.Context, id, value string) (bool, error) {
	m.reportedID = id
	m.reportedValue = value
	return m.reportAccepted, m.reportErr
}

func testConfig() *config.NotificationConfig {
	return &config.NotificationConfig{
		Heading: "Restart Required",
		Message: "Please restart.",
		Buttons: []config.Button{
			{Label: "Restart now", Value: "restart"},
			{Label: "Defer 1 hour", Value: "defer_1h"},
		},
	}
}

func TestSubmit_Accepted(t *testing.T) {
	t.Parallel()
	c := &mockClient{cfg: testConfig(), deferAllowed: true, reportAccepted: true}

	r, err := Submit(context.Background(), c, "abc", "restart")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if r.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", r.ExitCode)
	}
	if c.reportedValue != "restart" {
		t.Errorf("reported value = %q, want restart", c.reportedValue)
	}
}

func TestSubmit_DeferValue(t *testing.T) {
	t.Parallel()
	c := &mockClient{cfg: testConfig(), deferAllowed: true, reportAccepted: true}

	r, err := Submit(context.Background(), c, "abc", "defer_1h")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if r.ExitCode != 200 {
		t.Errorf("exit code = %d, want 200", r.ExitCode)
	}
}

func TestSubmit_InvalidValue(t *testing.T) {
	t.Parallel()
	c := &mockClient{cfg: testConfig(), deferAllowed: true}

	_, err := Submit(context.Background(), c, "abc", "bogus")
	if err == nil {
		t.Fatal("expected error for invalid value")
	}
	if c.reportedID != "" {
		t.Error("ReportChoice should not have been called")
	}
}

func TestSubmit_DeferFilteredOut(t *testing.T) {
	t.Parallel()
	c := &mockClient{cfg: testConfig(), deferAllowed: false}

	_, err := Submit(context.Background(), c, "abc", "defer_1h")
	if err == nil {
		t.Fatal("expected error when defer is filtered out")
	}
}

func TestSubmit_NotFound(t *testing.T) {
	t.Parallel()
	c := &mockClient{getErr: fmt.Errorf("not found")}

	_, err := Submit(context.Background(), c, "abc", "restart")
	if err == nil {
		t.Fatal("expected error for not-found notification")
	}
}

func TestSubmit_ReportRejected(t *testing.T) {
	t.Parallel()
	c := &mockClient{cfg: testConfig(), deferAllowed: true, reportAccepted: false}

	_, err := Submit(context.Background(), c, "abc", "restart")
	if err == nil {
		t.Fatal("expected error when choice not accepted")
	}
}

func TestFetchConfig_FiltersDeferButtons(t *testing.T) {
	t.Parallel()
	c := &mockClient{cfg: testConfig(), deferAllowed: false}

	_, buttons, err := FetchConfig(context.Background(), c, "abc")
	if err != nil {
		t.Fatalf("FetchConfig: %v", err)
	}
	for _, b := range buttons {
		if b.Value == "defer_1h" {
			t.Error("defer button should have been filtered")
		}
	}
	if len(buttons) != 1 {
		t.Errorf("expected 1 button after filtering, got %d", len(buttons))
	}
}
