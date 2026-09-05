// Package taiwanwatchlist persists the user's saved Taiwan securities (自選股).
//
// The store deliberately does not depend on the Taiwan provider/foundation types —
// it only persists the small, stable identity fields a watchlist entry needs
// (canonical symbol, code, name, exchange, security type, created_at). Resolving
// and validating that identity against the live Taiwan directory is the HTTP
// layer's job (backend/internal/httpapi), which already has that provider wired.
package taiwanwatchlist

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Entry is one saved Taiwan security. Canonical is the unique identity
// ({code}.{TWSE|TPEX}) — code alone is not unique across exchanges.
type Entry struct {
	Canonical    string
	Code         string
	Name         string
	Exchange     string
	SecurityType string
	CreatedAt    time.Time
}

type Store struct {
	db *sql.DB
}

// OpenStore opens (creating if needed) the SQLite-backed watchlist database at path.
// path == "" or ":memory:" opens a private in-memory database, matching the other
// Taiwan/portfolio stores in this codebase.
func OpenStore(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = ":memory:"
	}
	dataSource := path
	if path == ":memory:" {
		dataSource = fmt.Sprintf("file:easy-stock-taiwan-watchlist-%d?mode=memory&cache=shared", time.Now().UnixNano())
	} else if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create taiwan watchlist data directory: %w", err)
	}
	db, err := sql.Open("sqlite", dataSource)
	if err != nil {
		return nil, fmt.Errorf("open taiwan watchlist database: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS taiwan_watchlist (
			canonical TEXT PRIMARY KEY,
			code TEXT NOT NULL,
			name TEXT NOT NULL,
			exchange TEXT NOT NULL,
			security_type TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS taiwan_watchlist_created ON taiwan_watchlist(created_at ASC);
	`)
	if err != nil {
		return fmt.Errorf("migrate taiwan watchlist database: %w", err)
	}
	return nil
}

// Add saves a security, resolving the canonical symbol case (upper) and defaulting
// CreatedAt to now if unset. Adding an already-present canonical symbol is a no-op —
// the existing row (and its original CreatedAt) is kept, then returned.
func (s *Store) Add(ctx context.Context, entry Entry) (Entry, error) {
	canonical := strings.ToUpper(strings.TrimSpace(entry.Canonical))
	if canonical == "" {
		return Entry{}, fmt.Errorf("canonical symbol is required")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO taiwan_watchlist (canonical, code, name, exchange, security_type, created_at)
		VALUES (?,?,?,?,?,?) ON CONFLICT(canonical) DO NOTHING`,
		canonical, entry.Code, entry.Name, entry.Exchange, entry.SecurityType, formatTime(entry.CreatedAt))
	if err != nil {
		return Entry{}, fmt.Errorf("save taiwan watchlist entry: %w", err)
	}
	return s.Get(ctx, canonical)
}

// Remove deletes a saved security by canonical symbol. Removing an absent symbol
// is a no-op success (idempotent), not an error.
func (s *Store) Remove(ctx context.Context, canonical string) error {
	canonical = strings.ToUpper(strings.TrimSpace(canonical))
	if _, err := s.db.ExecContext(ctx, `DELETE FROM taiwan_watchlist WHERE canonical=?`, canonical); err != nil {
		return fmt.Errorf("remove taiwan watchlist entry: %w", err)
	}
	return nil
}

// Get returns one saved entry by canonical symbol, or sql.ErrNoRows if absent.
func (s *Store) Get(ctx context.Context, canonical string) (Entry, error) {
	canonical = strings.ToUpper(strings.TrimSpace(canonical))
	var entry Entry
	var createdAt string
	err := s.db.QueryRowContext(ctx, `SELECT canonical, code, name, exchange, security_type, created_at FROM taiwan_watchlist WHERE canonical=?`, canonical).
		Scan(&entry.Canonical, &entry.Code, &entry.Name, &entry.Exchange, &entry.SecurityType, &createdAt)
	if err != nil {
		return Entry{}, err
	}
	entry.CreatedAt = parseTime(createdAt)
	return entry, nil
}

// List returns every saved entry, ordered by created_at ascending (oldest-added
// first), with canonical symbol as a stable tiebreaker for entries added within
// the same instant.
func (s *Store) List(ctx context.Context) ([]Entry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT canonical, code, name, exchange, security_type, created_at FROM taiwan_watchlist ORDER BY created_at ASC, canonical ASC`)
	if err != nil {
		return nil, fmt.Errorf("list taiwan watchlist entries: %w", err)
	}
	defer rows.Close()
	entries := make([]Entry, 0)
	for rows.Next() {
		var entry Entry
		var createdAt string
		if err := rows.Scan(&entry.Canonical, &entry.Code, &entry.Name, &entry.Exchange, &entry.SecurityType, &createdAt); err != nil {
			return nil, fmt.Errorf("scan taiwan watchlist entry: %w", err)
		}
		entry.CreatedAt = parseTime(createdAt)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list taiwan watchlist entries: %w", err)
	}
	return entries, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
