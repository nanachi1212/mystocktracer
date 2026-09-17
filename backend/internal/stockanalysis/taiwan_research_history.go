package stockanalysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
)

var taiwanResearchDomains = []string{"price", "market", "industry", "institutional", "margin", "fundamentals", "corporate_events"}

type TaiwanResearchHistoryValidity struct {
	Valid              bool     `json:"valid"`
	Completeness       string   `json:"completeness"`
	Stale              bool     `json:"stale"`
	Partial            bool     `json:"partial"`
	UnavailableDomains []string `json:"unavailable_domains"`
}

type TaiwanResearchHistoryMetadata struct {
	EvidenceAsOf string                        `json:"evidence_as_of,omitempty"`
	Provenance   map[string]map[string]any     `json:"provenance"`
	Validity     TaiwanResearchHistoryValidity `json:"validity"`
}

func BuildTaiwanResearchHistoryMetadata(input TaiwanStockIntelligence, payload TaiwanResearchPayload) TaiwanResearchHistoryMetadata {
	domains := researchPayloadDomains(payload)
	metadata := TaiwanResearchHistoryMetadata{Provenance: map[string]map[string]any{}, Validity: TaiwanResearchHistoryValidity{Valid: true, Completeness: "complete", UnavailableDomains: []string{}}}
	for _, domain := range taiwanResearchDomains {
		value := domains[domain]
		state := evidenceDomainAvailability(value)
		if state == "stale" {
			metadata.Validity.Stale = true
		}
		if state == "partial" {
			metadata.Validity.Partial = true
		}
		if state == "unavailable" || state == "absent" {
			metadata.Validity.UnavailableDomains = append(metadata.Validity.UnavailableDomains, domain)
		}
		provenance := map[string]any{"status": state}
		for _, key := range []string{"as_of", "period", "latest_completed_date", "status", "freshness", "provider", "source", "source_url", "stale", "partial"} {
			if item, ok := value[key]; ok {
				if key == "source_url" {
					if safe := safeResearchSourceURL(fmt.Sprint(item)); safe != "" {
						provenance[key] = safe
					}
					continue
				}
				provenance[key] = item
			}
		}
		metadata.Provenance[domain] = provenance
		for _, key := range []string{"as_of", "period", "latest_completed_date"} {
			if candidate, ok := value[key].(string); ok && candidate > metadata.EvidenceAsOf {
				metadata.EvidenceAsOf = candidate
			}
		}
	}
	mergeEvidenceStatus := func(domain string, status TaiwanEvidenceStatus) {
		provenance := metadata.Provenance[domain]
		for key, value := range map[string]string{"status": status.Status, "freshness": status.Freshness, "as_of": status.AsOf, "source": status.Source} {
			if value != "" {
				provenance[key] = value
			}
		}
		if safe := safeResearchSourceURL(status.SourceURL); safe != "" {
			provenance["source_url"] = safe
		}
	}
	mergeEvidenceStatus("price", input.PriceHistory.TaiwanEvidenceStatus)
	mergeEvidenceStatus("market", input.MarketContext.TaiwanEvidenceStatus)
	mergeEvidenceStatus("industry", input.IndustryContext.TaiwanEvidenceStatus)
	mergeEvidenceStatus("institutional", input.Institutional.TaiwanEvidenceStatus)
	mergeEvidenceStatus("margin", input.Margin.TaiwanEvidenceStatus)
	mergeEvidenceStatus("fundamentals", input.Fundamentals.TaiwanEvidenceStatus)
	corporate := metadata.Provenance["corporate_events"]
	corporate["status"], corporate["provider"], corporate["source"], corporate["as_of"], corporate["stale"], corporate["partial"] = input.CorporateEvents.Status, input.CorporateEvents.Provider, input.CorporateEvents.Source, input.CorporateEvents.AsOf, input.CorporateEvents.Stale, input.CorporateEvents.Partial
	if safe := safeResearchSourceURL(input.CorporateEvents.SourceURL); safe != "" {
		corporate["source_url"] = safe
	}
	if len(payload.DataQuality.StaleComponents) > 0 {
		metadata.Validity.Stale = true
	}
	if len(payload.DataQuality.PartialComponents) > 0 {
		metadata.Validity.Partial = true
	}
	if metadata.Validity.Stale || metadata.Validity.Partial || len(metadata.Validity.UnavailableDomains) > 0 {
		metadata.Validity.Completeness = "limited"
	}
	if metadata.Validity.Partial {
		metadata.Validity.Completeness = "partial"
	}
	return metadata
}

func safeResearchSourceURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	return parsed.String()
}

type TaiwanResearchFieldChange struct {
	Path         string   `json:"path"`
	State        string   `json:"state"`
	Before       any      `json:"before,omitempty"`
	After        any      `json:"after,omitempty"`
	Delta        *float64 `json:"delta,omitempty"`
	DeltaPercent *float64 `json:"delta_percent,omitempty"`
}

type TaiwanResearchDomainComparison struct {
	Domain        string                      `json:"domain"`
	State         string                      `json:"state"`
	ChangedFields []TaiwanResearchFieldChange `json:"changed_fields"`
}

type TaiwanResearchAssumptionComparison struct {
	Kind         string   `json:"kind"`
	Section      string   `json:"section,omitempty"`
	Text         string   `json:"text"`
	EvidenceKeys []string `json:"evidence_keys"`
	Status       string   `json:"status"`
}

type TaiwanResearchComparisonSummary struct {
	Unchanged              int `json:"unchanged"`
	Changed                int `json:"changed"`
	NewlyAvailable         int `json:"newly_available"`
	NoLongerAvailable      int `json:"no_longer_available"`
	Stale                  int `json:"stale"`
	Partial                int `json:"partial"`
	Unavailable            int `json:"unavailable"`
	CorporateEventsAdded   int `json:"corporate_events_added"`
	CorporateEventsRemoved int `json:"corporate_events_removed"`
}

type TaiwanResearchComparison struct {
	Comparable             bool                                 `json:"comparable"`
	Reason                 string                               `json:"reason,omitempty"`
	CurrentRunID           string                               `json:"current_run_id"`
	PreviousRunID          string                               `json:"previous_run_id,omitempty"`
	Summary                TaiwanResearchComparisonSummary      `json:"summary"`
	Domains                []TaiwanResearchDomainComparison     `json:"domains"`
	Assumptions            []TaiwanResearchAssumptionComparison `json:"assumptions"`
	CorporateEventsAdded   []TaiwanResearchCorporateEventChange `json:"corporate_events_added"`
	CorporateEventsRemoved []TaiwanResearchCorporateEventChange `json:"corporate_events_removed"`
}

