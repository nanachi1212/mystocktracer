package appsettings

import "time"

const (
	DefaultLLMResponseTimeoutSeconds = 300
	MinLLMResponseTimeoutSeconds     = 30
	MaxLLMResponseTimeoutSeconds     = 3600
)

type LLM struct {
	Provider               string `json:"provider"`
	BaseURL                string `json:"base_url"`
	Model                  string `json:"model"`
	APIMode                string `json:"api_mode"`
	APIKey                 string `json:"api_key,omitempty"`
	ResponseTimeoutSeconds int    `json:"response_timeout_seconds,omitempty"`
}

type LLMProfile struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Provider         string `json:"provider"`
	BaseURL          string `json:"base_url"`
	Model            string `json:"model"`
	APIMode          string `json:"api_mode"`
	APIKeyConfigured bool   `json:"api_key_configured,omitempty"`
}

type TaiwanAlerts struct {
	CorporateEventsEnabled bool `json:"corporate_events_enabled"`
}

type BrokerCommission struct {
	Rate     *float64 `json:"commission_rate,omitempty"`
	Discount float64  `json:"commission_discount,omitempty"`
	Minimum  *float64 `json:"minimum_commission,omitempty"`
	Source   string   `json:"source,omitempty"`
}

type Values struct {
	LLM                LLM              `json:"llm"`
	LLMProfiles        []LLMProfile     `json:"llm_profiles,omitempty"`
	ActiveLLMProfileID string           `json:"active_llm_profile_id,omitempty"`
	BrokerCommission   BrokerCommission `json:"broker_commission,omitempty"`
	TaiwanAlerts       TaiwanAlerts     `json:"taiwan_alerts"`
	UpdatedAt          time.Time        `json:"updated_at,omitempty"`
}

func NormalizeLLMResponseTimeoutSeconds(value int) int {
	if value < MinLLMResponseTimeoutSeconds || value > MaxLLMResponseTimeoutSeconds {
		return DefaultLLMResponseTimeoutSeconds
	}
	return value
}
