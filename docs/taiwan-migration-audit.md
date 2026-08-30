# easy-stock 台股化遷移稽核

## 1. 稽核基準與結論

- 稽核基準：`upstream/main`，commit `b969d05984dda736de9ee3dc2a881e8c65a373c5`（2026-08-27，v0.9.2 release preparation）。
- 稽核範圍：354 個版本控管檔案；涵蓋 `backend`、`frontend`、`desktop`、`docs`、`scripts`、`integrations`、SQLite、Provider、API routes、domain types、symbol、cache、Prompt、Hermes、methodology/skills、tests 與 build/release workflow。
- 結論：應保留 upstream 的 Go/React/Electron/Hermes/local-first 骨架，採 **CN 相容 + TW 擴充**，不應建立 Taiwan-only rewrite。
- Phase 1 最小切入點：在 `foundation` 之上加入市場 Adapter/Registry，以市場自行負責 symbol normalization；新增 TWSE/TPEx 的官方證券目錄 Provider，再以目錄服務完成台股搜尋與辨識。
- 本稽核不把外部資料缺失轉成零，也不把測試 fixture 當正式資料。

## 2. Repository 真實架構

| 層 | 真實位置 | 職責 | 台股化判斷 |
|---|---|---|---|
| Desktop | `desktop/*.cjs`, `desktop/scripts`, `desktop/test` | Electron lifecycle、後端程序、Hermes runtime、瀏覽器登入、更新與資料保護 | 大致保留；雪球、淘股吧、微信整合屬 CN research source，Phase 1 不動 |
| Frontend | `frontend/src/App.tsx`, `components`, `lib` | React 工作台與 REST/WebSocket client | Phase 1 不重寫；台股搜尋可沿用既有 directory response shape |
| HTTP 編排 | `backend/internal/httpapi` | Routes、驗證、fallback、SQLite-backed workflow | 保留；新增台股目錄 handler 只需接現有 stock directory seam |
| 通用資料模型 | `backend/internal/foundation` | Quote、KLine、source metadata、market overview models、symbol | 多數可保留；`symbol.go` 是最主要 A 股硬耦合點 |
| 市場 Provider | `backend/internal/providers/*` | EastMoney、Sina、Tencent、CLS、Duanxianxia 等 provider | 目前全為 CN/跨市場來源；新增 `twse`、`tpex`，不污染既有 provider |
| Provider 組合 | `backend/internal/providers/marketoverview` | 介面切分與 primary/fallback | 可保留，但它不是全域 capability registry |
| 領域服務 | `sector`, `stockanalysis`, `marketemotion`, `strategy/inflection` | A 股題材、漲停、個股評分、情緒與策略 | 高度 CN 語意；Phase 1 不直接搬到台股 |
| Research/AI | `review`, `methodology`, `narrative`, `hermes` | 文章、摘要、知識庫、Prompt、vendor-neutral AI runtime | Hermes 保留；Prompt/內建游資材料延至 Phase 6/7 分市場處理 |
| Persistence | 各 store 內嵌 `CREATE TABLE` | review、emotion、theme radar、portfolio inspection 等 SQLite | local-first 與 schema migration pattern 保留；Phase 1 不大改資料庫 |
| Build/Test | root workspaces、Go tests、Vitest、Node test、GitHub Actions | Go/TS/Electron build、test、release | 保留並作相容性門檻 |

### 與原工程規格假設不一致之處

1. Repository 沒有單一通用 `Provider Registry`。現況是各 route 注入具體 client，另有 `marketoverview.Provider` 組合 primary/fallback。
2. Repository 已有 `foundation.Symbol`，但它同時承擔 canonical、Sina 與 EastMoney 格式，且 `NormalizeSymbol` 只接受 SH/SZ/BJ；不適合直接塞入台股分支判斷。
3. SQLite 沒有中央 schema 目錄；schema 分散在各 store 建構函式。Phase 1 無需為 symbol directory 新增資料庫。
4. `StockCatalogEntry`、stock directory 與多個 UI type 帶有 A-share/EastMoney 語意，但 response shape 本身可重用。
5. Hermes 已保持 vendor neutral；不需要重寫，LM Studio/OpenAI-compatible 也屬既有設定能力。
6. 最新版已含市場總覽與多來源 fallback，但主要內容仍以 A 股為核心；不能把它誤認為通用 market adapter。

