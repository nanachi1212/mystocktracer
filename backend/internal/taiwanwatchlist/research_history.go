package taiwanwatchlist

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	maxResearchEvidenceBytes = 256 * 1024
	maxResearchResultBytes   = 128 * 1024
	maxResearchMetadataBytes = 32 * 1024
)

var ErrResearchHistoryNotFound = errors.New("Taiwan AI research history not found")

type ResearchHistoryRecord struct {
	RunID            string          `json:"run_id"`
	Canonical        string          `json:"canonical"`
	SecurityName     string          `json:"security_name"`
	CreatedAt        time.Time       `json:"created_at"`
	EvidenceAsOf     string          `json:"evidence_as_of,omitempty"`
	ResearchVersion  string          `json:"research_version"`
	PayloadVersion   string          `json:"payload_version"`
	ModelProvider    string          `json:"model_provider,omitempty"`
	ModelName        string          `json:"model_name,omitempty"`
	EvidenceSnapshot json.RawMessage `json:"evidence_snapshot"`
	ResearchResult   json.RawMessage `json:"research_result"`
	Provenance       json.RawMessage `json:"provenance"`
	Validity         json.RawMessage `json:"validity"`
	Completeness     string          `json:"completeness"`
	Stale            bool            `json:"stale"`
	Partial          bool            `json:"partial"`
}

type ResearchHistorySummary struct {
	RunID           string    `json:"run_id"`
	Canonical       string    `json:"canonical"`
	SecurityName    string    `json:"security_name"`
	CreatedAt       time.Time `json:"created_at"`
	EvidenceAsOf    string    `json:"evidence_as_of,omitempty"`
	ResearchVersion string    `json:"research_version"`
	PayloadVersion  string    `json:"payload_version"`
	ModelProvider   string    `json:"model_provider,omitempty"`
	ModelName       string    `json:"model_name,omitempty"`
	Completeness    string    `json:"completeness"`
	Stale           bool      `json:"stale"`
	Partial         bool      `json:"partial"`
	HasPrevious     bool      `json:"has_previous"`
}

type ResearchHistoryPage struct {
	Runs   []ResearchHistorySummary `json:"runs"`
	Total  int                      `json:"total"`
	Limit  int                      `json:"limit"`
	Offset int                      `json:"offset"`
}

