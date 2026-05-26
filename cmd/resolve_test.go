package cmd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/TsekNet/hermes/internal/client"
)

type mockLister struct {
	entries []client.ListEntry
	err     error
}

func (m *mockLister) List(_ context.Context) ([]client.ListEntry, error) {
	return m.entries, m.err
}

func TestResolveID(t *testing.T) {
	t.Parallel()
	now := time.Now()
	entries := []client.ListEntry{
		{ID: "bbbb0000", CreatedAt: now.Add(1 * time.Second)},
		{ID: "aaaa0000", CreatedAt: now},
		{ID: "cccc0000", CreatedAt: now.Add(2 * time.Second)},
	}

	tests := []struct {
		name    string
		arg     string
		want    string
		wantErr bool
	}{
		{name: "position 1 resolves to earliest", arg: "1", want: "aaaa0000"},
		{name: "position 2 resolves to middle", arg: "2", want: "bbbb0000"},
		{name: "position 3 resolves to latest", arg: "3", want: "cccc0000"},
		{name: "hex ID passes through", arg: "abc123def456", want: "abc123def456"},
		{name: "16-char hex passes through", arg: "1234567890123456", want: "1234567890123456"},
		{name: "position 0 is invalid", arg: "0", wantErr: true},
		{name: "position out of range", arg: "99", wantErr: true},
		{name: "negative is invalid", arg: "-1", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := &mockLister{entries: entries}
			got, err := resolveID(context.Background(), l, tc.arg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveID_ListError(t *testing.T) {
	t.Parallel()
	l := &mockLister{err: fmt.Errorf("connection refused")}
	_, err := resolveID(context.Background(), l, "1")
	if err == nil {
		t.Fatal("expected error when list fails")
	}
}

func TestSortEntries_Tiebreaker(t *testing.T) {
	t.Parallel()
	now := time.Now()
	entries := []client.ListEntry{
		{ID: "zzzz", CreatedAt: now},
		{ID: "aaaa", CreatedAt: now},
	}
	sortEntries(entries)
	if entries[0].ID != "aaaa" {
		t.Errorf("first entry = %q, want aaaa", entries[0].ID)
	}
}
