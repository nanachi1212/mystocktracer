package marketadapter

import (
	"fmt"
	"regexp"
	"strings"

	"easy-stock/backend/internal/foundation"
)

type TaiwanAdapter struct{}

var taiwanCode = regexp.MustCompile(`^[0-9A-Z]{4,6}$`)

func (TaiwanAdapter) Market() string { return "TW" }

func (TaiwanAdapter) Capabilities() []Capability {
	return []Capability{CapabilityNormalizeSymbol, CapabilitySecurityDirectory, CapabilitySearchSymbol}
}

func (TaiwanAdapter) NormalizeSymbol(input string) (foundation.SecurityIdentity, error) {
	raw := strings.ToUpper(strings.TrimSpace(input))
	parts := strings.Split(raw, ".")
	if len(parts) != 2 || !taiwanCode.MatchString(parts[0]) {
		return foundation.SecurityIdentity{}, fmt.Errorf("Taiwan symbol must use CODE.TWSE or CODE.TPEX")
	}
	exchange := parts[1]
	if exchange != "TWSE" && exchange != "TPEX" {
		return foundation.SecurityIdentity{}, fmt.Errorf("unsupported Taiwan exchange %q", exchange)
	}
	return foundation.SecurityIdentity{
		Canonical: parts[0] + "." + exchange,
		Code:      parts[0], Market: "TW", Exchange: exchange,
		Currency: "TWD", Timezone: "Asia/Taipei",
	}, nil
}
