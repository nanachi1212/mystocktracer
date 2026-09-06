package stockanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/hermes"
)

type fakeTaiwanResearchPrompter struct {
	content        string
	err            error
	system, prompt string
}

func (p *fakeTaiwanResearchPrompter) PromptIsolated(_ context.Context, system, prompt string) (hermes.PromptResult, error) {
	p.system, p.prompt = system, prompt
	return hermes.PromptResult{Content: p.content}, p.err
}

func validTaiwanResearchJSON(symbol string) string {
	return `{"model_version":"taiwan_ai_research_v1","symbol":"` + symbol + `","headline":"證據呈現分歧","summary":"價格與市場狀態並不一致，資料限制如下。","sections":{"price":{"text":"完成交易日價格結構。","evidence_keys":["interpretation.components.price"]},"market":{"text":"市場狀態。","evidence_keys":["market_context.state"]},"industry":{"text":"產業相對狀態。","evidence_keys":["interpretation.components.industry"]},"institutional":{"text":"三類法人方向。","evidence_keys":["interpretation.components.institutional"]},"margin":{"text":"融資券變化。","evidence_keys":["interpretation.components.margin"]},"fundamentals":{"text":"營收資料。","evidence_keys":["interpretation.components.fundamentals"]}},"strengths":[],"risks":[],"conflicts":["價格與市場狀態不同"],"data_limitations":["盤中報價未視為完成交易日"],"research_notes":["後續觀察 evidence 是否改變"]}`
}

func researchInput() TaiwanStockIntelligence {
	i := TaiwanStockIntelligence{Symbol: "2330.TWSE", Identity: TaiwanIdentityEvidence{Name: "測試公司", Exchange: "TWSE", SecurityType: "stock"}, PriceHistory: TaiwanPriceHistoryEvidence{Return5D: f64(1), Return20D: f64(2), Bars: nil}, Quote: TaiwanQuoteEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "official_close", Freshness: "unavailable"}}}
	v := CalculateTaiwanStockInterpretation(i)
	i.Interpretation = &v
	return i
}

func TestTaiwanResearchPayloadIsBoundedDeterministicAndUsesM4BState(t *testing.T) {
	input := researchInput()
	input.PriceHistory.Bars = make([]foundation.KLine, 21)
	before, _ := json.Marshal(input)
	one, two := BuildTaiwanResearchPayload(input), BuildTaiwanResearchPayload(input)
	a, _ := json.Marshal(one)
	b, _ := json.Marshal(two)
	after, _ := json.Marshal(input)
	if string(a) != string(b) || string(before) != string(after) || strings.Contains(string(a), "bars") || one.Price["state"] != input.Interpretation.Components.Price.State {
		t.Fatalf("payload=%s", a)
	}
}

func TestGenerateTaiwanResearchUsesIsolatedGroundedPrompt(t *testing.T) {
	p := &fakeTaiwanResearchPrompter{content: validTaiwanResearchJSON("2330.TWSE")}
	got := GenerateTaiwanResearch(context.Background(), p, researchInput(), time.Unix(1, 0))
	if got.Status != "available" {
		t.Fatalf("research=%+v", got)
	}
	for _, required := range []string{"台灣股票", "只能使用", "模型記憶", "外部資訊", "不得預測", "不得推薦", "繁體中文", "JSON"} {
		if !strings.Contains(p.system, required) {
			t.Fatalf("system missing %q: %s", required, p.system)
		}
	}
	for _, forbidden := range []string{"A股全局交易決策器", "游資", "打板", "連板", "首板", "NextDayPlan", "position hint", "stop loss", "take profit", "scorecard", "price_history_summary\":{\"bars"} {
		if strings.Contains(p.prompt, forbidden) || strings.Contains(p.system, forbidden) {
			t.Fatalf("prompt contains %q", forbidden)
		}
	}
}

