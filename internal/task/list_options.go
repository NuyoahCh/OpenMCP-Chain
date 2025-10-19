package task

import (
	"strings"
	"time"
)

// SortOrder defines how results should be ordered when listing tasks.
type SortOrder int

const (
	// SortByUpdatedDesc orders tasks by UpdatedAt descending (most recent first).
	SortByUpdatedDesc SortOrder = iota
	// SortByUpdatedAsc orders tasks by UpdatedAt ascending (oldest first).
	SortByUpdatedAsc
)

// ListOptions controls how tasks are selected when querying the store.
type ListOptions struct {
	Limit      int
	Offset     int
	Statuses   []Status
	UpdatedGTE int64
	UpdatedLTE int64
	HasResult  *bool
	Order      SortOrder
	Query      string
}

// applyDefaults sanitizes the options and fills in default values.
func (opts *ListOptions) applyDefaults() {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	if opts.Statuses != nil {
		opts.Statuses = normalizeStatuses(opts.Statuses)
	}
	if opts.Order != SortByUpdatedAsc {
		opts.Order = SortByUpdatedDesc
	}
	opts.Query = strings.TrimSpace(opts.Query)
}

// ListOption mutates ListOptions.
type ListOption func(*ListOptions)

// WithLimit limits the number of tasks returned.
func WithLimit(limit int) ListOption {
	return func(opts *ListOptions) {
		opts.Limit = limit
	}
}

// WithOffset skips the first n matching tasks before returning results.
func WithOffset(offset int) ListOption {
	return func(opts *ListOptions) {
		opts.Offset = offset
	}
}

// WithStatuses filters tasks by the provided statuses.
func WithStatuses(statuses ...Status) ListOption {
	return func(opts *ListOptions) {
		opts.Statuses = append(opts.Statuses[:0], statuses...)
	}
}

// WithUpdatedSince filters tasks updated after the provided instant (inclusive).
func WithUpdatedSince(ts time.Time) ListOption {
	return func(opts *ListOptions) {
		if ts.IsZero() {
			opts.UpdatedGTE = 0
			return
		}
		opts.UpdatedGTE = ts.Unix()
	}
}

// WithUpdatedUntil filters tasks updated before the provided instant (inclusive).
func WithUpdatedUntil(ts time.Time) ListOption {
	return func(opts *ListOptions) {
		if ts.IsZero() {
			opts.UpdatedLTE = 0
			return
		}
		opts.UpdatedLTE = ts.Unix()
	}
}

// WithResultPresence filters tasks by whether they already contain execution results.
func WithResultPresence(hasResult bool) ListOption {
	return func(opts *ListOptions) {
		opts.HasResult = new(bool)
		*opts.HasResult = hasResult
	}
}

// WithSortOrder changes the returned order of tasks.
func WithSortOrder(order SortOrder) ListOption {
	return func(opts *ListOptions) {
		opts.Order = order
	}
}

// WithQuery filters tasks by fuzzy matching across goal, address and result fields.
func WithQuery(query string) ListOption {
	return func(opts *ListOptions) {
		opts.Query = query
	}
}

// buildListOptions applies option functions on top of defaults.
func buildListOptions(opts []ListOption) ListOptions {
	options := ListOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	options.applyDefaults()
	return options
}

func normalizeStatuses(input []Status) []Status {
	if len(input) == 0 {
		return nil
	}
	seen := make(map[Status]struct{}, len(input))
	result := make([]Status, 0, len(input))
	for _, status := range input {
		if !IsValidStatus(status) {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		result = append(result, status)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
