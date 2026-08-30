package foundation

import "time"

type SecurityType string

const (
	SecurityTypeStock SecurityType = "stock"
	SecurityTypeETF   SecurityType = "etf"
	SecurityTypeIndex SecurityType = "index"
)

type SecurityIdentity struct {
	Canonical   string       `json:"canonical"`
	Code        string       `json:"code"`
	Name        string       `json:"name"`
	FullName    string       `json:"full_name,omitempty"`
	Market      string       `json:"market"`
	Exchange    string       `json:"exchange"`
	Type        SecurityType `json:"security_type"`
	Currency    string       `json:"currency"`
	Timezone    string       `json:"timezone"`
	ListedAt    string       `json:"listed_at,omitempty"`
	Industry    string       `json:"industry,omitempty"`
	Provider    string       `json:"provider"`
	SourceURL   string       `json:"source_url"`
	RetrievedAt time.Time    `json:"retrieved_at"`
}