func TestTaiwanResearchRejectsInvalidOutputAndKeepsEvidenceAvailable(t *testing.T) {
	input := researchInput()
	for _, tt := range []struct{ name, content string }{
		{"wrong symbol", strings.Replace(validTaiwanResearchJSON("2330.TWSE"), "2330.TWSE", "2330", 1)},
		{"wrong version", strings.Replace(validTaiwanResearchJSON("2330.TWSE"), TaiwanAIResearchVersion, "latest", 1)},
		{"unknown key", strings.Replace(validTaiwanResearchJSON("2330.TWSE"), "interpretation.components.price", "foo.magic_score", 1)},
		{"malformed", "{"},
		{"forbidden", strings.Replace(validTaiwanResearchJSON("2330.TWSE"), "證據呈現分歧", "建議買進", 1)},
		{"oversized array", strings.Replace(validTaiwanResearchJSON("2330.TWSE"), `"conflicts":["價格與市場狀態不同"]`, `"conflicts":["1","2","3","4","5","6","7","8","9"]`, 1)},
		{"model provenance", strings.Replace(validTaiwanResearchJSON("2330.TWSE"), `"headline"`, `"status":"available","headline"`, 1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := &fakeTaiwanResearchPrompter{content: tt.content}
			got := GenerateTaiwanResearch(context.Background(), p, input, time.Now())
			if got.Status != "unavailable" || input.Interpretation == nil {
				t.Fatalf("got=%+v", got)
			}
		})
	}
	p := &fakeTaiwanResearchPrompter{err: errors.New("provider failed")}
	if got := GenerateTaiwanResearch(context.Background(), p, input, time.Now()); got.Status != "unavailable" {
		t.Fatalf("failure=%+v", got)
	}
}

func TestTaiwanResearchRejectsAbsentEvidenceAndETFMisuse(t *testing.T) {
	input := researchInput()
	absent := strings.Replace(validTaiwanResearchJSON(input.Symbol), `"interpretation.components.industry"`, `"industry_context.industry.relative_breadth"`, 1)
	if got := GenerateTaiwanResearch(context.Background(), &fakeTaiwanResearchPrompter{content: absent}, input, time.Now()); got.Status != "unavailable" {
		t.Fatalf("absent evidence accepted: %+v", got)
	}
	input.Identity.SecurityType = foundation.SecurityTypeETF
	if got := GenerateTaiwanResearch(context.Background(), &fakeTaiwanResearchPrompter{content: validTaiwanResearchJSON(input.Symbol)}, input, time.Now()); got.Status != "unavailable" {
		t.Fatalf("ETF company-like sections accepted: %+v", got)
	}
	etf := strings.Replace(validTaiwanResearchJSON(input.Symbol), `"industry":{"text":"產業相對狀態。"`, `"industry":{"text":"ETF 產業分析不適用。"`, 1)
	etf = strings.Replace(etf, `"fundamentals":{"text":"營收資料。"`, `"fundamentals":{"text":"ETF 公司基本面不適用。"`, 1)
	if got := GenerateTaiwanResearch(context.Background(), &fakeTaiwanResearchPrompter{content: etf}, input, time.Now()); got.Status != "available" {
		t.Fatalf("valid ETF limitation rejected: %+v", got)
	}
}

// ==================================================
// M8C -- Evidence-grounded Taiwan Research Synthesis: compact M8A whitelist, strengths/risks schema.
// ==================================================

func m8cStatement(category string) *foundation.FinancialStatementPeriod {
	return &foundation.FinancialStatementPeriod{
		AccountingCategory: category, FiscalYear: 2026, FiscalQuarter: 2,
		CumulativeEPS: f64(9.55), GrossMargin: f64(67.03), OperatingMargin: f64(59.29), NetMargin: f64(53.22),
		BalanceFiscalYear: 2026, BalanceFiscalQuarter: 1, BookValuePerShare: f64(28.5), DebtRatio: f64(30.94), DebtToEquity: f64(44.81), CurrentRatio: f64(245.76),
		CashflowFiscalYear: 2025, CashflowFiscalQuarter: 4, CashflowStatus: "available", OperatingCashFlow: i64ptr(1122637757), CashFlowToNetIncome: f64(148.06),
		// Raw ingredients that must never reach the AI payload even though they exist on this struct.
		Revenue: i64ptr(999000000), GrossProfit: i64ptr(1), OperatingIncome: i64ptr(1), NetIncome: i64ptr(888000000),
		TotalAssets: i64ptr(1), TotalLiabilities: i64ptr(1), Equity: i64ptr(1), CurrentAssets: i64ptr(1), CurrentLiabilities: i64ptr(1),
	}
}

func m8cValuation() *foundation.ValuationSnapshot {
	return &foundation.ValuationSnapshot{DataDate: "2026-09-04", PE: f64(18.5), PB: f64(6.2), DividendYield: f64(2.1)}
}