## 3. A 股耦合搜尋摘要

稽核排除了 `backend/internal/methodology/builtin/documents/**` 的大量內建文章正文，以免文章語料淹沒程式耦合；該目錄仍被分類為 CN-only knowledge content。

| 關鍵字/假設 | 命中文件數 | 主要位置 | 判斷 |
|---|---:|---|---|
| A股 | 15 | README、frontend、review、stockanalysis、narrative | UI/Prompt/產品定位需分市場 |
| 漲停 | 31 | limit-up workspace、foundation types、stockanalysis、emotion、providers | 模型可抽象，現有算法為 CN-only |
| 連板 | 21 | short-term、stockanalysis、review、frontend | D：Phase 1 隱藏/不接 TW，不機械翻譯 |
| 游資 | 22 | methodology、mastery、Prompt、frontend | D：保留 CN，台股研究來源另建模型 |
| 雪球 / 淘股吧 | 22 / 21 | desktop browser bridge、review automation、settings | D：保留 CN integration，不作 TW 核心 |
| 開盤啦 / 短線俠 | 20 / 3 | duanxianxia、sector radar、stock analysis | D：A 股 provider，不給 TW 使用 |
| 龍虎榜 | 10 | EastMoney、market overview、UI | D：台股不直接映射 |
| 上證 / 深證 / 創業板 / 科創板 | 9 / 5 / 8 / 1 | index catalog、stock catalog、analysis benchmark | C：TW 改用 TAIEX/TPEx，但保留 CN catalog |
| `.SH/.SZ/.BJ`, `secid` | 多處 | foundation、EastMoney/Sina、tests | B/C：canonical 與 provider symbol 必須拆開 |
| `Asia/Shanghai` | sector/review/provider 時間判斷 | scheduler、snapshot、trading date | C：TW 使用 `Asia/Taipei` 與官方休市資料 |
| CNY/人民幣 | models/docs/UI | fundamentals、display | C：TW 為 TWD；Phase 1 symbol metadata 即固定 TWD |

## 4. 耦合點分類與處置矩陣