func (s *Store) SaveResearchHistory(ctx context.Context, record ResearchHistoryRecord) (ResearchHistoryRecord, bool, error) {
	record.RunID = strings.TrimSpace(record.RunID)
	record.Canonical = strings.ToUpper(strings.TrimSpace(record.Canonical))
	if record.RunID == "" || record.Canonical == "" || record.ResearchVersion == "" || record.PayloadVersion == "" {
		return ResearchHistoryRecord{}, false, errors.New("research run id, canonical symbol, research version, and payload version are required")
	}
	if record.CreatedAt.IsZero() {
		return ResearchHistoryRecord{}, false, errors.New("research created_at is required")
	}
	for name, value := range map[string]json.RawMessage{
		"evidence": record.EvidenceSnapshot, "research": record.ResearchResult, "provenance": record.Provenance, "validity": record.Validity,
	} {
		if !json.Valid(value) {
			return ResearchHistoryRecord{}, false, fmt.Errorf("%s JSON is invalid", name)
		}
	}
	if len(record.EvidenceSnapshot) > maxResearchEvidenceBytes || len(record.ResearchResult) > maxResearchResultBytes || len(record.Provenance) > maxResearchMetadataBytes || len(record.Validity) > maxResearchMetadataBytes {
		return ResearchHistoryRecord{}, false, errors.New("research history exceeds storage bounds")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO taiwan_ai_research_history
		(run_id,canonical,security_name,created_at,evidence_as_of,research_version,payload_version,model_provider,model_name,evidence_json,research_json,provenance_json,validity_json,completeness,stale,partial)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(run_id) DO NOTHING`,
		record.RunID, record.Canonical, record.SecurityName, formatTime(record.CreatedAt), record.EvidenceAsOf,
		record.ResearchVersion, record.PayloadVersion, record.ModelProvider, record.ModelName,
		[]byte(record.EvidenceSnapshot), []byte(record.ResearchResult), []byte(record.Provenance), []byte(record.Validity),
		record.Completeness, record.Stale, record.Partial)
	if err != nil {
		return ResearchHistoryRecord{}, false, fmt.Errorf("save Taiwan AI research history: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return ResearchHistoryRecord{}, false, fmt.Errorf("read Taiwan AI research insert result: %w", err)
	}
	saved, err := s.GetResearchHistory(ctx, record.Canonical, record.RunID)
	if err != nil {
		if rows == 0 {
			return ResearchHistoryRecord{}, false, errors.New("research run id already belongs to another symbol")
		}
		return ResearchHistoryRecord{}, false, err
	}
	return saved, rows > 0, nil
}

func (s *Store) ListResearchHistory(ctx context.Context, canonical string, limit, offset int) (ResearchHistoryPage, error) {
	canonical = strings.ToUpper(strings.TrimSpace(canonical))
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ResearchHistoryPage{}, fmt.Errorf("begin Taiwan AI research history list: %w", err)
	}
	defer tx.Rollback()
	page := ResearchHistoryPage{Runs: []ResearchHistorySummary{}, Limit: limit, Offset: offset}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM taiwan_ai_research_history WHERE canonical=?`, canonical).Scan(&page.Total); err != nil {
		return ResearchHistoryPage{}, fmt.Errorf("count Taiwan AI research history: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT h.run_id,h.canonical,h.security_name,h.created_at,h.evidence_as_of,h.research_version,h.payload_version,h.model_provider,h.model_name,h.completeness,h.stale,h.partial,
		EXISTS(SELECT 1 FROM taiwan_ai_research_history p WHERE p.canonical=h.canonical AND p.id<h.id)
		FROM taiwan_ai_research_history h WHERE h.canonical=? ORDER BY h.id DESC LIMIT ? OFFSET ?`, canonical, limit, offset)
	if err != nil {
		return ResearchHistoryPage{}, fmt.Errorf("list Taiwan AI research history: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item ResearchHistorySummary
		var createdAt string
		if err := rows.Scan(&item.RunID, &item.Canonical, &item.SecurityName, &createdAt, &item.EvidenceAsOf, &item.ResearchVersion, &item.PayloadVersion, &item.ModelProvider, &item.ModelName, &item.Completeness, &item.Stale, &item.Partial, &item.HasPrevious); err != nil {
			return ResearchHistoryPage{}, fmt.Errorf("scan Taiwan AI research history: %w", err)
		}
		item.CreatedAt = parseTime(createdAt)
		page.Runs = append(page.Runs, item)
	}
	if err := rows.Err(); err != nil {
		return ResearchHistoryPage{}, fmt.Errorf("iterate Taiwan AI research history: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ResearchHistoryPage{}, fmt.Errorf("commit Taiwan AI research history list: %w", err)
	}
	return page, nil
}

// ListRecentResearchHistory returns the newest successful research runs across all
// symbols. Dashboard consumers use this bounded query instead of issuing one query
// per Watchlist/Portfolio symbol.
func (s *Store) ListRecentResearchHistory(ctx context.Context, limit int) ([]ResearchHistorySummary, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("research history limit is outside the allowed range")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT h.run_id,h.canonical,h.security_name,h.created_at,h.evidence_as_of,h.research_version,h.payload_version,h.model_provider,h.model_name,h.completeness,h.stale,h.partial,
		EXISTS(SELECT 1 FROM taiwan_ai_research_history p WHERE p.canonical=h.canonical AND p.id<h.id)
		FROM taiwan_ai_research_history h ORDER BY h.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent Taiwan AI research history: %w", err)
	}
	defer rows.Close()
	result := make([]ResearchHistorySummary, 0, limit)
	for rows.Next() {
		var item ResearchHistorySummary
		var createdAt string
		if err := rows.Scan(&item.RunID, &item.Canonical, &item.SecurityName, &createdAt, &item.EvidenceAsOf, &item.ResearchVersion, &item.PayloadVersion, &item.ModelProvider, &item.ModelName, &item.Completeness, &item.Stale, &item.Partial, &item.HasPrevious); err != nil {
			return nil, fmt.Errorf("scan recent Taiwan AI research history: %w", err)
		}
		item.CreatedAt = parseTime(createdAt)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent Taiwan AI research history: %w", err)
	}
	return result, nil
}

func (s *Store) GetResearchHistory(ctx context.Context, canonical, runID string) (ResearchHistoryRecord, error) {
	canonical = strings.ToUpper(strings.TrimSpace(canonical))
	runID = strings.TrimSpace(runID)
	row := s.db.QueryRowContext(ctx, `SELECT run_id,canonical,security_name,created_at,evidence_as_of,research_version,payload_version,model_provider,model_name,evidence_json,research_json,provenance_json,validity_json,completeness,stale,partial
		FROM taiwan_ai_research_history WHERE canonical=? AND run_id=?`, canonical, runID)
	record, err := scanResearchHistory(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ResearchHistoryRecord{}, ErrResearchHistoryNotFound
	}
	if err != nil {
		return ResearchHistoryRecord{}, fmt.Errorf("get Taiwan AI research history: %w", err)
	}
	return record, nil
}

func (s *Store) PreviousResearchHistory(ctx context.Context, canonical, runID string) (ResearchHistoryRecord, error) {
	canonical = strings.ToUpper(strings.TrimSpace(canonical))
	runID = strings.TrimSpace(runID)
	row := s.db.QueryRowContext(ctx, `SELECT p.run_id,p.canonical,p.security_name,p.created_at,p.evidence_as_of,p.research_version,p.payload_version,p.model_provider,p.model_name,p.evidence_json,p.research_json,p.provenance_json,p.validity_json,p.completeness,p.stale,p.partial
		FROM taiwan_ai_research_history p
		WHERE p.canonical=? AND p.id < (SELECT id FROM taiwan_ai_research_history WHERE canonical=? AND run_id=?)
		ORDER BY p.id DESC LIMIT 1`, canonical, canonical, runID)
	record, err := scanResearchHistory(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ResearchHistoryRecord{}, ErrResearchHistoryNotFound
	}
	if err != nil {
		return ResearchHistoryRecord{}, fmt.Errorf("get previous Taiwan AI research history: %w", err)
	}
	return record, nil
}

type researchHistoryScanner interface {
	Scan(dest ...any) error
}

func scanResearchHistory(row researchHistoryScanner) (ResearchHistoryRecord, error) {
	var record ResearchHistoryRecord
	var createdAt string
	var evidence, research, provenance, validity []byte
	if err := row.Scan(&record.RunID, &record.Canonical, &record.SecurityName, &createdAt, &record.EvidenceAsOf, &record.ResearchVersion, &record.PayloadVersion, &record.ModelProvider, &record.ModelName, &evidence, &research, &provenance, &validity, &record.Completeness, &record.Stale, &record.Partial); err != nil {
		return ResearchHistoryRecord{}, err
	}
	record.CreatedAt = parseTime(createdAt)
	record.EvidenceSnapshot = append(json.RawMessage(nil), evidence...)
	record.ResearchResult = append(json.RawMessage(nil), research...)
	record.Provenance = append(json.RawMessage(nil), provenance...)
	record.Validity = append(json.RawMessage(nil), validity...)
	return record, nil
}
