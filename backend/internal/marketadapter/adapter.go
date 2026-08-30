package marketadapter

import (
	"fmt"
	"sort"
	"strings"

	"easy-stock/backend/internal/foundation"
)

type Capability string

const (
	CapabilityNormalizeSymbol   Capability = "normalize_symbol"
	CapabilitySecurityDirectory Capability = "security_directory"
	CapabilitySearchSymbol      Capability = "search_symbol"
)

type Adapter interface {
	Market() string
	Capabilities() []Capability
	NormalizeSymbol(input string) (foundation.SecurityIdentity, error)
}

type Registry struct {
	adapters map[string]Adapter
}

func NewRegistry(adapters ...Adapter) (*Registry, error) {
	registry := &Registry{adapters: make(map[string]Adapter, len(adapters))}
	for _, adapter := range adapters {
		if adapter == nil {
			return nil, fmt.Errorf("market adapter is nil")
		}
		market := strings.ToUpper(strings.TrimSpace(adapter.Market()))
		if market == "" {
			return nil, fmt.Errorf("market adapter has no market")
		}
		if _, exists := registry.adapters[market]; exists {
			return nil, fmt.Errorf("market adapter %q already registered", market)
		}
		registry.adapters[market] = adapter
	}
	return registry, nil
}

func (r *Registry) Adapter(market string) (Adapter, bool) {
	adapter, ok := r.adapters[strings.ToUpper(strings.TrimSpace(market))]
	return adapter, ok
}

func (r *Registry) Markets() []string {
	markets := make([]string, 0, len(r.adapters))
	for market := range r.adapters {
		markets = append(markets, market)
	}
	sort.Strings(markets)
	return markets
}
