package cmd

import (
	"testing"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/store"
)

func TestServeCmd_Flags(t *testing.T) {
	t.Parallel()
	cmd := serveCmd()

	if cmd.Use != "serve" {
		t.Errorf("Use = %q, want serve", cmd.Use)
	}

	db := cmd.Flags().Lookup("db")
	if db == nil {
		t.Fatal("missing --db flag")
	}
	noTray := cmd.Flags().Lookup("no-tray")
	if noTray == nil {
		t.Fatal("missing --no-tray flag")
	}
}

func TestSortByDependencies(t *testing.T) {
	t.Parallel()

	qr := func(id, dependsOn string) *store.QueueRecord {
		return &store.QueueRecord{
			ID:     id,
			Config: &config.NotificationConfig{ID: id, DependsOn: dependsOn},
		}
	}

	tests := []struct {
		name    string
		input   []*store.QueueRecord
		wantIDs []string
	}{
		{
			name:    "empty input",
			input:   nil,
			wantIDs: nil,
		},
		{
			name:    "single item",
			input:   []*store.QueueRecord{qr("a", "")},
			wantIDs: []string{"a"},
		},
		{
			name:    "no dependencies preserves order",
			input:   []*store.QueueRecord{qr("a", ""), qr("b", ""), qr("c", "")},
			wantIDs: []string{"a", "b", "c"},
		},
		{
			name:    "linear chain reordered",
			input:   []*store.QueueRecord{qr("c", "b"), qr("b", "a"), qr("a", "")},
			wantIDs: []string{"a", "b", "c"},
		},
		{
			name:    "diamond dependency",
			input:   []*store.QueueRecord{qr("d", "b"), qr("c", "a"), qr("b", "a"), qr("a", "")},
			wantIDs: []string{"a", "b", "d", "c"},
		},
		{
			name:    "dependency on item not in queue",
			input:   []*store.QueueRecord{qr("b", "missing"), qr("a", "")},
			wantIDs: []string{"b", "a"},
		},
		{
			name:    "mutual cycle does not hang",
			input:   []*store.QueueRecord{qr("a", "b"), qr("b", "a")},
			wantIDs: []string{"b", "a"},
		},
		{
			name:    "self-referencing dependency",
			input:   []*store.QueueRecord{qr("a", "a")},
			wantIDs: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sortByDependencies(tt.input)

			if len(got) != len(tt.input) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.input))
			}

			if tt.wantIDs == nil {
				return
			}

			for i, want := range tt.wantIDs {
				if got[i].ID != want {
					ids := make([]string, len(got))
					for j, r := range got {
						ids[j] = r.ID
					}
					t.Fatalf("order = %v, want %v", ids, tt.wantIDs)
				}
			}
		})
	}
}
