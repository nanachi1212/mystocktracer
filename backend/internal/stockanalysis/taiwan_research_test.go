package stockanalysis

import (
	"context"
	"encoding/json"
	"errors"
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
	return `{"model_version":"taiwan_ai_research_v1","symbol":"` + symbol + `","headline":"證據呈現分歧","summary":"價格與市場狀態並不一致，資料限制如下。","sections":{"price":{"text":"完成交易日價格結構。","evidence_keys":["interpretation.components.price"]},"market":{"text":"市場狀態。","evidence_keys":["market_context.state"]},"industry":{"text":"產業相對狀態。","evidence_keys":["interpretation.components.industry"]},"institutional":{"text":"三類法人方向。","evidence_keys":["interpretation.components.institutional"]},"margin":{"text":"融資券變化。","evidence_keys":["interpretation.components.margin"]},"fundamentals":{"text":"營收資料。","evidence_keys":["interpretation.components.fundamentals"]}},"conflicts":["價格與市場狀態不同"],"data_limitations":["盤中報價未視為完成交易日"],"research_notes":["後續觀察 evidence 是否改變"]}`
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