// fundamentalsInput builds a researchInput() with Fundamentals.Data populated for M8C payload
// testing; valuation/statement may each be nil to exercise the unavailable/not_applicable paths.
func fundamentalsInput(valuation *foundation.ValuationSnapshot, statement *foundation.FinancialStatementPeriod) TaiwanStockIntelligence {
	return fundamentalsInputWithCapabilities(valuation, statement, nil)
}

// M8C.2 — same as fundamentalsInput but also sets TaiwanFundamentals.Capabilities, so a valuation/
// financial_statement provider failure can carry its authoritative data_insufficient reason (see
// providers/taiwan/fundamentals.go's Fundamentals(), which always populates these two keys).
func fundamentalsInputWithCapabilities(valuation *foundation.ValuationSnapshot, statement *foundation.FinancialStatementPeriod, capabilities map[string]foundation.FundamentalCapability) TaiwanStockIntelligence {
	i := researchInput()
	i.Fundamentals = TaiwanFundamentalsEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "available"}, Data: &foundation.TaiwanFundamentals{
		Revenue: []foundation.MonthlyRevenue{{Period: "2026-08", OfficialYoY: f64(12.16)}}, Valuation: valuation, Statement: statement, Capabilities: capabilities,
	}}
	v := CalculateTaiwanStockInterpretation(i)
	i.Interpretation = &v
	return i
}

func TestM8CPayloadIncludesApprovedFieldsExcludesNetMarginAndRawComponents(t *testing.T) {
	payload := BuildTaiwanResearchPayload(fundamentalsInput(m8cValuation(), m8cStatement("ci")))
	raw, _ := json.Marshal(payload)
	s := string(raw)
	for _, want := range []string{
		`"pe":18.5`, `"pb":6.2`, `"dividend_yield_percent":2.1`, `"data_date":"2026-09-04"`,
		`"cumulative_eps":9.55`, `"gross_margin_percent":67.03`, `"operating_margin_percent":59.29`,
		`"book_value_per_share":28.5`, `"debt_ratio_percent":30.94`, `"debt_to_equity_percent":44.81`, `"current_ratio_percent":245.76`,
		`"operating_cash_flow":1122637757`, `"cash_flow_to_net_income":148.06`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("payload missing %q: %s", want, s)
		}
	}
	for _, forbidden := range []string{"net_margin", "53.22", "total_assets", "total_liabilities", `"equity"`, "current_assets", "current_liabilities", `"revenue"`, `"net_income":`, "gross_profit", "operating_income", "888000000", "999000000"} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("payload leaks forbidden field %q: %s", forbidden, s)
		}
	}
}

func TestM8CPayloadPreservesIndependentPeriods(t *testing.T) {
	payload := BuildTaiwanResearchPayload(fundamentalsInput(m8cValuation(), m8cStatement("ci")))
	raw, _ := json.Marshal(payload)
	s := string(raw)
	for _, want := range []string{`"fiscal_year":2026`, `"fiscal_quarter":2`, `"balance_fiscal_year":2026`, `"balance_fiscal_quarter":1`, `"cashflow_fiscal_year":2025`, `"cashflow_fiscal_quarter":4`} {
		if !strings.Contains(s, want) {
			t.Fatalf("payload missing independent period %q: %s", want, s)
		}
	}
}

func TestM8CPayloadStatusUnavailableWhenSubDomainAbsent(t *testing.T) {
	payload := BuildTaiwanResearchPayload(fundamentalsInput(nil, nil))
	if payload.Fundamentals["valuation_status"] != "unavailable" || payload.Fundamentals["financial_statement_status"] != "unavailable" ||
		payload.Fundamentals["balance_status"] != "unavailable" || payload.Fundamentals["cashflow_status"] != "unavailable" {
		t.Fatalf("sub-domain status not independently unavailable: %+v", payload.Fundamentals)
	}
	raw, _ := json.Marshal(payload)
	for _, absent := range []string{`"valuation":{`, `"financial_statement":{`, `"balance":{`, `"cashflow":{`} {
		if strings.Contains(string(raw), absent) {
			t.Fatalf("absent sub-domain must not appear as a nested object: %s", raw)
		}
	}
}