| 分類 | 原功能 | 程式位置 | 目前依賴 / A 股假設 | 台股版本處理 | 動作 | 風險 | 優先級 |
|---|---|---|---|---|---|---|---|
| A | Electron lifecycle | `desktop/main.cjs`, `backend-process.cjs` | 無市場假設 | 原樣沿用 | 保留 | 低 | P3 |
| A | Hermes runtime/vendor gateway | `backend/internal/hermes`, `desktop/hermes-*` | Prompt 有 A 股語意，runtime 無 | runtime 保留，Prompt Phase 6 分市場 | 保留 | 低 | P3 |
| A | HTTP/Auth/WebSocket | `backend/internal/httpapi/auth.go`, `stream.go` | symbol parser 間接為 CN | transport 保留，TW route 使用 adapter | 保留 | 中 | P1 |
| A | Quote/KLine/SourceMeta | `foundation/types.go` | 部分註解/欄位假設 CNY | 結構保留；補強 market/exchange/currency 與證據欄位另期處理 | 保留/窄改 | 中 | P1 |
| A | SQLite/store migration | `review/store.go`, `marketemotion/store.go`, `portfolioinspection/store.go`, `duanxianxia/store.go` | 個別 schema 有 CN domain 欄位 | 保留 migration pattern；不共用 CN table 給 TW 假資料 | 保留 | 中 | P2 |
| A | Deterministic calculations | `stockanalysis/engine.go`, `portfolioinspection/metrics.go` | 部分門檻/benchmark 是 CN | 通用數學保留，市場規則由 adapter 提供 | 抽象後保留 | 高 | P2 |
| B | Symbol normalization | `foundation/symbol.go` 及所有 caller | 5/6 位數、首碼推斷 SH/SZ/BJ、內含 Sina/EastMoney 格式 | 市場 adapter 負責 normalize；TW canonical=`code.EXCHANGE` | 抽象 | 高 | P0 |
| B | Provider selection/fallback | `providers/marketoverview/provider.go`, `httpapi/server.go` | 具體 EastMoney/Tencent/Sina client | capability registry；不同能力排序，不宣稱 unsupported 為資料 | 抽象 | 高 | P0 |
| B | Stock directory/search | `httpapi/stock_directory.go`, `PortfolioSetupForm.tsx` | EastMoney A-share catalog、6 位碼顯示 | 聚合官方 TWSE/TPEx catalog；共用搜尋 response | 抽象 | 中 | P0 |
| B | Market/index model | `foundation/market_overview.go`, provider index catalogs | CN indices 為 core/benchmark | 新增 TAIEX/TPEx 目錄，Phase 2 才接行情 | 抽象 | 中 | P1 |
| B | Limit rule | `stockanalysis.limitUpThreshold`, `theme_screen.stockLimitRegime`, `sector.nearLimitUpThreshold` | 10/20/30% 與板別首碼 | Adapter 提供規則；TW 第一階段不啟用計分 | 抽象 | 高 | P2 |
| B | Trading calendar/time | `sector/radar.go`, review/duanxianxia 時間函式 | `Asia/Shanghai`、工作日近似交易日 | `TaiwanTradingCalendar` 使用 Asia/Taipei；休市日由官方來源更新 | 抽象 | 高 | P1 |
| B | Research source | `review`, desktop bridges | 平台名稱等於資料模型 | 建立 official/news/analyst/blog/youtube/podcast/user 類型 | 抽象 | 中 | P7 |
| C | 上市/上櫃/ETF 識別 | 無通用 TW 模型 | A 股 market suffix | TWSE/TPEx 官方目錄決定 exchange/type，不靠代碼猜交易所 | 新增 | 高 | P0 |
| C | TWSE/TPEx directory | 無 | EastMoney catalog 唯一股票目錄 | 官方 OpenAPI provider，timeout、source URL、error state | 新增 | 高 | P0 |
| C | TAIEX/TPEx | EastMoney catalog 僅有 Taiwan weighted | 缺 TPEx core index | Phase 2 取得行情；Phase 1 僅定義 index identity | 新增 | 中 | P1 |
| C | 三大法人 | 無台股模型 | CN 主力/席位不能替代 | Phase 3 新模型與官方來源 | 替換 | 高 | P3 |
| C | 融資融券 | `MarketMarginPoint` 為滬深北彙總 | CN margin semantics | Phase 3 依 TWSE/TPEx 個股資料建立新模型 | 替換 | 高 | P3 |
| C | 台股基本面 | EastMoney F10 | CNY、CN reports | Phase 4 使用官方/MOPS/FinMind，保存 available/published time | 替換 | 高 | P4 |
| C | 台股產業/題材 | `sector/*` local CN taxonomy + EastMoney | 開盤啦/EastMoney membership | Phase 5 建 verified/inferred/user_defined/ai_generated 關係 | 替換 | 高 | P5 |
| D | 連板梯隊/晉級/炸板 | `LimitUpWorkspace`, `limit_up_ladder`, `marketemotion` | A 股短線制度與語言 | TW 不接此 pipeline；Phase 5 顯示市場原始廣度指標 | 停用於 TW | 高 | P5 |
| D | 龍虎榜/游資心法 | EastMoney billboard、methodology/mastery | A 股席位資料 | 保留 CN；TW 不宣稱有等價能力 | 不搬 | 中 | P6/7 |
| D | 雪球/淘股吧/微信復盤 | desktop/review automation | CN 平台登入與文章 | 保留 CN；TW ResearchSource 後續另接 | 不搬 | 中 | P7 |

## 5. Symbol 與市場邊界決策

### Canonical identity

Phase 1 採 `CODE.EXCHANGE`：

