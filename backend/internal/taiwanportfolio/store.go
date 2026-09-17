package taiwanportfolio

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const MaxNoteLength = 500

type Holding struct {
	Canonical   string    `json:"canonical"`
	DisplayName string    `json:"display_name,omitempty"`
	Shares      float64   `json:"shares"`
	AverageCost float64   `json:"average_cost"`
	Note        string    `json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Store struct{ db *sql.DB }

func OpenStore(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = ":memory:"
	}
	dsn := path
	if path == ":memory:" {
		dsn = fmt.Sprintf("file:easy-stock-taiwan-portfolio-%d?mode=memory&cache=shared", time.Now().UnixNano())
	} else if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create Taiwan portfolio data directory: %w", err)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open Taiwan portfolio database: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if _, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure Taiwan portfolio database: %w", err)
	}
	if path != ":memory:" {
		_, _ = db.Exec(`PRAGMA journal_mode=WAL;`)
	}
	if err = store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Taiwan portfolio migration: %w", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS taiwan_portfolio_holdings (
		canonical_symbol TEXT PRIMARY KEY,
		display_name TEXT NOT NULL DEFAULT '',
		shares REAL NOT NULL,
		average_cost REAL NOT NULL,
		note TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		return fmt.Errorf("create Taiwan portfolio holdings: %w", err)
	}
	columns, err := tableColumns(ctx, tx, "taiwan_portfolio_holdings")
	if err != nil {
		return err
	}
	additions := []struct{ name, definition string }{
		{"display_name", "TEXT NOT NULL DEFAULT ''"},
		{"note", "TEXT NOT NULL DEFAULT ''"},
		{"created_at", "TEXT NOT NULL DEFAULT ''"},
		{"updated_at", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, addition := range additions {
		if columns[addition.name] {
			continue
		}
		if _, err = tx.ExecContext(ctx, "ALTER TABLE taiwan_portfolio_holdings ADD COLUMN "+addition.name+" "+addition.definition); err != nil {
			return fmt.Errorf("add Taiwan portfolio %s column: %w", addition.name, err)
		}
	}
	now := formatTime(time.Now().UTC())
	if _, err = tx.ExecContext(ctx, `UPDATE taiwan_portfolio_holdings SET created_at=? WHERE created_at=''`, now); err != nil {
		return fmt.Errorf("backfill Taiwan portfolio created_at: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE taiwan_portfolio_holdings SET updated_at=created_at WHERE updated_at=''`); err != nil {
		return fmt.Errorf("backfill Taiwan portfolio updated_at: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS taiwan_portfolio_holdings_canonical ON taiwan_portfolio_holdings(canonical_symbol)`); err != nil {
		return fmt.Errorf("index Taiwan portfolio holdings: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit Taiwan portfolio migration: %w", err)
	}
	return nil
}

func tableColumns(ctx context.Context, tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return nil, fmt.Errorf("inspect Taiwan portfolio schema: %w", err)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, kind string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, fmt.Errorf("read Taiwan portfolio schema: %w", err)
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

func (s *Store) List(ctx context.Context) ([]Holding, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT canonical_symbol,display_name,shares,average_cost,note,created_at,updated_at FROM taiwan_portfolio_holdings ORDER BY created_at,canonical_symbol`)
	if err != nil {
		return nil, fmt.Errorf("list Taiwan portfolio holdings: %w", err)
	}
	defer rows.Close()
	result := []Holding{}
	for rows.Next() {
		holding, err := scanHolding(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, holding)
	}
	return result, rows.Err()
}

func (s *Store) Upsert(ctx context.Context, holding Holding) (Holding, bool, error) {
	if err := ValidateHolding(holding); err != nil {
		return Holding{}, false, err
	}
	holding.Canonical = strings.ToUpper(strings.TrimSpace(holding.Canonical))
	holding.DisplayName = strings.TrimSpace(holding.DisplayName)
	holding.Note = strings.TrimSpace(holding.Note)
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Holding{}, false, fmt.Errorf("begin Taiwan portfolio update: %w", err)
	}
	defer tx.Rollback()
	var createdRaw string
	err = tx.QueryRowContext(ctx, `SELECT created_at FROM taiwan_portfolio_holdings WHERE canonical_symbol=?`, holding.Canonical).Scan(&createdRaw)
	created := err == nil
	if err != nil && err != sql.ErrNoRows {
		return Holding{}, false, fmt.Errorf("read Taiwan portfolio holding: %w", err)
	}
	if created {
		holding.CreatedAt = parseTime(createdRaw)
	} else {
		holding.CreatedAt = now
	}
	holding.UpdatedAt = now
	if _, err = tx.ExecContext(ctx, `INSERT INTO taiwan_portfolio_holdings(canonical_symbol,display_name,shares,average_cost,note,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?) ON CONFLICT(canonical_symbol) DO UPDATE SET display_name=excluded.display_name,shares=excluded.shares,average_cost=excluded.average_cost,note=excluded.note,updated_at=excluded.updated_at`,
		holding.Canonical, holding.DisplayName, holding.Shares, holding.AverageCost, holding.Note, formatTime(holding.CreatedAt), formatTime(holding.UpdatedAt)); err != nil {
		return Holding{}, false, fmt.Errorf("save Taiwan portfolio holding: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return Holding{}, false, fmt.Errorf("commit Taiwan portfolio update: %w", err)
	}
	return holding, created, nil
}

func (s *Store) Delete(ctx context.Context, canonical string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM taiwan_portfolio_holdings WHERE canonical_symbol=?`, strings.ToUpper(strings.TrimSpace(canonical)))
	if err != nil {
		return false, fmt.Errorf("delete Taiwan portfolio holding: %w", err)
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

type scanner interface{ Scan(...any) error }

func scanHolding(row scanner) (Holding, error) {
	var holding Holding
	var createdAt, updatedAt string
	if err := row.Scan(&holding.Canonical, &holding.DisplayName, &holding.Shares, &holding.AverageCost, &holding.Note, &createdAt, &updatedAt); err != nil {
		return Holding{}, fmt.Errorf("scan Taiwan portfolio holding: %w", err)
	}
	holding.CreatedAt = parseTime(createdAt)
	holding.UpdatedAt = parseTime(updatedAt)
	return holding, nil
}

func ValidateHolding(holding Holding) error {
	if strings.TrimSpace(holding.Canonical) == "" {
		return fmt.Errorf("canonical symbol is required")
	}
	if math.IsNaN(holding.Shares) || math.IsInf(holding.Shares, 0) || holding.Shares <= 0 {
		return fmt.Errorf("shares must be a finite number greater than zero")
	}
	if math.IsNaN(holding.AverageCost) || math.IsInf(holding.AverageCost, 0) || holding.AverageCost < 0 {
		return fmt.Errorf("average cost must be a finite non-negative number")
	}
	if math.IsInf(holding.Shares*holding.AverageCost, 0) || math.IsNaN(holding.Shares*holding.AverageCost) {
		return fmt.Errorf("total cost must be finite")
	}
	if len([]rune(strings.TrimSpace(holding.Note))) > MaxNoteLength {
		return fmt.Errorf("note must be at most %d characters", MaxNoteLength)
	}
	return nil
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