func TestM8CPayloadStatusNotApplicableForNonStock(t *testing.T) {
	input := fundamentalsInput(nil, nil)
	input.Identity.SecurityType = foundation.SecurityTypeETF
	input.Fundamentals = TaiwanFundamentalsEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "not_applicable"}}
	v := CalculateTaiwanStockInterpretation(input)
	input.Interpretation = &v
	payload := BuildTaiwanResearchPayload(input)
	if payload.Fundamentals["valuation_status"] != "not_applicable" || payload.Fundamentals["cashflow_status"] != "not_applicable" {
		t.Fatalf("non-stock sub-domain status should be not_applicable, not unavailable: %+v", payload.Fundamentals)
	}
}

// M8C.1 — cashflow_status must reflect the archive's own authoritative status (propagated from
// providers/taiwan's cashflowSnapshot.Status via FinancialStatementPeriod.CashflowStatus), never be
// inferred purely from metric/period presence. A present, valid operating_cash_flow value must NOT
// override an authoritative "partial" status into "available".
func TestM8CCashflowStatusReflectsAuthoritativeSnapshotNotMetricPresence(t *testing.T) {
	partial := m8cStatement("ci")
	partial.CashflowStatus = "partial"
	payload := BuildTaiwanResearchPayload(fundamentalsInput(m8cValuation(), partial))
	if payload.Fundamentals["cashflow_status"] != "partial" {
		t.Fatalf("authoritative partial status must survive even though operating_cash_flow is present and valid: %+v", payload.Fundamentals)
	}
	cf, _ := payload.Fundamentals["cashflow"].(map[string]any)
	if cf["operating_cash_flow"] == nil {
		t.Fatal("the metric itself must still be present -- partial status describes confidence, not absence")
	}

	available := m8cStatement("ci")
	available.CashflowStatus = "available"
	payloadAvailable := BuildTaiwanResearchPayload(fundamentalsInput(m8cValuation(), available))
	if payloadAvailable.Fundamentals["cashflow_status"] != "available" {
		t.Fatalf("explicit available status must be preserved as available: %+v", payloadAvailable.Fundamentals)
	}

	// M8C.2 — defensive fallback: an unexpected empty CashflowStatus (should never happen in
	// practice, since statement() always sets it alongside the period) must resolve to the
	// conservative "unavailable", never a fabricated "available" that overstates confidence beyond
	// what the missing metadata actually supports.
	fallback := m8cStatement("ci")
	fallback.CashflowStatus = ""
	payloadFallback := BuildTaiwanResearchPayload(fundamentalsInput(m8cValuation(), fallback))
	if payloadFallback.Fundamentals["cashflow_status"] != "unavailable" {
		t.Fatalf("empty CashflowStatus must fall back to unavailable (conservative), not available: %+v", payloadFallback.Fundamentals)
	}
	availableKeys := availableTaiwanResearchEvidenceKeys(payloadFallback)
	if availableKeys["fundamentals.data.cashflow.operating_cash_flow"] {
		t.Fatal("cashflow metrics must not be citeable when the fallback status is unavailable, even though the metric value is present")
	}
}

func TestM8CPayloadZeroNegativeAndNullPreserved(t *testing.T) {
	statement := m8cStatement("ci")
	zero, negativeOCF, negativeRatio := f64(0), i64ptr(-150005), f64(-6.34)
	statement.CumulativeEPS = nil
	statement.OperatingCashFlow, statement.CashFlowToNetIncome = negativeOCF, negativeRatio
	valuation := m8cValuation()
	valuation.DividendYield = zero
	payload := BuildTaiwanResearchPayload(fundamentalsInput(valuation, statement))
	raw, _ := json.Marshal(payload)
	s := string(raw)
	if !strings.Contains(s, `"dividend_yield_percent":0`) {
		t.Fatalf("explicit zero dropped: %s", s)
	}
	if !strings.Contains(s, `"operating_cash_flow":-150005`) || !strings.Contains(s, `"cash_flow_to_net_income":-6.34`) {
		t.Fatalf("negative values not preserved: %s", s)
	}
	if !strings.Contains(s, `"cumulative_eps":null`) {
		t.Fatalf("null must stay null, never become 0: %s", s)
	}
	available := availableTaiwanResearchEvidenceKeys(payload)
	if !available["fundamentals.data.valuation.dividend_yield_percent"] {
		t.Fatal("explicit zero must remain an available/citable evidence key")
	}
	if !available["fundamentals.data.cashflow.operating_cash_flow"] || !available["fundamentals.data.cashflow.cash_flow_to_net_income"] {
		t.Fatal("negative values must remain available/citable evidence keys")
	}
	if available["fundamentals.data.financial_statement.cumulative_eps"] {
		t.Fatal("null cumulative_eps must not be citable")
	}
}