type TaiwanResearchCorporateEventChange struct {
	EventID     string `json:"event_id"`
	Title       string `json:"title,omitempty"`
	Category    string `json:"category,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

func CompareTaiwanResearchHistory(currentRunID, previousRunID, currentResearchVersion, previousResearchVersion, currentPayloadVersion, previousPayloadVersion string, currentEvidence, previousEvidence, currentResearch, previousResearch json.RawMessage) TaiwanResearchComparison {
	comparison := TaiwanResearchComparison{CurrentRunID: currentRunID, PreviousRunID: previousRunID, Domains: []TaiwanResearchDomainComparison{}, Assumptions: []TaiwanResearchAssumptionComparison{}, CorporateEventsAdded: []TaiwanResearchCorporateEventChange{}, CorporateEventsRemoved: []TaiwanResearchCorporateEventChange{}}
	if !supportedTaiwanResearchHistoryVersion(currentPayloadVersion) || !supportedTaiwanResearchHistoryVersion(previousPayloadVersion) || !supportedTaiwanResearchHistoryVersion(currentResearchVersion) || !supportedTaiwanResearchHistoryVersion(previousResearchVersion) {
		comparison.Reason = "research history version is not comparable"
		return comparison
	}
	current, err := decodeResearchObject(currentEvidence)
	if err != nil {
		comparison.Reason = "current evidence snapshot is invalid"
		return comparison
	}
	previous, err := decodeResearchObject(previousEvidence)
	if err != nil {
		comparison.Reason = "previous evidence snapshot is invalid"
		return comparison
	}
	if fmt.Sprint(current["research_version"]) != currentPayloadVersion || fmt.Sprint(previous["research_version"]) != previousPayloadVersion {
		comparison.Reason = "research payload version metadata mismatch"
		return comparison
	}
	var currentResult, previousResult genericResearchResult
	if json.Unmarshal(currentResearch, &currentResult) != nil || json.Unmarshal(previousResearch, &previousResult) != nil || currentResult.ModelVersion != currentResearchVersion || previousResult.ModelVersion != previousResearchVersion {
		comparison.Reason = "research result version metadata mismatch"
		return comparison
	}
	comparison.Comparable = true
	for _, domain := range taiwanResearchDomains {
		before, _ := previous[domain].(map[string]any)
		after, _ := current[domain].(map[string]any)
		state := compareEvidenceDomain(before, after)
		item := TaiwanResearchDomainComparison{Domain: domain, State: state, ChangedFields: compareDomainFields(domain, before, after)}
		comparison.Domains = append(comparison.Domains, item)
		incrementComparisonSummary(&comparison.Summary, state)
	}
	comparison.CorporateEventsAdded, comparison.CorporateEventsRemoved = compareCorporateEventIDs(previous, current)
	comparison.Summary.CorporateEventsAdded = len(comparison.CorporateEventsAdded)
	comparison.Summary.CorporateEventsRemoved = len(comparison.CorporateEventsRemoved)
	comparison.Assumptions = compareResearchAssumptions(previousResearch, currentResearch, comparison.Domains)
	return comparison
}

func supportedTaiwanResearchHistoryVersion(version string) bool {
	return version == TaiwanAIResearchVersionV1 || version == TaiwanAIResearchVersionV2
}

func decodeResearchObject(data json.RawMessage) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func researchPayloadDomains(payload TaiwanResearchPayload) map[string]map[string]any {
	return map[string]map[string]any{"price": payload.Price, "market": payload.Market, "industry": payload.Industry, "institutional": payload.Institutional, "margin": payload.Margin, "fundamentals": payload.Fundamentals, "corporate_events": payload.CorporateEvents}
}

func evidenceDomainAvailability(value map[string]any) string {
	if len(value) == 0 {
		return "absent"
	}
	if truthy(value["stale"]) || strings.EqualFold(fmt.Sprint(value["freshness"]), "stale") {
		return "stale"
	}
	if truthy(value["partial"]) || strings.EqualFold(fmt.Sprint(value["status"]), "partial") {
		return "partial"
	}
	for key, item := range value {
		if strings.HasSuffix(key, "_status") && strings.EqualFold(fmt.Sprint(item), "partial") {
			return "partial"
		}
	}
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(value["status"])))
	switch status {
	case "unavailable", "unsupported", "data_insufficient", "not_queried":
		return "unavailable"
	}
	return "available"
}

func compareEvidenceDomain(before, after map[string]any) string {
	beforeState, afterState := evidenceDomainAvailability(before), evidenceDomainAvailability(after)
	if afterState == "unavailable" && beforeState != "unavailable" && beforeState != "absent" {
		return "no_longer_available"
	}
	switch afterState {
	case "stale", "partial", "unavailable":
		return afterState
	case "absent":
		if beforeState != "absent" {
			return "no_longer_available"
		}
		return "unchanged"
	}
	if beforeState == "absent" || beforeState == "unavailable" {
		return "newly_available"
	}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	if bytes.Equal(beforeJSON, afterJSON) {
		return "unchanged"
	}
	return "changed"
}

var comparableResearchFields = map[string][]string{
	"price":            {"return_5d_percent", "return_20d_percent", "state", "status", "quote_status", "quote_freshness", "latest_completed_date"},
	"market":           {"state", "confidence", "status", "freshness", "as_of", "advance_ratio", "advancing_amount_ratio"},
	"industry":         {"state", "status", "industry_id", "industry_name", "benchmark_scope", "relative_breadth", "relative_capital"},
	"institutional":    {"state", "status", "as_of", "foreign_official_net", "investment_trust_official_net", "dealer_official_net"},
	"margin":           {"state", "status", "as_of", "margin_change", "short_change"},
	"fundamentals":     {"state", "status", "period", "official_monthly_revenue_yoy_percent", "valuation_status", "valuation.data_date", "valuation.pe", "valuation.pb", "valuation.dividend_yield_percent", "financial_statement_status", "financial_statement.fiscal_year", "financial_statement.fiscal_quarter", "financial_statement.cumulative_eps", "financial_statement.gross_margin_percent", "financial_statement.operating_margin_percent", "balance_status", "balance.balance_fiscal_year", "balance.balance_fiscal_quarter", "balance.book_value_per_share", "balance.debt_ratio_percent", "balance.debt_to_equity_percent", "balance.current_ratio_percent", "cashflow_status", "cashflow.cashflow_fiscal_year", "cashflow.cashflow_fiscal_quarter", "cashflow.operating_cash_flow", "cashflow.cash_flow_to_net_income"},
	"corporate_events": {"status", "as_of", "stale", "partial"},
}

func compareDomainFields(domain string, before, after map[string]any) []TaiwanResearchFieldChange {
	changes := []TaiwanResearchFieldChange{}
	for _, path := range comparableResearchFields[domain] {
		oldValue, oldOK := nestedValue(before, path)
		newValue, newOK := nestedValue(after, path)
		if !oldOK && !newOK {
			continue
		}
		oldJSON, _ := json.Marshal(oldValue)
		newJSON, _ := json.Marshal(newValue)
		if oldOK && newOK && bytes.Equal(oldJSON, newJSON) {
			continue
		}
		change := TaiwanResearchFieldChange{Path: domain + "." + path, Before: oldValue, After: newValue, State: "changed"}
		if !oldOK {
			change.State = "newly_available"
		}
		if !newOK {
			change.State = "no_longer_available"
		}
		periodField := strings.Contains(path, "fiscal_year") || strings.Contains(path, "fiscal_quarter")
		if oldNumber, ok := numberValue(oldValue); ok && !periodField {
			if newNumber, ok := numberValue(newValue); ok {
				delta := newNumber - oldNumber
				change.Delta = &delta
				if oldNumber != 0 {
					percent := delta / math.Abs(oldNumber) * 100
					change.DeltaPercent = &percent
				}
			}
		}
		changes = append(changes, change)
	}
	return changes
}

func nestedValue(value map[string]any, path string) (any, bool) {
	var current any = value
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok || current == nil {
			return nil, false
		}
	}
	return current, true
}

func numberValue(value any) (float64, bool) {
	switch number := value.(type) {
	case json.Number:
		result, err := number.Float64()
		return result, err == nil
	case float64:
		return number, true
	default:
		return 0, false
	}
}

func truthy(value any) bool {
	result, _ := value.(bool)
	return result
}

func incrementComparisonSummary(summary *TaiwanResearchComparisonSummary, state string) {
	switch state {
	case "unchanged":
		summary.Unchanged++
	case "changed":
		summary.Changed++
	case "newly_available":
		summary.NewlyAvailable++
	case "no_longer_available":
		summary.NoLongerAvailable++
	case "stale":
		summary.Stale++
	case "partial":
		summary.Partial++
	case "unavailable":
		summary.Unavailable++
	}
}

func compareCorporateEventIDs(previous, current map[string]any) ([]TaiwanResearchCorporateEventChange, []TaiwanResearchCorporateEventChange) {
	textValue := func(value any) string {
		if value == nil {
			return ""
		}
		return fmt.Sprint(value)
	}
	eventsByID := func(payload map[string]any) map[string]TaiwanResearchCorporateEventChange {
		result := map[string]TaiwanResearchCorporateEventChange{}
		domain, _ := payload["corporate_events"].(map[string]any)
		events, _ := domain["events"].([]any)
		for _, raw := range events {
			item, _ := raw.(map[string]any)
			if id := strings.TrimSpace(fmt.Sprint(item["event_id"])); id != "" {
				result[id] = TaiwanResearchCorporateEventChange{EventID: id, Title: textValue(item["title"]), Category: textValue(item["category"]), PublishedAt: textValue(item["published_at"])}
			}
		}
		return result
	}
	before, after := eventsByID(previous), eventsByID(current)
	added, removed := []TaiwanResearchCorporateEventChange{}, []TaiwanResearchCorporateEventChange{}
	for id, event := range after {
		if _, exists := before[id]; !exists {
			added = append(added, event)
		}
	}
	for id, event := range before {
		if _, exists := after[id]; !exists {
			removed = append(removed, event)
		}
	}
	sort.Slice(added, func(i, j int) bool { return added[i].EventID < added[j].EventID })
	sort.Slice(removed, func(i, j int) bool { return removed[i].EventID < removed[j].EventID })
	return added, removed
}

type genericResearchSection struct {
	Text         string   `json:"text"`
	EvidenceKeys []string `json:"evidence_keys"`
}

type genericResearchResult struct {
	ModelVersion string                            `json:"model_version"`
	Sections     map[string]genericResearchSection `json:"sections"`
	Strengths    []genericResearchSection          `json:"strengths"`
	Risks        []genericResearchSection          `json:"risks"`
}

func compareResearchAssumptions(previousJSON, currentJSON json.RawMessage, domains []TaiwanResearchDomainComparison) []TaiwanResearchAssumptionComparison {
	var previous, current genericResearchResult
	if json.Unmarshal(previousJSON, &previous) != nil || json.Unmarshal(currentJSON, &current) != nil {
		return []TaiwanResearchAssumptionComparison{}
	}
	domainStates := map[string]string{}
	for _, domain := range domains {
		domainStates[domain.Domain] = domain.State
	}
	result := []TaiwanResearchAssumptionComparison{}
	sectionNames := make([]string, 0, len(previous.Sections))
	for name := range previous.Sections {
		sectionNames = append(sectionNames, name)
	}
	sort.Strings(sectionNames)
	for _, name := range sectionNames {
		prior := previous.Sections[name]
		status := assumptionStatus(prior, current.Sections[name], domainStates[name], "section", current)
		result = append(result, TaiwanResearchAssumptionComparison{Kind: "section", Section: name, Text: prior.Text, EvidenceKeys: prior.EvidenceKeys, Status: status})
	}
	for _, group := range []struct {
		name  string
		items []genericResearchSection
	}{{"strength", previous.Strengths}, {"risk", previous.Risks}} {
		for _, prior := range group.items {
			status := assumptionStatus(prior, genericResearchSection{}, evidenceKeysDomainState(prior.EvidenceKeys, domainStates), group.name, current)
			result = append(result, TaiwanResearchAssumptionComparison{Kind: group.name, Text: prior.Text, EvidenceKeys: prior.EvidenceKeys, Status: status})
		}
	}
	return result
}

func assumptionStatus(previous, current genericResearchSection, evidenceState, kind string, currentResearch genericResearchResult) string {
	if evidenceState == "unavailable" || evidenceState == "partial" || evidenceState == "stale" || evidenceState == "no_longer_available" {
		return "insufficient_evidence"
	}
	if kind == "section" {
		if evidenceState == "changed" || evidenceState == "newly_available" {
			return "weakened"
		}
		if previous.Text == current.Text && sameStrings(previous.EvidenceKeys, current.EvidenceKeys) {
			return "still_supported"
		}
		return "not_comparable"
	}
	sameGroup, oppositeGroup := currentResearch.Strengths, currentResearch.Risks
	if kind == "risk" {
		sameGroup, oppositeGroup = currentResearch.Risks, currentResearch.Strengths
	}
	if containsEvidenceSet(oppositeGroup, previous.EvidenceKeys) {
		return "contradicted_by_new_evidence"
	}
	if evidenceState == "changed" || evidenceState == "newly_available" {
		return "weakened"
	}
	if containsEvidenceSet(sameGroup, previous.EvidenceKeys) {
		return "still_supported"
	}
	return "not_comparable"
}

func evidenceKeysDomainState(keys []string, states map[string]string) string {
	state := "unchanged"
	for _, key := range keys {
		domain := researchEvidenceKeyDomain(key)
		candidate := states[domain]
		if candidate == "unavailable" || candidate == "partial" || candidate == "stale" || candidate == "no_longer_available" {
			return candidate
		}
		if candidate == "changed" || candidate == "newly_available" {
			state = candidate
		}
	}
	return state
}

func researchEvidenceKeyDomain(key string) string {
	switch {
	case strings.HasPrefix(key, "interpretation.components."):
		parts := strings.Split(key, ".")
		if len(parts) > 2 {
			return parts[2]
		}
	case strings.HasPrefix(key, "price_history_summary."), strings.HasPrefix(key, "quote."):
		return "price"
	case strings.HasPrefix(key, "market_context."):
		return "market"
	case strings.HasPrefix(key, "industry_context."):
		return "industry"
	case strings.HasPrefix(key, "institutional."):
		return "institutional"
	case strings.HasPrefix(key, "margin."):
		return "margin"
	case strings.HasPrefix(key, "fundamentals."):
		return "fundamentals"
	case strings.HasPrefix(key, "corporate_events."):
		return "corporate_events"
	}
	return strings.SplitN(key, ".", 2)[0]
}

func containsEvidenceSet(items []genericResearchSection, keys []string) bool {
	for _, item := range items {
		if sameStrings(item.EvidenceKeys, keys) {
			return true
		}
	}
	return false
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	a, b := append([]string(nil), left...), append([]string(nil), right...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
