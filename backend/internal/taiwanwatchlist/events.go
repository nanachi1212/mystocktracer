package taiwanwatchlist

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
)

type CorporateEventSyncResult struct {
	Canonical          string                            `json:"canonical"`
	Provider           string                            `json:"provider"`
	Status             string                            `json:"status"`
	Baseline           bool                              `json:"baseline"`
	LastSuccessfulSync *time.Time                        `json:"last_successful_sync,omitempty"`
	NewEvents          []foundation.TaiwanCorporateEvent `json:"new_events"`
}

// ApplyCorporateEvents records provider state and returns only notification-
// eligible events. The first successful observation creates a baseline; an
// unavailable provider never mutates the previous successful state.
func (s *Store) ApplyCorporateEvents(ctx context.Context, canonical string, feed foundation.TaiwanCorporateEventFeed, syncedAt time.Time) (CorporateEventSyncResult, error) {
	return s.ApplyCorporateEventsWithPreference(ctx, canonical, feed, syncedAt, true)
}

// ApplyCorporateEventsWithPreference keeps provider observation state moving
// while allowing the product-level inbox to be disabled. Events observed while
// disabled remain seen, so re-enabling cannot backfill a burst of old alerts.
func (s *Store) ApplyCorporateEventsWithPreference(ctx context.Context, canonical string, feed foundation.TaiwanCorporateEventFeed, syncedAt time.Time, alertsEnabled bool) (CorporateEventSyncResult, error) {
	canonical = strings.ToUpper(strings.TrimSpace(canonical))
	provider := strings.TrimSpace(feed.Provider)
	result := CorporateEventSyncResult{Canonical: canonical, Provider: provider, Status: feed.Status, NewEvents: []foundation.TaiwanCorporateEvent{}}
	if canonical == "" || provider == "" {
		return result, fmt.Errorf("canonical symbol and event provider are required")
	}
	if !successfulEventFeedStatus(feed.Status) {
		state, err := s.corporateEventState(ctx, canonical, provider)
		if err != nil && err != sql.ErrNoRows {
			return result, err
		}
		result.LastSuccessfulSync = state
		return result, nil
	}
	if syncedAt.IsZero() {
		syncedAt = time.Now().UTC()
	} else {
		syncedAt = syncedAt.UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin corporate event sync: %w", err)
	}
	defer tx.Rollback()
	var lastRaw string
	err = tx.QueryRowContext(ctx, `SELECT last_successful_sync FROM taiwan_corporate_event_state WHERE canonical=? AND provider=?`, canonical, provider).Scan(&lastRaw)
	baseline := err == sql.ErrNoRows
	if err != nil && err != sql.ErrNoRows {
		return result, fmt.Errorf("read corporate event state: %w", err)
	}
	lastSuccessful := parseTime(lastRaw)
	securityName := canonical
	if err := tx.QueryRowContext(ctx, `SELECT name FROM taiwan_watchlist WHERE canonical=?`, canonical).Scan(&securityName); err != nil && err != sql.ErrNoRows {
		return result, fmt.Errorf("read corporate event security name: %w", err)
	}
	for _, event := range feed.Events {
		if strings.TrimSpace(event.ID) == "" {
			continue
		}
		var exists int
		err = tx.QueryRowContext(ctx, `SELECT 1 FROM taiwan_seen_corporate_events WHERE canonical=? AND provider=? AND event_id=?`, canonical, provider, event.ID).Scan(&exists)
		if err != nil && err != sql.ErrNoRows {
			return result, fmt.Errorf("read seen corporate event: %w", err)
		}
		isNew := err == sql.ErrNoRows
		publishedAt := ""
		if event.PublishedAt != nil {
			publishedAt = formatTime(*event.PublishedAt)
		}
		if isNew {
			if _, err := tx.ExecContext(ctx, `INSERT INTO taiwan_seen_corporate_events (canonical,provider,event_id,published_at,first_seen_at) VALUES (?,?,?,?,?)`, canonical, provider, event.ID, publishedAt, formatTime(syncedAt)); err != nil {
				return result, fmt.Errorf("save seen corporate event: %w", err)
			}
			if !baseline && event.PublishedAt != nil && event.PublishedAt.After(lastSuccessful) {
				result.NewEvents = append(result.NewEvents, event)
				if alertsEnabled && !event.Stale && !event.Partial {
					status := event.Status
					if status == "" {
						status = feed.Status
					}
					if _, err := tx.ExecContext(ctx, `INSERT INTO taiwan_corporate_event_alerts
						(canonical,provider,event_id,security_name,title,category,published_at,source,source_url,retrieved_at,status,stale,partial,created_at,read_at)
						VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?, '') ON CONFLICT(canonical,provider,event_id) DO NOTHING`,
						canonical, provider, event.ID, securityName, event.Title, event.Category, publishedAt,
						event.Source, event.SourceURL, formatTime(event.RetrievedAt), status, event.Stale, event.Partial, formatTime(syncedAt)); err != nil {
						return result, fmt.Errorf("save corporate event alert: %w", err)
					}
				}
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO taiwan_corporate_event_state (canonical,provider,status,last_successful_sync) VALUES (?,?,?,?)
		ON CONFLICT(canonical,provider) DO UPDATE SET status=excluded.status,last_successful_sync=excluded.last_successful_sync`, canonical, provider, feed.Status, formatTime(syncedAt)); err != nil {
		return result, fmt.Errorf("save corporate event state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit corporate event sync: %w", err)
	}
	result.Baseline = baseline
	result.LastSuccessfulSync = &syncedAt
	return result, nil
}

func (s *Store) corporateEventState(ctx context.Context, canonical, provider string) (*time.Time, error) {
	var value string
	if err := s.db.QueryRowContext(ctx, `SELECT last_successful_sync FROM taiwan_corporate_event_state WHERE canonical=? AND provider=?`, canonical, provider).Scan(&value); err != nil {
		return nil, err
	}
	parsed := parseTime(value)
	if parsed.IsZero() {
		return nil, nil
	}
	return &parsed, nil
}

func successfulEventFeedStatus(status string) bool {
	switch status {
	case foundation.TaiwanCorporateEventsAvailable, foundation.TaiwanCorporateEventsNoEvents:
		return true
	default:
		return false
	}
}