func TestM8CCiOnlyRatiosUnavailableForNonCiCategory(t *testing.T) {
	// Representative fh (financial holding, e.g. 2882.TWSE) case: the ci-only ratio policy has
	// already nulled these fields upstream in the real provider -- simulate that here.
	statement := m8cStatement("fh")
	statement.GrossMargin, statement.OperatingMargin, statement.DebtRatio, statement.DebtToEquity, statement.CurrentRatio = nil, nil, nil, nil, nil
	payload := BuildTaiwanResearchPayload(fundamentalsInput(m8cValuation(), statement))
	fs, _ := payload.Fundamentals["financial_statement"].(map[string]any)
	balance, _ := payload.Fundamentals["balance"].(map[string]any)
	if fs["margin_applicability"] != "not_applicable_for_category" || balance["ratio_applicability"] != "not_applicable_for_category" {
		t.Fatalf("fh category must be marked not_applicable_for_category, not a data gap: fs=%+v balance=%+v", fs, balance)
	}
	// All-category fields remain available for a fh company.
	eps, _ := fs["cumulative_eps"].(*float64)
	bvps, _ := balance["book_value_per_share"].(*float64)
	if eps == nil || *eps != 9.55 || bvps == nil || *bvps != 28.5 {
		t.Fatalf("all-category fields must remain available for fh: fs=%+v balance=%+v", fs, balance)
	}
	available := availableTaiwanResearchEvidenceKeys(payload)
	for _, key := range []string{
		"fundamentals.data.financial_statement.gross_margin_percent", "fundamentals.data.financial_statement.operating_margin_percent",
		"fundamentals.data.balance.debt_ratio_percent", "fundamentals.data.balance.debt_to_equity_percent", "fundamentals.data.balance.current_ratio_percent",
	} {
		if available[key] {
			t.Fatalf("ci-only key must never be citable for a non-ci category: %s", key)
		}
	}
	if !available["fundamentals.data.financial_statement.cumulative_eps"] || !available["fundamentals.data.balance.book_value_per_share"] {
		t.Fatal("all-category fields must remain citable for fh")
	}
}

func TestM8CPartialDomainDoesNotFailWholePayload(t *testing.T) {
	// valuation + financial statement available; balance and cashflow absent.
	statement := m8cStatement("ci")
	statement.BalanceFiscalYear, statement.BalanceFiscalQuarter, statement.DebtRatio, statement.DebtToEquity, statement.CurrentRatio, statement.BookValuePerShare = 0, 0, nil, nil, nil, nil
	statement.CashflowFiscalYear, statement.CashflowFiscalQuarter, statement.OperatingCashFlow, statement.CashFlowToNetIncome = 0, 0, nil, nil
	input := fundamentalsInput(m8cValuation(), statement)
	payload := BuildTaiwanResearchPayload(input)
	if payload.Fundamentals["valuation_status"] != "available" || payload.Fundamentals["financial_statement_status"] != "available" {
		t.Fatalf("available domains must remain available: %+v", payload.Fundamentals)
	}
	if payload.Fundamentals["balance_status"] != "unavailable" || payload.Fundamentals["cashflow_status"] != "unavailable" {
		t.Fatalf("absent domains must be unavailable, not fabricated: %+v", payload.Fundamentals)
	}
	available := availableTaiwanResearchEvidenceKeys(payload)
	for _, key := range []string{"fundamentals.data.balance.book_value_per_share", "fundamentals.data.cashflow.operating_cash_flow"} {
		if available[key] {
			t.Fatalf("absent domain key must not be citable: %s", key)
		}
	}
	// GenerateTaiwanResearch must not fail merely because an optional domain is absent.
	p := &fakeTaiwanResearchPrompter{content: validTaiwanResearchJSON(input.Symbol)}
	if got := GenerateTaiwanResearch(context.Background(), p, input, time.Now()); got.Status != "available" {
		t.Fatalf("partial-domain evidence must not fail the whole research response: %+v", got)
	}
}

