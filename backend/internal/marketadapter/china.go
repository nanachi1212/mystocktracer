package marketadapter

import "easy-stock/backend/internal/foundation"

type ChinaAdapter struct{}

func (ChinaAdapter) Market() string { return "CN" }

func (ChinaAdapter) Capabilities() []Capability { return []Capability{CapabilityNormalizeSymbol} }

func (ChinaAdapter) NormalizeSymbol(input string) (foundation.SecurityIdentity, error) {
	normalized, err := foundation.NormalizeSymbol(input)
	if err != nil {
		return foundation.SecurityIdentity{}, err
	}
	return foundation.SecurityIdentity{
		Canonical: normalized.Canonical,
		Code:      normalized.RawCode,
		Market:    "CN",
		Exchange:  normalized.Market,
		Type:      foundation.SecurityTypeStock,
		Currency:  "CNY",
		Timezone:  "Asia/Shanghai",
	}, nil
}