- `2330.TWSE`
- `6488.TPEX`
- `0050.TWSE`

理由：裸碼 `2330` 無法單獨證明 exchange；官方目錄是權威解析來源。provider-specific symbol 不存入 canonical identity。既有 CN canonical (`600000.SH`) 保持不變。

### 類型

`security_type` 第一階段只使用 `stock`、`etf`、`index`、`unknown`。公司基本資料 Provider 能可靠識別股票；ETF 由官方基金/商品目錄補充。無來源時不得以名稱或價格猜測。

### Provider capability

Phase 1 僅宣告並實作 `search_symbol` / `security_directory`。`quote`、`daily_bars`、`institutional_flows`、`margin`、`fundamentals` 若未實作應回報 unsupported，而非空資料成功。

## 6. 官方來源核實

- TWSE OpenAPI base：`https://openapi.twse.com.tw/v1`；上市公司基本資料：`/opendata/t187ap03_L`。
- TPEx OpenAPI base：`https://www.tpex.org.tw/openapi/v1`；上櫃股票基本資料：`/mopsfin_t187ap03_O`。
- 兩者皆為官方 OpenAPI；Phase 1 只使用目錄能力。行情、K 線、法人、融資融券屬後續 Phase，不在這輪偽裝完成。
- TWSE 官方基金基本資料 `/opendata/t187ap47_L` 已 live 核實涵蓋 0050、0056、00878；ETF 不需要 bootstrap 或硬編目錄。

## 7. Phase 1 修改邊界與風險控制

1. 不改 CN provider 行為或既有 `NormalizeSymbol` 對 CN 的輸出。
2. 新增市場 registry，而非在數百處加入 `if market == "TW"`。
3. 官方 HTTP provider 使用既有 `net/http`，不新增依賴；設 timeout、限制 response size、檢查 status、拒絕空目錄。
4. 解析測試全部使用固定 fixture；live smoke test 明確區分網路、HTTP、schema、not found。
5. Phase 1 不修改 AI Prompt、短線情緒、題材、法人、融資融券、報價、K 線或主 UI。
6. 上游合併風險主要集中在 `foundation/symbol.go` 與 server wiring；透過新增檔案與最小 caller 變更降低衝突。

## 8. Phase 0 驗收

- [x] 掃描 repository 與 build/test manifest。
- [x] 盤點 A 股耦合詞、symbol、timezone、currency、provider、Prompt、AI/Hermes、SQLite、UI、tests。
- [x] 分類保留、抽象、替換、不搬移項目。
- [x] 以真實架構修正原規格假設。
- [x] 定義 Phase 1 最小、可測試、可回退的實作邊界。

## 9. Phase 1 已知限制

- 本輪只有 security identity、官方 directory、search、cache 與 market adapter；quote、K-line、index、法人、融資融券、基本面、台股 Prompt/UI 都仍是 `unsupported` 或未接入，不得解讀為已完成。
- 台股 API 使用獨立 `/api/v1/tw/securities`；既有 `/api/v1/stocks/directory` 與 CN quote/analysis path 保持不變，避免 TW identity 被送入只支援 SH/SZ/BJ 的 provider。
- 目錄 cache 為 process memory，重啟後重新抓取；上游失敗時只在同一 process 已有成功 snapshot 的情況回傳 `stale=true`。
- TWSE/TPEx 產業代碼目前保留官方原值，尚未在 Phase 1 建立中文產業 taxonomy。
- 上市日期目前保留官方民國/西元原始字串，尚未跨 provider 轉換；不可用它直接做歷史計算。
- Windows 全庫 Go tests 有兩個 upstream 非 Phase 1 失敗：Unix mode `0600` 斷言，以及 theme-radar SQLite cleanup file lock。Desktop tests 有四個 upstream Windows/path/process mode 失敗。Phase 1 定向 tests、backend build 與 frontend tests/build 另行驗證。
- npm audit 於既有 lockfile 回報 1 low、4 high；本輪不執行會改 dependency graph/lockfile 的自動修復。
