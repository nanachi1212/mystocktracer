package foundation

import "time"

const (
	TaiwanCorporateEventProviderToAlpha = "toalpha"
	TaiwanCorporateEventSourceMOPS      = "mops_material_information"

	TaiwanCorporateEventsAvailable   = "available"
	TaiwanCorporateEventsNoEvents    = "no_events"
	TaiwanCorporateEventsNotQueried  = "not_queried"
	TaiwanCorporateEventsUnsupported = "unsupported"
	TaiwanCorporateEventsPartial     = "partial"
	TaiwanCorporateEventsStale       = "stale"
	TaiwanCorporateEventsUnavailable = "unavailable"
)

// TaiwanCorporateEvent is the product-owned, provider-neutral representation of
// a Taiwan company announcement. Provider-specific JSON must be normalized into
// this type before it can reach product or AI layers.
type TaiwanCorporateEvent struct {
	ID                   string     `json:"event_id"`
	Symbol               string     `json:"symbol"`
	Title                string     `json:"title"`
	Category             string     `json:"category,omitempty"`
	Detail               string     `json:"detail,omitempty"`
	PublishedAt          *time.Time `json:"published_at,omitempty"`
	EventDate            *time.Time `json:"event_time,omitempty"`
	Provider             string     `json:"provider"`
	Source               string     `json:"source"`
	SourceURL            string     `json:"source_url,omitempty"`
	RetrievedAt          time.Time  `json:"retrieved_at"`
	Status               string     `json:"status"`
	Stale                bool       `json:"stale"`
	Partial              bool       `json:"partial"`
	Reason               string     `json:"reason,omitempty"`
	ClassificationSource string     `json:"classification_source,omitempty"`
	Important            *bool      `json:"important,omitempty"`
}

// TaiwanCorporateEventFeed keeps absence and failure states distinct. In
// particular, unavailable/not_queried never means that a company has no events.
type TaiwanCorporateEventFeed struct {
	Status      string                 `json:"status"`
	Provider    string                 `json:"provider"`
	Source      string                 `json:"source"`
	SourceURL   string                 `json:"source_url,omitempty"`
	RetrievedAt *time.Time             `json:"retrieved_at,omitempty"`
	AsOf        string                 `json:"as_of,omitempty"`
	Stale       bool                   `json:"stale"`
	Partial     bool                   `json:"partial"`
	Reason      string                 `json:"reason,omitempty"`
	Events      []TaiwanCorporateEvent `json:"events"`
}
