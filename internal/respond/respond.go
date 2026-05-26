package respond

import (
	"context"
	"fmt"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/exitcodes"
)

// Client is the subset of client.Client used by Submit.
type Client interface {
	GetUIConfig(ctx context.Context, id string) (*config.NotificationConfig, bool, error)
	ReportChoice(ctx context.Context, id, value string) (bool, error)
}

// Result holds the outcome of a Submit call.
type Result struct {
	Config   *config.NotificationConfig
	Buttons  []config.Button
	Value    string
	ExitCode int32
}

// Submit validates and sends a user response for a notification.
// It fetches the config, filters defer buttons if exhausted, validates
// the value against known buttons, and calls ReportChoice.
func Submit(ctx context.Context, c Client, id, value string) (*Result, error) {
	cfg, deferAllowed, err := c.GetUIConfig(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get config for %s: %w", id, err)
	}

	buttons := cfg.Buttons
	if !deferAllowed {
		buttons = config.FilterDeferButtons(buttons)
	}
	cfg.Buttons = buttons

	if !cfg.HasValue(value) {
		return &Result{Config: cfg, Buttons: buttons}, fmt.Errorf("unknown value %q", value)
	}

	accepted, err := c.ReportChoice(ctx, id, value)
	if err != nil {
		return nil, fmt.Errorf("report choice: %w", err)
	}
	if !accepted {
		return nil, fmt.Errorf("choice %q not accepted by service", value)
	}

	return &Result{
		Config:   cfg,
		Buttons:  buttons,
		Value:    value,
		ExitCode: exitcodes.ForValue(value),
	}, nil
}

// FetchConfig fetches and prepares the notification config for display.
// Returns the config with filtered buttons if deferrals are exhausted.
func FetchConfig(ctx context.Context, c Client, id string) (*config.NotificationConfig, []config.Button, error) {
	cfg, deferAllowed, err := c.GetUIConfig(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get config for %s: %w", id, err)
	}

	buttons := cfg.Buttons
	if !deferAllowed {
		buttons = config.FilterDeferButtons(buttons)
	}
	cfg.Buttons = buttons

	return cfg, buttons, nil
}
