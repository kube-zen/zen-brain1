// Package store provides shared storage utilities for Zen components.
// Subpackage sqlite standardizes SQLite driver and connection options across
// zen-brain, zen-ingester, zen-egress, zen-bridge, and zen-protect.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite" // Pure Go driver; CGO_ENABLED=0 compatible
)

// SQLiteOptions configures SQLite connection pragmas.
// All Zen components should use this (via OpenSQLite) for consistent behavior.
type SQLiteOptions struct {
	// JournalMode is "WAL" (default) or "DELETE"
	JournalMode string
	// BusyTimeout in ms; 0 = default 5000
	BusyTimeout int
	// Synchronous: "NORMAL" (default), "FULL", "OFF"
	Synchronous string
	// ReadOnly opens the database read-only (e.g. for legacy migration reads)
	ReadOnly bool
}

// DefaultSQLiteOptions returns recommended defaults for edge and app SQLite DBs.
func DefaultSQLiteOptions() SQLiteOptions {
	return SQLiteOptions{
		JournalMode: "WAL",
		BusyTimeout: 5000,
		Synchronous: "NORMAL",
	}
}

// OpenSQLite opens a SQLite database using the zen-sdk standard driver (modernc.org/sqlite).
// Use this in zen-brain, zen-ingester, zen-egress, zen-bridge, and zen-protect so all
// components share one driver and consistent pragmas. Builds with CGO_ENABLED=0.
func OpenSQLite(ctx context.Context, path string, opts *SQLiteOptions) (*sql.DB, error) {
	if opts == nil {
		o := DefaultSQLiteOptions()
		opts = &o
	}
	journalMode := opts.JournalMode
	if journalMode == "" {
		journalMode = "WAL"
	}
	busyTimeout := opts.BusyTimeout
	if busyTimeout <= 0 {
		busyTimeout = 5000
	}
	sync := opts.Synchronous
	if sync == "" {
		sync = "NORMAL"
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("_pragma=journal_mode(%s)", journalMode))
	parts = append(parts, fmt.Sprintf("_pragma=busy_timeout(%d)", busyTimeout))
	parts = append(parts, fmt.Sprintf("_pragma=synchronous(%s)", sync))
	if opts.ReadOnly {
		parts = append(parts, "_pragma=query_only(1)")
	}
	dsn := path + "?" + strings.Join(parts, "&")

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}
	// Reasonable defaults for long-lived connections
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	return db, nil
}

// OpenSQLiteSimple opens with default options (WAL, busy_timeout=5000, synchronous=NORMAL).
// Convenience when you don't need to customize.
func OpenSQLiteSimple(ctx context.Context, path string) (*sql.DB, error) {
	return OpenSQLite(ctx, path, nil)
}