func TestM8CBackendNeverReferencesM8BScreenerContext(t *testing.T) {
	raw, err := os.ReadFile("taiwan_research.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if strings.Contains(source, "TaiwanResearchEntryContext") || strings.Contains(strings.ToLower(source), "screener") {
		t.Fatal("M8B Screener context must never reach the AI research contract")
	}
}

func TestM8CEvidenceKeyAvailabilityRules(t *testing.T) {
	payload := BuildTaiwanResearchPayload(fundamentalsInput(m8cValuation(), m8cStatement("ci")))
	available := availableTaiwanResearchEvidenceKeys(payload)
	for _, key := range []string{
		"fundamentals.data.valuation.pe", "fundamentals.data.valuation.pb", "fundamentals.data.valuation.dividend_yield_percent",
		"fundamentals.data.financial_statement.cumulative_eps", "fundamentals.data.financial_statement.gross_margin_percent", "fundamentals.data.financial_statement.operating_margin_percent",
		"fundamentals.data.balance.book_value_per_share", "fundamentals.data.balance.debt_ratio_percent", "fundamentals.data.balance.debt_to_equity_percent", "fundamentals.data.balance.current_ratio_percent",
		"fundamentals.data.cashflow.operating_cash_flow", "fundamentals.data.cashflow.cash_flow_to_net_income",
	} {
		if !available[key] {
			t.Fatalf("expected key available: %s", key)
		}
	}
	if _, ok := taiwanResearchEvidenceKeys["fundamentals.data.financial_statement.net_margin_percent"]; ok {
		t.Fatal("net_margin must never be an allowlisted evidence key")
	}
}

func TestM8CStrengthsRisksSchema(t *testing.T) {
	input := fundamentalsInput(m8cValuation(), m8cStatement("ci"))
	base := validTaiwanResearchJSON(input.Symbol)
	// Empty strengths/risks is valid and must not be rejected.
	if got := GenerateTaiwanResearch(context.Background(), &fakeTaiwanResearchPrompter{content: base}, input, time.Now()); got.Status != "available" || got.Strengths == nil || got.Risks == nil || len(got.Strengths) != 0 || len(got.Risks) != 0 {
		t.Fatalf("empty strengths/risks should be accepted: %+v", got)
	}
	// A valid, evidence-grounded strength citing an M8C key is accepted.
	withStrength := strings.Replace(base, `"strengths":[]`, `"strengths":[{"text":"營業活動現金流量為正。","evidence_keys":["fundamentals.data.cashflow.operating_cash_flow"]}]`, 1)
	if got := GenerateTaiwanResearch(context.Background(), &fakeTaiwanResearchPrompter{content: withStrength}, input, time.Now()); got.Status != "available" || len(got.Strengths) != 1 {
		t.Fatalf("valid strength rejected: %+v", got)
	}
	// A valid risk citing an M8C key is accepted.
	withRisk := strings.Replace(base, `"risks":[]`, `"risks":[{"text":"營業現金流／淨利為負，屬風險證據。","evidence_keys":["fundamentals.data.cashflow.cash_flow_to_net_income"]}]`, 1)
	if got := GenerateTaiwanResearch(context.Background(), &fakeTaiwanResearchPrompter{content: withRisk}, input, time.Now()); got.Status != "available" || len(got.Risks) != 1 {
		t.Fatalf("valid risk rejected: %+v", got)
	}
	// Missing strengths/risks entirely (omitted field, not even []) is rejected.
	missing := strings.Replace(base, `"strengths":[],"risks":[],`, "", 1)
	if got := GenerateTaiwanResearch(context.Background(), &fakeTaiwanResearchPrompter{content: missing}, input, time.Now()); got.Status != "unavailable" {
		t.Fatalf("omitted strengths/risks must be rejected: %+v", got)
	}
	// A strength item citing an unavailable evidence key is rejected (fh company, ci-only key).
	fhInput := fundamentalsInput(m8cValuation(), func() *foundation.FinancialStatementPeriod { s := m8cStatement("fh"); s.DebtRatio = nil; return s }())
	badStrength := strings.Replace(validTaiwanResearchJSON(fhInput.Symbol), `"strengths":[]`, `"strengths":[{"text":"負債比率健康。","evidence_keys":["fundamentals.data.balance.debt_ratio_percent"]}]`, 1)
	if got := GenerateTaiwanResearch(context.Background(), &fakeTaiwanResearchPrompter{content: badStrength}, fhInput, time.Now()); got.Status != "unavailable" {
		t.Fatalf("strength citing unavailable/not-applicable key must be rejected: %+v", got)
	}
	// A risk item citing an unavailable evidence key is rejected the same way.
	badRisk := strings.Replace(validTaiwanResearchJSON(fhInput.Symbol), `"risks":[]`, `"risks":[{"text":"負債比率偏高。","evidence_keys":["fundamentals.data.balance.debt_ratio_percent"]}]`, 1)
	if got := GenerateTaiwanResearch(context.Background(), &fakeTaiwanResearchPrompter{content: badRisk}, fhInput, time.Now()); got.Status != "unavailable" {
		t.Fatalf("risk citing unavailable/not-applicable key must be rejected: %+v", got)
	}
	// Too many strengths (exceeds bound) is rejected.
	over := `[{"text":"a","evidence_keys":["fundamentals.data.valuation.pe"]},{"text":"b","evidence_keys":["fundamentals.data.valuation.pe"]},{"text":"c","evidence_keys":["fundamentals.data.valuation.pe"]},{"text":"d","evidence_keys":["fundamentals.data.valuation.pe"]},{"text":"e","evidence_keys":["fundamentals.data.valuation.pe"]},{"text":"f","evidence_keys":["fundamentals.data.valuation.pe"]},{"text":"g","evidence_keys":["fundamentals.data.valuation.pe"]}]`
	oversized := strings.Replace(base, `"strengths":[]`, `"strengths":`+over, 1)
	if got := GenerateTaiwanResearch(context.Background(), &fakeTaiwanResearchPrompter{content: oversized}, input, time.Now()); got.Status != "unavailable" {
		t.Fatalf("oversized strengths array must be rejected: %+v", got)
	}
}

