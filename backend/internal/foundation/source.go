package foundation

import "time"

// SourceMeta travels with product data so freshness and fallback provenance
// remain observable at every API and UI boundary.
type SourceMeta struct {
	Source          string     `json:"source"`
	SourceURL       string     `json:"source_url,omitempty"`
	AvailableFields []string   `json:"available_fields,omitempty"`
	FetchedAt       time.Time  `json:"fetched_at"`
	LatencyMS       int64      `json:"latency_ms"`
	Stale           bool       `json:"stale"`
	TradeDate       string     `json:"trade_date,omitempty"`
	SnapshotID      string     `json:"snapshot_id,omitempty"`
	NextRefreshAt   *time.Time `json:"next_refresh_at,omitempty"`
	FallbackReason  string     `json:"fallback_reason,omitempty"`
	CarryForward    bool       `json:"carry_forward,omitempty"`
	Status          string     `json:"status,omitempty"`
	Freshness       string     `json:"freshness,omitempty"`
	IsRealtime      bool       `json:"is_realtime"`
}
