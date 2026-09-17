package taiwanwatchlist

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	AlertFilterAll    = "all"
	AlertFilterUnread = "unread"
	AlertFilterRead   = "read"
)

type Alert struct {
	ID           int64      `json:"id"`
	Canonical    string     `json:"canonical"`
	SecurityName string     `json:"security_name"`
	Provider     string     `json:"provider"`
	EventID      string     `json:"event_id"`
	Title        string     `json:"title"`
	Category     string     `json:"category,omitempty"`
	PublishedAt  *time.Time `json:"published_at,omitempty"`
	Source       string     `json:"source"`
	SourceURL    string     `json:"source_url,omitempty"`
	RetrievedAt  *time.Time `json:"retrieved_at,omitempty"`
	Status       string     `json:"status"`
	Stale        bool       `json:"stale"`
	Partial      bool       `json:"partial"`
	CreatedAt    time.Time  `json:"created_at"`
	Read         bool       `json:"read"`
	ReadAt       *time.Time `json:"read_at,omitempty"`
}

type AlertPage struct {
	Alerts      []Alert `json:"alerts"`
	UnreadCount int     `json:"unread_count"`
	Total       int     `json:"total"`
	Limit       int     `json:"limit"`
	Offset      int     `json:"offset"`
}

func (s *Store) ListAlerts(ctx context.Context, filter string, limit, offset int) (AlertPage, error) {
	where := ""
	switch filter {
	case AlertFilterUnread:
		where = " WHERE read_at=''"
	case AlertFilterRead:
		where = " WHERE read_at<>''"
	case AlertFilterAll:
	default:
		return AlertPage{}, fmt.Errorf("unsupported alert filter")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return AlertPage{}, fmt.Errorf("begin Taiwan alert list: %w", err)
	}
	defer tx.Rollback()
	page := AlertPage{Alerts: []Alert{}, Limit: limit, Offset: offset}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM taiwan_corporate_event_alerts WHERE read_at=''`).Scan(&page.UnreadCount); err != nil {
		return AlertPage{}, fmt.Errorf("count unread Taiwan alerts: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM taiwan_corporate_event_alerts`+where).Scan(&page.Total); err != nil {
		return AlertPage{}, fmt.Errorf("count Taiwan alerts: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,canonical,security_name,provider,event_id,title,category,published_at,source,source_url,retrieved_at,status,stale,partial,created_at,read_at
		FROM taiwan_corporate_event_alerts`+where+` ORDER BY CASE WHEN published_at='' THEN created_at ELSE published_at END DESC, created_at DESC, id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return AlertPage{}, fmt.Errorf("list Taiwan alerts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var alert Alert
		var publishedAt, retrievedAt, createdAt, readAt string
		if err := rows.Scan(&alert.ID, &alert.Canonical, &alert.SecurityName, &alert.Provider, &alert.EventID, &alert.Title, &alert.Category, &publishedAt, &alert.Source, &alert.SourceURL, &retrievedAt, &alert.Status, &alert.Stale, &alert.Partial, &createdAt, &readAt); err != nil {
			return AlertPage{}, fmt.Errorf("scan Taiwan alert: %w", err)
		}
		if value := parseTime(publishedAt); !value.IsZero() {
			alert.PublishedAt = &value
		}
		if value := parseTime(retrievedAt); !value.IsZero() {
			alert.RetrievedAt = &value
		}
		alert.CreatedAt = parseTime(createdAt)
		if value := parseTime(readAt); !value.IsZero() {
			alert.Read = true
			alert.ReadAt = &value
		}
		page.Alerts = append(page.Alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return AlertPage{}, fmt.Errorf("iterate Taiwan alerts: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return AlertPage{}, fmt.Errorf("commit Taiwan alert list: %w", err)
	}
	return page, nil
}

func (s *Store) MarkAlertRead(ctx context.Context, id int64, readAt time.Time) (bool, error) {
	if readAt.IsZero() {
		readAt = time.Now().UTC()
	}
	result, err := s.db.ExecContext(ctx, `UPDATE taiwan_corporate_event_alerts SET read_at=? WHERE id=?`, formatTime(readAt), id)
	if err != nil {
		return false, fmt.Errorf("mark Taiwan alert read: %w", err)
	}
	changed, err := result.RowsAffected()
	return changed > 0, err
}

func (s *Store) MarkAllAlertsRead(ctx context.Context, readAt time.Time) (int64, error) {
	if readAt.IsZero() {
		readAt = time.Now().UTC()
	}
	result, err := s.db.ExecContext(ctx, `UPDATE taiwan_corporate_event_alerts SET read_at=? WHERE read_at=''`, formatTime(readAt))
	if err != nil {
		return 0, fmt.Errorf("mark all Taiwan alerts read: %w", err)
	}
	return result.RowsAffected()
}
