package stockanalysis

import (
	"encoding/json"
	"testing"
)

func historyEvidenceJSON(t *testing.T, version string, overrides map[string]map[string]any) json.RawMessage {
	t.Helper()
	payload := map[string]any{"research_version": version, "symbol": "2330.TWSE"}
	for _, domain := range taiwanResearchDomains {
		payload[domain] = map[string]any{"status": "available"}
	}
	for domain, value := range overrides {
		payload[domain] = value
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func historyResearchJSON(t *testing.T, version string, sectionText string, strengths, risks []genericResearchSection) json.RawMessage {
	t.Helper()
	sections := map[string]genericResearchSection{}
	for _, domain := range taiwanResearchDomains {
		sections[domain] = genericResearchSection{Text: sectionText + " " + domain, EvidenceKeys: []string{domain + ".status"}}
	}
	data, err := json.Marshal(map[string]any{"model_version": version, "sections": sections, "strengths": strengths, "risks": risks})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTaiwanResearchComparisonIdenticalAndChangedNumericEvidence(t *testing.T) {
	previous := historyEvidenceJSON(t, TaiwanAIResearchVersionV2, map[string]map[string]any{"price": {"status": "available", "return_5d_percent": 10.0}})
	current := historyEvidenceJSON(t, TaiwanAIResearchVersionV2, map[string]map[string]any{"price": {"status": "available", "return_5d_percent": 15.0}})
	priorResearch := historyResearchJSON(t, TaiwanAIResearchVersionV2, "same", nil, nil)
	currentResearch := historyResearchJSON(t, TaiwanAIResearchVersionV2, "same", nil, nil)
	comparison := CompareTaiwanResearchHistory("current-run-0001", "previous-run-001", TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV2, current, previous, currentResearch, priorResearch)
	if !comparison.Comparable || comparison.Summary.Changed != 1 || comparison.Summary.Unchanged != 6 {
		t.Fatalf("comparison=%+v", comparison)
	}
	price := comparison.Domains[0]
	if len(price.ChangedFields) != 1 || price.ChangedFields[0].Delta == nil || *price.ChangedFields[0].Delta != 5 || price.ChangedFields[0].DeltaPercent == nil || *price.ChangedFields[0].DeltaPercent != 50 {
		t.Fatalf("price changes=%+v", price.ChangedFields)
	}
	identical := CompareTaiwanResearchHistory("current-run-0002", "current-run-0001", TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV2, current, current, currentResearch, currentResearch)
	if !identical.Comparable || identical.Summary.Unchanged != 7 || identical.Summary.Changed != 0 {
		t.Fatalf("identical=%+v", identical)
	}
}

func TestTaiwanResearchComparisonAvailabilityStalePartialAndCorporateEvents(t *testing.T) {
	previous := historyEvidenceJSON(t, TaiwanAIResearchVersionV1, map[string]map[string]any{
		"market": {"status": "unavailable"}, "margin": {"status": "available"},
	})
	current := historyEvidenceJSON(t, TaiwanAIResearchVersionV2, map[string]map[string]any{
		"market": {"status": "available", "state": "balanced"}, "industry": {"status": "partial"}, "margin": {"status": "unavailable"},
		"fundamentals":     {"status": "available", "freshness": "stale"},
		"corporate_events": {"status": "available", "events": []any{map[string]any{"event_id": "event-new"}}},
	})
	research := historyResearchJSON(t, TaiwanAIResearchVersionV2, "current", nil, nil)
	previousResearch := historyResearchJSON(t, TaiwanAIResearchVersionV1, "previous", nil, nil)
	comparison := CompareTaiwanResearchHistory("current-run-0003", "previous-run-003", TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV1, TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV1, current, previous, research, previousResearch)
	if !comparison.Comparable || comparison.Summary.NewlyAvailable < 1 || comparison.Summary.Partial != 1 || comparison.Summary.NoLongerAvailable != 1 || comparison.Summary.Stale != 1 || comparison.Summary.CorporateEventsAdded != 1 {
		t.Fatalf("comparison=%+v", comparison)
	}
}

func TestTaiwanResearchComparisonAssumptionsAreConservative(t *testing.T) {
	evidence := historyEvidenceJSON(t, TaiwanAIResearchVersionV2, nil)
	priorStrength := genericResearchSection{Text: "法人證據", EvidenceKeys: []string{"institutional.status"}}
	previous := historyResearchJSON(t, TaiwanAIResearchVersionV2, "same", []genericResearchSection{priorStrength}, nil)
	still := historyResearchJSON(t, TaiwanAIResearchVersionV2, "same", []genericResearchSection{priorStrength}, nil)
	comparison := CompareTaiwanResearchHistory("current-run-0004", "previous-run-004", TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV2, evidence, evidence, still, previous)
	if got := comparison.Assumptions[len(comparison.Assumptions)-1].Status; got != "still_supported" {
		t.Fatalf("status=%s", got)
	}
	contradicted := historyResearchJSON(t, TaiwanAIResearchVersionV2, "same", nil, []genericResearchSection{{Text: "法人風險", EvidenceKeys: priorStrength.EvidenceKeys}})
	comparison = CompareTaiwanResearchHistory("current-run-0005", "previous-run-005", TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV2, TaiwanAIResearchVersionV2, evidence, evidence, contradicted, previous)
	if got := comparison.Assumptions[len(comparison.Assumptions)-1].Status; got != "contradicted_by_new_evidence" {
		t.Fatalf("status=%s", got)
	}
}

func TestTaiwanResearchComparisonRejectsIncompatibleVersion(t *testing.T) {
	evidence := historyEvidenceJSON(t, "taiwan_ai_research_v99", nil)
	research := historyResearchJSON(t, TaiwanAIResearchVersionV2, "same", nil, nil)
	comparison := CompareTaiwanResearchHistory("current-run-0006", "previous-run-006", "taiwan_ai_research_v99", TaiwanAIResearchVersionV2, "taiwan_ai_research_v99", TaiwanAIResearchVersionV2, evidence, evidence, research, research)
	if comparison.Comparable || comparison.Reason == "" {
		t.Fatalf("comparison=%+v", comparison)
	}
}

func TestTaiwanResearchHistoryMetadataPreservesSourceStatesAndSafeURLs(t *testing.T) {
	payload := TaiwanResearchPayload{ResearchVersion: TaiwanAIResearchVersionV2, Price: map[string]any{"status": "available", "latest_completed_date": "2026-09-17", "source_url": "javascript:alert(1)"}, Market: map[string]any{"status": "partial"}, Fundamentals: map[string]any{"status": "available", "freshness": "stale"}}
	input := TaiwanStockIntelligence{PriceHistory: TaiwanPriceHistoryEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "available", Source: "official", SourceURL: "javascript:alert(1)"}}}
	metadata := BuildTaiwanResearchHistoryMetadata(input, payload)
	if metadata.EvidenceAsOf != "2026-09-17" || !metadata.Validity.Partial || !metadata.Validity.Stale || metadata.Validity.Completeness != "partial" {
		t.Fatalf("metadata=%+v", metadata)
	}
	if _, exists := metadata.Provenance["price"]["source_url"]; exists {
		t.Fatalf("unsafe URL persisted: %+v", metadata.Provenance["price"])
	}
}
