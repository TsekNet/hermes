package cmd

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/TsekNet/hermes/internal/client"
)

// Lister fetches active notifications for position-based ID resolution.
type Lister interface {
	List(ctx context.Context) ([]client.ListEntry, error)
}

// sortEntries sorts list entries by CreatedAt ascending, then ID as tiebreaker.
// This ensures position numbers are stable across calls.
func sortEntries(entries []client.ListEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].CreatedAt.Equal(entries[j].CreatedAt) {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].CreatedAt.Before(entries[j].CreatedAt)
	})
}

// resolveID converts a user-provided argument to a real notification ID.
// If arg parses as a positive integer under 16 digits, it's treated as a
// 1-based position from "hermes list". Otherwise it's a passthrough ID.
func resolveID(ctx context.Context, l Lister, arg string) (string, error) {
	n, err := strconv.Atoi(arg)
	if err != nil || len(arg) >= 16 {
		return arg, nil
	}
	if n < 1 {
		return "", fmt.Errorf("position must be >= 1, got %d", n)
	}

	entries, err := l.List(ctx)
	if err != nil {
		return "", fmt.Errorf("list notifications: %w", err)
	}
	sortEntries(entries)

	if n > len(entries) {
		return "", fmt.Errorf("position %d out of range (have %d active)", n, len(entries))
	}
	return entries[n-1].ID, nil
}
