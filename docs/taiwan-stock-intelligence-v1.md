# Taiwan Stock Intelligence Evidence v1

`taiwan_stock_intelligence_v1` 是單一 canonical 台灣證券的 deterministic evidence contract。它回答「目前可靠知道什麼」，不提供分數、排名、預測、買賣建議、部位、停損、停利、目標價、AI 或新聞合成。

## Identity 與支援類型

API 只接受既有 Security Directory 的 `{code}.TWSE` 或 `{code}.TPEX`，不猜交易所、不做名稱或 Yahoo ticker inference。普通股可取得所有已驗證 section；ETF（含槓桿與反向）可保留 identity、quote、KLine、法人及融資券原始 evidence，但 fundamentals 與 ordinary-stock industry context 為 `not_applicable`/`unavailable`。未知類型 fail closed。

Identity 的產業 evidence 來自 M3，保留 exchange-qualified ID、官方代碼、名稱與 taxonomy source。分類基準是 `current_reference`；unclassified 不猜測。

## Sections 與 provenance

Bundle 包含 identity、Quote、20-bar price history、Taiwan fundamentals、institutional、margin、所屬 exchange 的 M2A/M2B market context，以及 M3 industry context。每個 section 分別保存 status、freshness、trade date/as-of、source、URL、fetched-at、fallback metadata 或 unavailable reason；沒有單一 overall current 狀態掩蓋差異。

Quote 的 official 不代表 current；trade date 晚於 `TaiwanTradingCalendar` latest completed target 時保留 raw observation，但 freshness 為 `unavailable`，monthly fallback 仍保留 `fallback_reason`。KLine 會先排除 target 之後的未完成 bar；N-session return 定義為 latest completed close / N sessions ago close - 1，因此 5d 需要 6 個 completed closes、20d 需要 21 個。資料不足回 `null`。Institutional 與 margin 的正、負、零值原樣保留；missing 不改成零。

Fundamentals 原樣聚合既有 capabilities、monthly revenue、statement、valuation 與 dividend evidence。既有 PIT 選擇規則仍為 `query_at > available_at`，相等不可用；各 section 不被包裝成同一時間點 fully PIT-safe。

## Market、industry 與 request budget

股票使用所屬 exchange context：TWSE 股票取 TWSE，TPEx 股票取 TPEX，不以 COMBINED 覆蓋。Canonical identity 只要求所屬 exchange directory，無關交易所失敗不阻斷解析。Industry facts 直接使用 M3 source identity。一次 evidence request 以同一份 directory 與一輪 bulk daily snapshot（TWSE、TPEx 各一次）同時計算 breadth、emotion、industry，避免重複三次全市場下載；單股 Quote、KLine、fundamentals、institutional、margin 仍依各自官方 provider request。沒有全市場逐檔 Quote/KLine sweep，也不依賴 Tencent、EastMoney、Kaipanla、Duanxianxia、Hermes 或 A-share decision engine。

## PIT 與已知限制

本版只提供 current/latest-completed evidence，沒有 historical intelligence endpoint。Security Master 與 industry 為 `current_reference`，歷史解讀可能受分類變更、上市櫃轉換及 survivorship drift 影響。Price benchmark/excess return、technical indicators、news/research、interpretation、scoring、prediction、recommendation 與 AI 均不在 v1。
