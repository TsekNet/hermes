package app

import (
	"context"
	"testing"

	"github.com/TsekNet/hermes/internal/config"
)

func TestNew(t *testing.T) {
	t.Parallel()
	cfg := &config.NotificationConfig{Heading: "Test", Message: "Body"}
	a := New(cfg)
	if a.cfg != cfg {
		t.Error("cfg not set")
	}
	if !a.deferAllowed {
		t.Error("deferAllowed should be true in local mode")
	}
	if a.cfg.Platform == "" {
		t.Error("platform should be set")
	}
}

func TestNewWithGRPC(t *testing.T) {
	t.Parallel()
	cfg := &config.NotificationConfig{Heading: "Test", Message: "Body"}
	a := NewWithGRPC(cfg, nil, "notif-1", false)

	if a.notificationID != "notif-1" {
		t.Errorf("notificationID = %q", a.notificationID)
	}
	if a.deferAllowed {
		t.Error("deferAllowed should be false")
	}
	if a.cfg.Platform == "" {
		t.Error("platform should be set")
	}
}

func TestGetConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.NotificationConfig{Heading: "H", Message: "M"}
	a := New(cfg)
	if got := a.GetConfig(); got.Heading != "H" {
		t.Errorf("heading = %q", got.Heading)
	}
}

func TestDeferralAllowed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		local bool
		grpc  bool
		want  bool
	}{
		{"local mode", true, false, true},
		{"grpc allowed", false, true, true},
		{"grpc blocked", false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config.NotificationConfig{Heading: "H", Message: "M"}
			var a *App
			if tt.local {
				a = New(cfg)
			} else {
				a = NewWithGRPC(cfg, nil, "id", tt.grpc)
			}
			if got := a.DeferralAllowed(); got != tt.want {
				t.Errorf("DeferralAllowed = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStartup(t *testing.T) {
	t.Parallel()
	a := New(&config.NotificationConfig{Heading: "H", Message: "M"})
	ctx := context.Background()
	a.Startup(ctx)
	if a.ctx != ctx {
		t.Error("context not set")
	}
}

func TestShutdown_NilWatcher(t *testing.T) {
	t.Parallel()
	a := New(&config.NotificationConfig{Heading: "H", Message: "M"})
	a.Shutdown(context.Background())
}

func TestShutdown_Idempotent(t *testing.T) {
	t.Parallel()
	a := New(&config.NotificationConfig{Heading: "H", Message: "M"})
	a.Shutdown(context.Background())
	a.Shutdown(context.Background())
}

func TestHeight(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		images []string
		want   int
	}{
		{"no images", nil, WindowHeight},
		{"empty images", []string{}, WindowHeight},
		{"with images", []string{"https://example.com/slide.png"}, WindowHeightImages},
		{"multiple images", []string{"https://a.com/1.png", "https://b.com/2.png"}, WindowHeightImages},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config.NotificationConfig{Heading: "H", Message: "M", Images: tt.images}
			if got := Height(cfg); got != tt.want {
				t.Errorf("Height() = %d, want %d", got, tt.want)
			}
		})
	}
}
