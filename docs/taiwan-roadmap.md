# easy-stock-TW Roadmap

本路線圖以資料正確、來源可追溯、可測試及 upstream mergeability 為優先。每一階段必須在前一階段驗證完成後才開始；資料不可得時回報明確狀態，不產生假資料。

## Phase 0 — Audit

**範圍**

- 掃描真實架構與 A 股耦合。
- 決定保留、抽象、替換與不搬移項目。
- 核實官方資料入口與 Phase 1 邊界。

**交付**

- `docs/taiwan-migration-audit.md`
- `docs/taiwan-roadmap.md`

**狀態：本輪完成。**

## Phase 1 — Market Foundation

**範圍**

- Market Adapter 與 capability registry。
- CN 相容的 symbol normalization 邊界。
- Taiwan canonical symbol、market、exchange、security type、currency、timezone。
- TWSE/TPEx 官方證券目錄 Provider。
- 台股代碼/名稱搜尋與 2330、2317、2454、6488、0050 smoke tests。

**驗收**

- 同一標的不會以裸碼/provider code/canonical 重複建檔。
- 2330/台積電、2317/鴻海、2454/聯發科、6488/環球晶、0050 可得到名稱、代碼、上市/上櫃/ETF、TWD、TW。
- provider failure/unsupported 不是空成功或零值假資料。
- 原 CN tests、frontend build、desktop tests 仍通過。

**狀態：本輪實作；完成後停止，不自動進入 Phase 2。**

## Phase 2 — Quote & K-Line

- Quote、daily bars、TAIEX、TPEx index。
- Quote 使用 TWSE/TPEx OpenAPI，失敗時降級至同一官方交易所的月成交端點。
- 每筆資料保留 source URL、trade date、stale、fallback、status 與 `is_realtime=false`。
- 台股搜尋、收盤行情、日 K 與兩市場指數最小 UI 接入。
- 以官方資料交叉驗證 2330、2317、2454、6488、0050。

**狀態：本輪完成。**

**已知限制**

- 官方端點提供的是收盤資料，不宣稱盤中即時行情。
- 日 K 以官方月資料逐月讀取，單次最多 240 根；目前沒有引入 FinMind token 或非官方 Yahoo 資料。
- provider fallback 是官方當日 OpenAPI → 官方月成交資料；若兩者都失敗，回傳明確錯誤，不補零值或硬編行情。
- HTTP 層沿用證券目錄 12 小時 cache；行情本身不做長時間持久化，避免把舊收盤價誤標成即時資料。

## Phase 3 — Taiwan Chip Data

- 外資及陸資、投信、自營商 buy/sell/net。
- 連買賣、5/20 日累積與成交量占比，防 look-ahead bias。
- 融資餘額/增減、融券餘額/增減、券資比。

## Phase 4 — Fundamentals

- 月營收、EPS、PE、dividend 優先。
- 後續補 revenue、margin、ROE、PB。
- 保存 event_date、published_at、available_at、retrieved_at。
- MOPS 不穩定時只建立明確 unsupported 的 provider boundary，不寫脆弱假爬蟲。

## Phase 5 — Market Intelligence

- TaiwanMarketSnapshot：TAIEX、TPEx、turnover、advance/decline/flat、limit counts、法人、融資變化。
- Sector/Industry/Theme/SupplyChain 關係與 verified/inferred/user_defined/ai_generated 狀態。
- 先顯示可回溯原始指標，不發布未回測的神秘 0–100 分數。

## Phase 6 — AI Research

- Taiwan-specific stock research context 與 Prompt。
- 趨勢成長、法人驅動、題材動能、景氣循環、高股息/防禦、事件驅動、震盪觀察、弱勢風險。
- Bull/Base/Bear 條件、反證與失效條件。
- Portfolio concentration/industry/theme exposure；Hermes runtime 保持 vendor neutral。

## Phase 7 — Research Sources

- ResearchSource：official/news/analyst/blog/youtube/podcast/user。
- MOPS、新聞、研究文章與 Research Agent 分批接入。
- 摘要分離 Fact、Opinion、Prediction、Evidence、Risk、WatchCondition。

## Phase 8 — Historical Validation

- 每日 snapshot、研究假設與後驗驗證。
- 本地研究記憶與相同股票的前次假設比較。
- backtesting-ready availability timestamps，禁止 look-ahead bias。

## 全階段共同門檻

- 官方資料優先；非正式來源需標示 unofficial/experimental。
- production 禁止 mock/fake/placeholder 市場資料。
- API key 不 hardcode、不 commit、不寫入 log。
- 每一 Provider 都有 fixture parser test、timeout、source metadata 與 failure state。
- 不刪除 `LICENSE`、作者資訊、Git 歷史或 CN 能力。
- 每階段執行 Go tests、frontend tests/build、desktop tests/build；沒有的檢查不假裝存在。