func TestM8CPredictiveCausalAndDerivedMetricLanguageRejected(t *testing.T) {
	input := fundamentalsInput(m8cValuation(), m8cStatement("ci"))
	for _, tt := range []struct {
		name string
		text string
	}{
		{"predictive rise", "本益比偏低，未來股價將會上漲。"},
		{"predictive causal", "本益比偏低，因此股價會上漲。"},
		{"upside target language", "股價上看新高。"},
		{"ROA derivation", "資產報酬率（ROA）表現良好。"},
		{"FCF derivation", "自由現金流（FCF）充裕。"},
		{"PEG derivation", "PEG 顯示成長合理。"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			content := strings.Replace(validTaiwanResearchJSON(input.Symbol), "價格與市場狀態並不一致，資料限制如下。", tt.text, 1)
			if got := GenerateTaiwanResearch(context.Background(), &fakeTaiwanResearchPrompter{content: content}, input, time.Now()); got.Status != "unavailable" {
				t.Fatalf("forbidden language accepted: %+v", got)
			}
		})
	}
}

// ==================================================
// M8C.2 -- Capability Status Fidelity Finalization: preserve authoritative data_insufficient from
// TaiwanFundamentals.Capabilities, and never let internally-inconsistent state make an
// unavailable/data_insufficient/not_applicable domain's metrics citeable.
// ==================================================

func dataInsufficientCapability(reason string) foundation.FundamentalCapability {
	return foundation.FundamentalCapability{Status: "data_insufficient", Reason: reason}
}

func TestM8CValuationAuthoritativeDataInsufficientPreserved(t *testing.T) {
	// data.Valuation == nil (the provider's c.valuation() call itself failed), but Capabilities
	// carries the authoritative reason -- exactly what providers/taiwan's Fundamentals() produces.
	input := fundamentalsInputWithCapabilities(nil, m8cStatement("ci"), map[string]foundation.FundamentalCapability{
		"valuation": dataInsufficientCapability("official valuation row unavailable"),
	})
	payload := BuildTaiwanResearchPayload(input)
	if payload.Fundamentals["valuation_status"] != "data_insufficient" {
		t.Fatalf("authoritative data_insufficient must be preserved, not collapsed to unavailable: %+v", payload.Fundamentals)
	}
	if _, ok := payload.Fundamentals["valuation"]; ok {
		t.Fatal("no valuation metrics should be present when the domain itself failed")
	}
	available := availableTaiwanResearchEvidenceKeys(payload)
	for _, key := range []string{"fundamentals.data.valuation.pe", "fundamentals.data.valuation.pb", "fundamentals.data.valuation.dividend_yield_percent"} {
		if available[key] {
			t.Fatalf("valuation keys must not be citeable when the domain is data_insufficient: %s", key)
		}
	}
}

