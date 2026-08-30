package foundation

import "testing"

func TestTaiwanRuleProfileUsesOfficialMetadataAndFailsClosed(t *testing.T) {
	stock := TaiwanRuleProfile(SecurityIdentity{Type: SecurityTypeStock, SourceURL: "official"})
	if stock.Status != "official" || stock.TickSizeClass != "ordinary_stock" || stock.BoardLotShares == nil || *stock.BoardLotShares != 1000 {
		t.Fatalf("stock = %+v", stock)
	}
	domestic := TaiwanRuleProfile(SecurityIdentity{Type: SecurityTypeETF, SourceURL: "official", TaiwanMetadata: &TaiwanSecurityMetadata{ComponentScope: "domestic", Strategy: "ordinary"}})
	if domestic.Status != "official" || domestic.TickSizeClass != "etf" || domestic.PriceLimitPct == nil || *domestic.PriceLimitPct != .10 {
		t.Fatalf("domestic = %+v", domestic)
	}
	foreign := TaiwanRuleProfile(SecurityIdentity{Type: SecurityTypeETF, SourceURL: "official", TaiwanMetadata: &TaiwanSecurityMetadata{ComponentScope: "foreign", Strategy: "ordinary"}})
	if foreign.Status != "official" || foreign.PriceLimitClass != "no_limit" || foreign.TaxClass != "foreign_component_etf" {
		t.Fatalf("foreign = %+v", foreign)
	}
	unknownMultiplier := TaiwanRuleProfile(SecurityIdentity{Type: SecurityTypeETF, SourceURL: "official", TaiwanMetadata: &TaiwanSecurityMetadata{ComponentScope: "domestic", Strategy: "leveraged_or_inverse"}})
	if unknownMultiplier.Status != "data_insufficient" || unknownMultiplier.PriceLimitPct != nil {
		t.Fatalf("leveraged = %+v", unknownMultiplier)
	}
}

func TestBrokerCommissionRequiresExplicitRate(t *testing.T) {
	if fee, err := (BrokerCommissionConfig{}).Commission(100000); err != nil || fee != nil {
		t.Fatalf("unset = %v, %v", fee, err)
	}
	rate, discount, minimum := .001425, .6, 20.0
	fee, err := (BrokerCommissionConfig{Rate: &rate, Discount: discount, Minimum: &minimum, Source: "broker_config"}).Commission(100000)
	if err != nil || fee == nil || *fee != 85.5 {
		t.Fatalf("configured = %v, %v", fee, err)
	}
}