func TestM8CFinancialStatementAuthoritativeDataInsufficientPreserved(t *testing.T) {
	// data.Statement == nil (statement() itself failed), but Capabilities carries the reason.
	// financial_statement and balance share the same underlying failure (existing coupling, not
	// solved here) -- both must inherit data_insufficient together, never split or downgraded.
	input := fundamentalsInputWithCapabilities(m8cValuation(), nil, map[string]foundation.FundamentalCapability{
		"financial_statement": dataInsufficientCapability("financial category unsupported"),
	})
	payload := BuildTaiwanResearchPayload(input)
	if payload.Fundamentals["financial_statement_status"] != "data_insufficient" || payload.Fundamentals["balance_status"] != "data_insufficient" {
		t.Fatalf("authoritative data_insufficient must be preserved for both coupled domains: %+v", payload.Fundamentals)
	}
	available := availableTaiwanResearchEvidenceKeys(payload)
	for _, key := range []string{
		"fundamentals.data.financial_statement.cumulative_eps", "fundamentals.data.financial_statement.gross_margin_percent",
		"fundamentals.data.balance.book_value_per_share", "fundamentals.data.balance.debt_ratio_percent",
	} {
		if available[key] {
			t.Fatalf("keys must not be citeable when the domain is data_insufficient: %s", key)
		}
	}
}

func TestM8CValuationWithoutCapabilityEntryStaysUnavailable(t *testing.T) {
	// No Capabilities entry at all (e.g. an older/incomplete fixture) must not fabricate
	// data_insufficient out of nothing -- the existing conservative "unavailable" default applies.
	input := fundamentalsInputWithCapabilities(nil, m8cStatement("ci"), nil)
	payload := BuildTaiwanResearchPayload(input)
	if payload.Fundamentals["valuation_status"] != "unavailable" {
		t.Fatalf("missing capability metadata must stay unavailable, not invent data_insufficient: %+v", payload.Fundamentals)
	}
}

func TestM8CCashflowPartialMetricsRemainCiteable(t *testing.T) {
	partial := m8cStatement("ci")
	partial.CashflowStatus = "partial"
	payload := BuildTaiwanResearchPayload(fundamentalsInput(m8cValuation(), partial))
	available := availableTaiwanResearchEvidenceKeys(payload)
	if !available["fundamentals.data.cashflow.operating_cash_flow"] || !available["fundamentals.data.cashflow.cash_flow_to_net_income"] {
		t.Fatal("partial means lower-confidence batch evidence, not absence -- actual metrics must remain citeable")
	}
}

func TestM8CInconsistentDomainStateNeverMakesMetricsCiteable(t *testing.T) {
	// Directly constructs an internally-inconsistent TaiwanResearchPayload (a shape that
	// addTaiwanM8CEvidence itself never produces, since the metric map and *_status are always set
	// together) to prove availableTaiwanResearchEvidenceKeys()'s status gate is a real, independent
	// safety net -- not merely an artifact of BuildTaiwanResearchPayload's own consistency.
	payload := TaiwanResearchPayload{
		Fundamentals: map[string]any{
			"valuation_status": "data_insufficient", "valuation": map[string]any{"pe": f64(18.5)},
			"financial_statement_status": "unavailable", "financial_statement": map[string]any{"cumulative_eps": f64(9.55), "margin_applicability": "applicable"},
			"balance_status": "not_applicable", "balance": map[string]any{"book_value_per_share": f64(28.5), "ratio_applicability": "applicable"},
			"cashflow_status": "unavailable", "cashflow": map[string]any{"operating_cash_flow": i64ptr(1000), "cash_flow_to_net_income": f64(10)},
		},
	}
	available := availableTaiwanResearchEvidenceKeys(payload)
	for _, key := range []string{
		"fundamentals.data.valuation.pe", "fundamentals.data.financial_statement.cumulative_eps",
		"fundamentals.data.balance.book_value_per_share", "fundamentals.data.cashflow.operating_cash_flow", "fundamentals.data.cashflow.cash_flow_to_net_income",
	} {
		if available[key] {
			t.Fatalf("a numeric value present alongside a non-available status must never become citeable: %s", key)
		}
	}
}
