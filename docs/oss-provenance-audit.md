# OSS Provenance Audit

本文件記錄 mystocktracer 的程式碼來源盤點結果，以及未來若要切換到 OSI 相容授權所需的替換計畫。

- 稽核基準 commit：`ec4d5c5` (`main`, 2026-09-18)
- 上游分叉點：`b969d05`（jundizhou, 2026-08-27，`chore: prepare release v0.9.2`）
- 稽核方法：以 Git history 為證據，逐檔比對檔案是否存在於分叉點的 tree，再檢查分叉後的修改範圍與 runtime 相依關係。
- 本文件是工程稽核，不是法律意見。

---

## Current licensing situation

| 項目 | 內容 |
| --- | --- |
| 目前 root license | PolyForm Noncommercial License 1.0.0，著作權標示 `Copyright (c) 2026 jundizhou` |
| 上游專案 | [jundizhou/easy-stock](https://github.com/jundizhou/easy-stock) |
| 衍生關係 | 本 repository 是 easy-stock 的衍生作品，完整保留上游 Git 歷史（188 commits 中 92 個來自上游作者） |
| 目前 HEAD 的性質 | source-available，非 OSI 開源 |

### 為什麼還不能假設可以 relicensing

1. **著作權人不是本專案維護者。** `LICENSE` 的著作權人為 jundizhou。PolyForm Noncommercial 授權下游取得的是使用與修改的權利，**不包含**變更授權條款的權利。
2. **上游程式碼仍大量存在於 HEAD。** 分叉點的 373 個追蹤檔案中，370 個仍存在於目前 tree（3 個已刪除）。
3. **台股功能建立在繼承的框架上。** 新寫的台股模組是原創，但它們在繼承的 HTTP server、設定層、AI runtime、桌面 shell 與 Go module 之內執行（詳見 [Inherited code inventory](#inherited-code-inventory)）。
4. **「改過的檔案」不等於「原創檔案」。** 49 個繼承檔案在分叉後被修改，但其結構與大部分內容仍源自上游，provenance 仍為 inherited。
5. **套件內含 AGPL-3.0-only 第三方元件**（見 [Third-party bundled components](#third-party-bundled-components)）。

---

## Provenance categories

本文件使用下列 evidence-based 分類：

| 分類 | 定義 | 判定依據 |
| --- | --- | --- |
| `confirmed-inherited` | 檔案存在於上游分叉點 `b969d05`，且分叉後未被修改 | `git ls-tree b969d05` ∩ 目前 tree，且不在 `git diff b969d05..HEAD` |
| `likely-inherited` | 檔案存在於分叉點，分叉後有修改，但結構與主要內容仍來自上游 | 同上 ∩ diff 名單，並人工檢視修改範圍 |
| `original` | 檔案在分叉後才建立，由 nanachi1212 撰寫 | 不存在於 `b969d05` tree |
| `third-party-permissive` | 明確授權的外部相依或素材 | 見依賴清單與 notice 檔 |
| `unclear` | 無法從 Git 證據判定原始出處 | 本輪標註於下方 |

> 分叉後的所有 commit 作者皆為 `nanachi1212`，但這只證明**誰提交**，不證明**內容原創性**；`original` 標記僅用於分叉後新建、且內容為台股領域自行實作的檔案。

---

## 統計概要

| 指標 | 數量 |
| --- | --- |
| 目前追蹤檔案總數 | 501 |
| 分叉點檔案數 | 373 |
| **仍存在的繼承檔案** | **370** |
| ├ `confirmed-inherited`（分叉後未修改） | 321 |
| └ `likely-inherited`（分叉後有修改） | 49 |
| 分叉後新建檔案 | 131 |
| 分叉後刪除的上游檔案 | 3（`desktop/wechat-service.cjs` 及其 script／test） |
| 繼承程式碼行數（`.go/.ts/.tsx/.cjs/.mjs/.css/.html`） | 約 66,700 行 |
| 原創程式碼行數（同副檔名） | 約 35,100 行 |

---

## Inherited code inventory

### Backend（Go）

Go module 路徑本身仍為 `easy-stock/backend`，因此 **192 個繼承檔案 + 80 個原創檔案全部都以 `easy-stock/backend/...` 匯入**。這是 identity 殘留，也是 Phase B 的機械性工作。

| 套件 | 繼承檔案 | 分類 | Taiwan runtime 相依 |
| --- | ---: | --- | --- |
| `internal/methodology` | 47 | 45 confirmed / 2 likely | 無（A 股游資心法資料庫） |
| `internal/providers`（cls / duanxianxia / eastmoney / hotstock / marketoverview / sina / tencent） | 40 | confirmed-inherited | 無（全為中國市場資料源） |
| `internal/httpapi` | 38 | 29 confirmed / 9 likely | **有**：`server.go`、`config.go`、`settings.go` 是所有台股路由的掛載點與設定層 |
| `internal/sector` | 14 | confirmed-inherited | 部分（`sector/taiwan.go` 為原創，與繼承的 sector 型別同套件） |
| `internal/review` | 12 | confirmed-inherited | 無（雪球／淘股吧／微信複盤） |
| `internal/portfolioinspection` | 12 | confirmed-inherited | 無（A 股持倉巡檢，與台股 `taiwanportfolio` 無關） |
| `internal/stockanalysis` | 9 | confirmed-inherited | 部分（台股 `taiwan*.go` 為原創，共用同套件的型別） |
| `internal/foundation` | 5 | 4 confirmed / 1 likely (`types.go`) | **有**：`types.go`、`symbol.go` 提供台股模組使用的基礎型別 |
| `internal/strategy` | 4 | confirmed-inherited | 無 |
| `internal/marketemotion` | 3 | confirmed-inherited | 部分（`marketemotion/taiwan.go` 為原創） |
| `internal/hermes` | 2 | likely-inherited | **有**：台股 AI Research 透過 `hermes.Runtime` 呼叫模型 |
| `internal/runtimelog` / `internal/narrative` / `internal/appsettings` | 6 | confirmed / likely | 有（設定與日誌） |
| `cmd/server` | 2 | likely-inherited | **有**：程式進入點 |
| `backend/docs` | 10 | 8 confirmed / 2 likely | 文件 |
| `go.mod` / `go.sum` | 2 | confirmed-inherited | **有**：module 名稱 |

### Frontend（React / TypeScript）

| 項目 | 繼承檔案 | 分類 | Taiwan runtime 相依 |
| --- | ---: | --- | --- |
| `src/App.tsx` | 1 | likely-inherited | **有**：應用外殼與導覽，台股 workspace 掛在其中 |
| `src/lib/backend.ts` | 1 | likely-inherited | **有**：所有台股 API 呼叫都經過 `requestJSON` |
| `src/styles.css` | 1 | likely-inherited | **有**：全站樣式 |
| `src/lib/hermes.ts`、`llm-providers.ts` | 2 | likely-inherited | **有**：AI Research 的模型連線層 |
| `src/components/`（AIChat、KLine、LimitUp、MarketOverview、PortfolioInspection、ReviewDiary、SettingsDrawer、StockAIAnalysis、TradingMastery 等） | 19 | confirmed / likely | 大多無（A 股畫面）；`SettingsDrawer`、`AppUpdatePanel`、`MarkdownContent` 為共用 |
| `src/lib/`（billboard、chat、kline、market-overview、portfolio-draft、sector-map、short-term、stock-analysis、runtime-log） | 23 | confirmed-inherited | 大多無 |
| `src/main.tsx`、`index.html`、`vite.config.ts`、`tsconfig.json`、`package.json` | 5 | confirmed / likely | **有**：建置與進入點 |
| `src/data/billboard-seat-mappings.*` | 2 | confirmed-inherited | 無 |
| `public/easy-stock-mark*.{png,svg}` | 2 | confirmed-inherited（素材） | 有（favicon） |

原創前端：26 個檔案，全部為 `Taiwan*` workspace、`taiwan-product.ts`、`subscription-ai.*` 及其測試。

### Desktop（Electron）

| 檔案 | 分類 | 說明 |
| --- | --- | --- |
| `main.cjs` | likely-inherited | 應用進入點；`app.setName('easy-stock')` 決定 userData 路徑 |
| `preload.cjs`、`backend-process.cjs`、`update-manager.cjs`、`update-feed.cjs`、`user-data.cjs`、`data-protection.cjs`、`runtime-logger.cjs`、`hermes-runtime-root.cjs` | confirmed / likely | 桌面 shell 基礎 |
| `browser-auth.cjs`、`xueqiu-*.cjs`、`taoguba-browser-bridge.cjs`、`review-login-preload.cjs` | confirmed-inherited | A 股網站登入橋接 |
| `scripts/`（16 個） | confirmed / likely | 封裝、簽章、更新中繼資料；`electron-builder.mjs` 內含 `appId: com.jundizhou.easystock`、`productName: easy-stock` |
| `test/`（15 個） | confirmed / likely | 桌面端測試 |
| `assets/easy-stock.{icns,ico,png,svg,iconset}` | confirmed-inherited（素材） | 應用圖示 |
| `subscription-ai.cjs` | original | 分叉後新建 |

### Scripts

| 檔案 | 分類 |
| --- | --- |
| `scripts/rebuild-restart.sh` | confirmed-inherited |
| `scripts/rebuild-restart.mjs`、`rebuild-restart.test.mjs` | original |

### Tests

測試檔案的 provenance 跟隨其對應模組：繼承模組的測試為 inherited（後端約 40 個、前端約 15 個、桌面 15 個），台股模組的測試為 original（後端約 30 個、前端 13 個）。

### Docs

| 檔案 | 分類 |
| --- | --- |
| `docs/user-guide.md`、`development.md`、`market-overview-plan.md`、`billboard-seat-mappings.md`、`daily-review-oss-sync.md` | confirmed / likely-inherited |
| `docs/taiwan-*.md`、`docs/TAIWAN_DEVELOPMENT_OWNERSHIP.md`、`docs/TOALPHA_MOPS_INTEGRATION_AUDIT.md`、本文件 | original |
| `backend/docs/*`（10 個） | confirmed / likely-inherited |
| `.github/release-notes/v0.1.0–v0.9.2`（13 個） | confirmed-inherited（上游發布紀錄，屬歷史文件，應保留） |

### Assets

| 項目 | 分類 | 說明 |
| --- | --- | --- |
| `desktop/assets/easy-stock.*`（5 個） | confirmed-inherited | 應用圖示與安裝檔圖示 |
| `frontend/public/easy-stock-mark.*`（2 個） | confirmed-inherited | 網頁 favicon |
| `docs/assets/easy-stock-*.{png,svg}`（13 個） | confirmed-inherited | 上游產品截圖與架構圖，內容為 A 股畫面 |

### Third-party bundled components

| 元件 | 授權 | 說明 |
| --- | --- | --- |
| `integrations/wechat-download-api`（`tmwgsicp/wechat-download-api`，pin 於 commit `043c2f9`） | **AGPL-3.0-only** | 桌面安裝包內建的微信公眾號下載服務。目前以獨立行程方式執行並附上游 LICENSE 與 notice。屬 A 股／複盤功能，台股產品不使用。 |
| npm / Go 相依套件 | 各自授權（多為 MIT / Apache-2.0 / BSD） | `third-party-permissive` |

---

## Original mystocktracer areas

下列項目在分叉點 tree 中不存在，為分叉後新建，且內容為台股領域自行實作。**但請注意：這些模組全部在繼承的 runtime 框架內執行**（見每項的「框架相依」欄）。

| 能力 | 主要檔案 | 分類 | 框架相依 |
| --- | --- | --- | --- |
| 官方台股資料 provider | `backend/internal/providers/taiwan/*`（19 檔） | original | 繼承的 `foundation` 型別 |
| 台股市場 contract / 規則 / 型別 | `backend/internal/foundation/taiwan_*.go`（13 檔） | original | 與繼承的 `foundation/types.go` 同套件 |
| 公司事件整合（ToAlpha / MOPS） | `backend/internal/providers/toalpha/*`（4 檔） | original | 繼承的 HTTP client 慣例 |
| 事件提醒中心 | `backend/internal/taiwanwatchlist/{alerts,events}.go`、`httpapi/taiwan_alerts.go`、`frontend/.../TaiwanEventAlertCenter.tsx` | original | 繼承的 `httpapi.Server` 路由 |
| AI Research | `backend/internal/stockanalysis/taiwan_research.go`、`taiwan_interpretation.go` | original | **繼承的 `internal/hermes` runtime** |
| 研究歷史與前次比較 | `stockanalysis/taiwan_research_history.go`、`taiwanwatchlist/research_history.go`、`httpapi/taiwan_research_history.go`、`TaiwanResearchHistory.tsx` | original | 繼承的路由層 |
| 台股自選股 | `backend/internal/taiwanwatchlist/store.go`、`httpapi/taiwan_watchlist.go`、`TaiwanWatchlistWorkspace.tsx` | original | 繼承的路由層 |
| 台股持倉 | `backend/internal/taiwanportfolio/store.go`、`httpapi/taiwan_portfolio.go`、`TaiwanPortfolioWorkspace.tsx` | original | 繼承的路由層 |
| 台股每日總覽 Dashboard | `backend/internal/httpapi/taiwan_dashboard.go`(+test)、`frontend/src/components/TaiwanDailyDashboard.tsx`(+test) | original | 繼承的 `httpapi` 套件、`foundation` 型別、`App.tsx` 外殼、`backend.ts` 請求層 |
| 台股市場廣度／情緒／產業雷達 | `providers/taiwan/breadth.go`、`marketemotion/taiwan.go`、`sector/taiwan.go`、`httpapi/taiwan_market.go` | original | 與繼承模組同套件 |
| 台股選股器 | `httpapi/taiwan_screener.go`、`TaiwanScreenerWorkspace.tsx` | original | 繼承的路由層 |
| market adapter 註冊機制 | `backend/internal/marketadapter/*`（4 檔） | original | 繼承的 `foundation` 型別 |
| Pull Request 品質 CI | `.github/workflows/pr-quality.yml` | original | 無 |

### Taiwan Dashboard provenance result

Taiwan Daily Dashboard（PR #6）的結論：

- **實作為 original**：`taiwan_dashboard.go`、`taiwan_dashboard_test.go`、`TaiwanDailyDashboard.tsx`、`TaiwanDailyDashboard.test.tsx` 四個檔案全部在分叉後新建，不存在上游對應物。
- **但無法獨立於繼承框架運作**：它匯入繼承的 `internal/foundation`（型別）、掛載於繼承的 `internal/httpapi` server、前端掛在繼承的 `App.tsx` 與 `lib/backend.ts` 之上，並在 module path `easy-stock/backend` 之內編譯。

因此 Dashboard 的 provenance 標記為 **original feature on inherited infrastructure**，不能單獨視為「可 relicensing 的乾淨區塊」。

---

## TWstock relationship

Repository：[`nanachi1212/TWstockfor_tick-stock-panel`](https://github.com/nanachi1212/TWstockfor_tick-stock-panel)，授權為 **MIT**（`Copyright (c) 2026 tickflow-stock-panel contributors`）。

| 項目 | 狀態 |
| --- | --- |
| 目前實際 reuse 的 TWstock 程式碼 | **無**。全文檢索 `TWstock` / `tick-stock-panel` 僅命中 `CLAUDE.md` 與 `docs/TAIWAN_DEVELOPMENT_OWNERSHIP.md` 的架構說明，沒有任何複製的原始碼、資料或素材。 |
| 目前需要保留的 MIT attribution | **無**（因為沒有實際 reuse）。 |
| 目前的關係 | **architecture reference only**：ownership 邊界、canonical domain 劃分、Point-in-Time 與 provenance 語意。 |

### 可作為未來 permissive replacement source 的區域

TWstock 為 MIT，未來若要以 permissive 程式碼替換繼承的 runtime，下列 domain 是合法且合適的來源方向（實際可用性需在 Phase B 逐項確認）：

- Security Master / 證券基本資料
- TWSE / TPEx / MOPS 官方資料擷取與正規化
- 日 OHLCV 歷史行情
- 三大法人、融資融券
- 月營收、財報、Point-in-Time fundamentals
- 估值、股利、股本相關正規化紀錄
- deterministic technical / fundamental factors
- Research Facts 層

**不適合**從 TWstock 取代的部分：桌面 shell、Electron 封裝、React 應用外殼、設定層、AI runtime——這些是 mystocktracer 的產品層責任，TWstock 沒有對應實作。

若未來實際拷入 TWstock 程式碼，必須：保留 MIT 著作權聲明與授權全文、在檔案或 `NOTICE` 中記錄來源路徑與 commit、不得標記為 mystocktracer 原創。

---

## OpenStock relationship

Repository：[`Open-Dev-Society/OpenStock`](https://github.com/Open-Dev-Society/OpenStock)

本專案對 OpenStock 的使用界線：

**允許**
- 產品設計參考
- 公開 repository 呈現方式參考
- UX 概念參考
- contributor experience 與文件組織方式的啟發

**禁止**
- 複製原始碼、React 元件、CSS、prompt、測試、文件文字或素材
- 將其標記為本專案的 source-code source

**目前狀態：本 repository 不含任何 OpenStock 來源的程式碼或內容**（全文檢索 `OpenStock` / `Open-Dev-Society` 零命中）。未來若要引入任何 OpenStock 實作，必須先做明確的授權相容性判斷，不得在本輪或後續順手加入。

---

## Relicensing blockers

下列項目必須先處理，才能考慮變更 root license。

### 必須取得著作權人同意（無法以工程手段解決）

| 項目 | 說明 |
| --- | --- |
| `LICENSE` 著作權人為 jundizhou | 只要 HEAD 仍含上游程式碼，變更授權需原作者書面同意。**這是本專案最上層的 blocker。** |

### 必須 replace（runtime 關鍵的繼承程式碼）

| 項目 | 說明 |
| --- | --- |
| `backend/internal/httpapi/server.go`、`config.go`、`settings.go`、`stream.go` | 所有台股路由的掛載點與設定層 |
| `backend/cmd/server/main.go` | 程式進入點 |
| `backend/go.mod` module path `easy-stock/backend` | 影響 230 個 Go 檔案的 import |
| `backend/internal/foundation/{types,symbol,market_overview,hot_stock}.go` | 台股模組使用的基礎型別 |
| `backend/internal/hermes/runtime.go` | 台股 AI Research 的模型 runtime |
| `backend/internal/appsettings/store.go` | 設定持久化 |
| `frontend/src/App.tsx`、`lib/backend.ts`、`styles.css`、`main.tsx`、`index.html` | 前端外殼、請求層與全站樣式 |
| `frontend/src/lib/hermes.ts`、`llm-providers.ts` | AI 連線層 |
| `desktop/*.cjs`、`desktop/scripts/*` | 桌面 shell 與封裝流程 |

### 必須 remove 或 replace（非 runtime 關鍵）

| 項目 | 說明 |
| --- | --- |
| 舊版 A 股後端模組（`methodology`、`review`、`portfolioinspection`、`strategy`、中國 providers） | 約 114 個繼承檔案 |
| 舊版 A 股前端畫面 | 約 38 個繼承檔案 |
| 繼承素材（圖示、favicon、產品截圖，共 20 個） | 需原創替換 |
| 繼承文件（`docs/` 5 個、`backend/docs/` 10 個） | 需重寫或移除 |
| `integrations/wechat-download-api`（AGPL-3.0-only） | 若未來採 permissive 授權，內建 AGPL 元件需重新評估散布方式 |

### 必須 investigate

| 項目 | 需確認什麼 |
| --- | --- |
| **桌面更新來源 `easy-stock-fs.oss-cn-beijing.aliyuncs.com`** | `desktop/update-feed.cjs` 與 `.github/workflows/release.yml` 目前指向上游控制的 Alibaba OSS bucket。本 repository 的桌面版會從該來源取得更新。這同時是 identity 與**供應鏈**議題，需要維護者決定自有更新來源後才能變更；本輪不動以免破壞既有安裝的更新路徑。 |
| `.github/release-notes/v0.1.0–v0.9.2` | 上游發布紀錄，屬歷史文件，建議保留為 attribution 的一部分；但需確認是否算 relicensing 範圍 |
| `docs/billboard-seat-mappings.md` 與 `frontend/src/data/billboard-seat-mappings.ts` | 內含市場席位對照資料，原始資料出處未在 repository 中說明 → `unclear` |
| `backend/internal/methodology` 快取的「游資心法」內容 | 來源為外部 `trading-mastery` 資料，原始授權未記錄 → `unclear` |
| `desktop/assets/easy-stock.svg` 等圖示的原始設計來源 | repository 中無設計出處記錄 → `unclear` |

---

## Legacy A-share inventory

判斷依據：(1) 台股產品是否仍使用；(2) 是否為繼承程式碼；(3) 是否仍有 runtime 相依；(4) 移除是否破壞 build；(5) 未來是否需要原創替換。

| 模組 | 繼承 | 台股使用 | Runtime 相依 | 移除影響 build | 處置 |
| --- | --- | --- | --- | --- | --- |
| `backend/internal/portfolioinspection`（12 檔） | 是 | 否 | `httpapi/portfolio_inspection.go` 路由 | 需一併移除路由與前端畫面 | **remove**（Phase B） |
| `backend/internal/methodology`（47 檔） | 是 | 否 | `httpapi` 路由 + Hermes skill 安裝 | 需一併移除 | **remove**（Phase B，先釐清快取內容授權） |
| `backend/internal/review`（12 檔） | 是 | 否 | `httpapi/review_*.go`、桌面登入橋接 | 需一併移除桌面 bridge | **remove**（Phase B） |
| `backend/internal/strategy`（4 檔） | 是 | 否 | `httpapi/strategy` 路由 | 需一併移除 | **remove**（Phase B） |
| 中國 providers：`eastmoney`(14)、`duanxianxia`(11)、`sina`(4)、`tencent`(4)、`cls`(2)、`hotstock`(2)、`marketoverview`(2) | 是 | 否 | `httpapi/market_*.go`、`stock_hot.go`、`theme_screen.go`、`limit_up_ladder.go` | 需一併移除對應路由 | **remove**（Phase B） |
| A 股 HTTP routes（`/api/v1/market`、`/quotes`、`/short-term`、`/themes`、`/reviews`、`/portfolio-inspections`、`/strategy`、`/stocks/hot-ranks`、`/sector-map` 等 33 條非 `/tw/` 路由中的 A 股部分） | 是 | 否 | 目前仍註冊 | 移除需同步前端 | **remove**（Phase B） |
| 共用 routes（`/api/v1/settings*`、`/api/v1/ai`、`/api/v1/ws`、`/api/v1/sources`、`/api/v1/stocks/directory`） | 是 | **是** | 台股設定與 AI 均使用 | 不可移除 | **replace**（Phase B，原創重寫） |
| A 股前端畫面（`LimitUpWorkspace`、`MarketOverviewWorkspace`、`PortfolioInspection*`、`ReviewDiary`、`TradingMastery`、`StockAIAnalysisWorkspace`、`AIChatWorkspace`、`market/MarketDataViews` 等，約 15 個元件 + 23 個 lib） | 是 | 否 | `App.tsx` 仍以 hash 路由掛載（`#limit-up`、`#reviews`、`#mastery`、`#stock-ai`、`#portfolio-inspection`、`#themes`、`#market`、`#ai`），台股導覽不顯示 | 需同步移除 `App.tsx` 分支 | **remove**（Phase B） |
| A 股 prompts（`portfolioinspection/prompt.go`、`expectation_prompt.go`、`review_diary.go` 去識別化 prompt、`hermes/runtime.go` system prompt 中的 A 股段落） | 是 | 部分（Hermes system prompt 為共用） | 有 | — | **replace**（system prompt）／**remove**（其餘） |
| A 股相關設定（`A_STOCK_*` 環境變數共 20+ 個、`A_STOCK_ADDR`、各資料庫路徑） | 是 | **是**（台股也用同一組前綴） | 有 | 變更會破壞既有安裝的設定與資料路徑 | **investigate**（Phase B，需設計相容遷移） |
| 桌面 A 股登入橋接（`xueqiu-*`、`taoguba-*`、`review-login-preload.cjs`、`browser-auth.cjs`） | 是 | 否 | `main.cjs` 引用 | 需同步移除 | **remove**（Phase B） |
| `integrations/wechat-download-api`（AGPL-3.0-only） | 第三方 | 否 | 桌面封裝 | 需移除封裝步驟 | **remove**（Phase B） |
| `.github/workflows/release.yml` 的 OSS 發布步驟 | 是 | 是（桌面更新） | 有 | — | **investigate**（需自有更新來源） |
| `.github/release-notes/v0.1.0–v0.9.2`（13 檔） | 是 | 否 | 無 | 否 | **keep temporarily**（歷史 attribution） |
| `docs/assets/easy-stock-*`（13 個 A 股截圖） | 是 | 否（README 已不引用） | 無 | 否 | **remove**（Phase B，確認無引用後） |

> 本輪**不執行**任何上述刪除。理由：這些模組目前仍有 runtime 路由註冊與測試覆蓋，大規模刪除會產生無法在單一 PR 內安全審查的 diff，且與本輪的 repository hygiene 目標無關。

---

## Phase B replacement plan

**目標問題：** 如果希望未來 mystocktracer HEAD 可以採 OSI 相容授權，還有哪些繼承自 easy-stock 的部分必須移除或原創重寫？

> **前置條件：** 即使下列工作全部完成，是否可以變更授權仍取決於著作權人同意，或確認 HEAD 已不含任何上游受著作權保護的表達。工程替換是必要條件，不是充分條件。

### B1. Runtime-critical inherited backend

| # | Path / module | Provenance | 目前用途 | Taiwan 相依 | 處置 | 替換方式 | TWstock MIT 可替換 | 難度 | 相依順序 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| B1-1 | `backend/go.mod` module path | confirmed-inherited | Go module 名稱 | 全部 | replace | 改為 `mystocktracer/backend`，同步 272 檔 import | 否 | 低（機械性，但 diff 大） | 最先做，其餘後端工作都受益 |
| B1-2 | `backend/cmd/server/main.go` | likely-inherited | 進入點、設定載入、userData 路徑 | 有 | replace | 原創重寫（約 200 行） | 否 | 低 | B1-1 之後 |
| B1-3 | `backend/internal/httpapi/server.go` | likely-inherited | 路由註冊、middleware、CORS | 有 | replace | 拆成 `twserver` 原創 router，只掛台股路由 | 否 | 中 | B1-1 之後，與 B1-5 併行 |
| B1-4 | `httpapi/config.go`、`settings.go`、`settings_agent.go`、`stream.go` | likely-inherited | 設定 API、SSE | 有 | replace | 原創重寫，同時處理 `A_STOCK_*` 前綴遷移 | 否 | 中 | B1-3 之後 |
| B1-5 | `foundation/{types,symbol,market_overview,hot_stock}.go` | confirmed / likely | 基礎型別 | 有 | replace | 台股所需部分原創重寫；A 股型別隨模組移除 | **部分可**（Security Master、symbol 正規化） | 中 | 與 B1-3 併行 |
| B1-6 | `backend/internal/hermes/runtime.go`(+test) | likely-inherited | AI 模型 runtime（1,418 行） | **有**（AI Research 唯一路徑） | replace | 原創重寫模型呼叫層，或改用標準 SDK | 否（TWstock 無對應） | **高** | B1-3 之後；Phase B 最大單項 |
| B1-7 | `internal/appsettings/store.go` | likely-inherited | 設定持久化 | 有 | replace | 原創重寫 | 否 | 低 | B1-4 之前 |
| B1-8 | `internal/runtimelog`、`internal/narrative` | confirmed-inherited | 日誌與敘述工具 | 部分 | replace | 原創重寫或改用標準庫 | 否 | 低 | 任意 |

### B2. Runtime-critical inherited frontend

| # | Path | Provenance | 目前用途 | Taiwan 相依 | 處置 | 替換方式 | TWstock MIT 可替換 | 難度 | 相依順序 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| B2-1 | `frontend/src/lib/backend.ts` | likely-inherited | 所有 API 請求 | 有 | replace | 原創重寫（薄封裝，約 150 行） | 否 | 低 | 最先做 |
| B2-2 | `frontend/src/App.tsx`（1,306 行） | likely-inherited | 應用外殼、導覽、hash 路由 | 有 | replace | 移除 A 股分支後原創重寫為純台股外殼 | 否 | 中 | B4 之後（A 股畫面移除可大幅簡化） |
| B2-3 | `frontend/src/styles.css` | likely-inherited | 全站樣式 | 有 | replace | 原創設計系統 | 否 | 中 | 可獨立進行 |
| B2-4 | `frontend/src/lib/hermes.ts`、`llm-providers.ts` | likely-inherited | AI 連線 | 有 | replace | 隨 B1-6 一併重寫 | 否 | 中 | B1-6 之後 |
| B2-5 | `frontend/src/components/{SettingsDrawer,AppUpdatePanel,MarkdownContent,HermesAgentSettingsPanel}.tsx` | confirmed / likely | 共用 UI | 有 | replace | 原創重寫 | 否 | 中 | B2-2 之後 |
| B2-6 | `frontend/index.html`、`main.tsx`、`vite.config.ts`、`tsconfig.json`、`package.json` | confirmed / likely | 建置設定 | 有 | replace | 重新產生（`npm create vite`）後套用既有設定 | 否 | 低 | 任意 |

### B3. Desktop shell / packaging identity

| # | Path | Provenance | 目前用途 | Taiwan 相依 | 處置 | 替換方式 | 難度 | 相依順序 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| B3-1 | `desktop/main.cjs` `app.setName('easy-stock')` | likely-inherited | 決定 userData 目錄 | 有 | replace | **需資料遷移**：改名會使既有安裝讀不到自選股、持倉、研究歷史。必須先寫遷移邏輯（偵測舊目錄 → 複製 → 標記完成） | 中 | 必須在 B3-2 之前 |
| B3-2 | `desktop/scripts/electron-builder.mjs` `appId: com.jundizhou.easystock`、`productName: easy-stock`；`package-mac.mjs` `appBundleId` | likely-inherited | 安裝檔識別 | 有 | replace | **會破壞安裝升級路徑**：既有使用者會得到並存的第二份安裝。需搭配版本公告 | 中 | B3-1 之後 |
| B3-3 | `desktop/update-feed.cjs` 預設更新來源 = 上游 OSS bucket | confirmed-inherited | 自動更新 | 有 | investigate → replace | 需先建立自有更新來源（GitHub Releases 或自有物件儲存），再切換 | 中 | 獨立，但屬**優先處理**（供應鏈） |
| B3-4 | `desktop/*.cjs` 其餘 shell 檔案 | confirmed / likely | 桌面 shell | 有 | replace | 原創重寫 | 中 | B3-1 之後 |
| B3-5 | `desktop/scripts/*`（16 檔）、`desktop/test/*`（15 檔） | confirmed / likely | 封裝與測試 | 有 | replace | 原創重寫 | 中 | B3-2 之後 |

### B4. Legacy A-share modules

| # | 範圍 | 處置 | 說明 | 難度 | 相依順序 |
| --- | --- | --- | --- | --- | --- |
| B4-1 | 中國 providers（39 檔）+ 對應 `httpapi` 路由 | remove | 台股零相依 | 低 | 可最先做，且能大幅縮小後續替換面積 |
| B4-2 | `portfolioinspection`(12)、`review`(12)、`strategy`(4)、`methodology`(47) + 路由 | remove | 台股零相依 | 低 | B4-1 之後 |
| B4-3 | A 股前端畫面與 lib（約 38 檔）+ `App.tsx` hash 分支 | remove | 台股零相依 | 低 | 與 B4-2 併行；完成後 B2-2 難度大幅下降 |
| B4-4 | 桌面 A 股登入橋接（`xueqiu-*`、`taoguba-*`、`browser-auth.cjs`、`review-login-preload.cjs`） | remove | 隨 B4-2 一併 | 低 | B4-2 之後 |
| B4-5 | `integrations/wechat-download-api`（AGPL-3.0-only）與封裝步驟 | remove | 隨 B4-2 一併；同時解除 AGPL 散布議題 | 低 | B4-2 之後 |
| B4-6 | `A_STOCK_*` 環境變數前綴（20+ 個）與資料庫檔名 | replace | **需相容遷移**：直接改名會使既有安裝讀不到本機資料 | 中 | B3-1 之後，與其共用遷移機制 |

### B5. Scripts

| # | Path | Provenance | 處置 | 難度 |
| --- | --- | --- | --- | --- |
| B5-1 | `scripts/rebuild-restart.sh` | confirmed-inherited | replace（原創重寫，或併入既有的原創 `.mjs` 版本） | 低 |
| B5-2 | `.github/workflows/release.yml` | confirmed-inherited | replace（隨 B3-3 重寫發布流程） | 中 |

### B6. Tests

| # | 範圍 | 處置 | 說明 | 難度 |
| --- | --- | --- | --- | --- |
| B6-1 | 繼承的後端測試（約 40 檔） | 隨對應模組 remove / replace | 測試 provenance 跟隨被測模組 | 低 |
| B6-2 | 繼承的前端測試（約 15 檔） | 同上 | — | 低 |
| B6-3 | 繼承的桌面測試（15 檔） | replace | 隨 B3 重寫 | 中 |

### B7. Assets

| # | 範圍 | 處置 | 替換方式 | 難度 |
| --- | --- | --- | --- | --- |
| B7-1 | `desktop/assets/easy-stock.{icns,ico,png,svg,iconset}` | replace | 原創圖示設計，並更新 `electron-builder.mjs` 引用 | 低（設計工作） |
| B7-2 | `frontend/public/easy-stock-mark.{svg,png}` | replace | 原創 favicon | 低 |
| B7-3 | `docs/assets/easy-stock-*`（13 個 A 股截圖） | remove | README 已不引用；確認其他文件無引用後移除 | 低 |

### B8. Remaining docs

| # | 範圍 | 處置 | 難度 |
| --- | --- | --- | --- |
| B8-1 | `backend/docs/{architecture,data-sources,roadmap,api-routes,hermes-integration,inflection-*,live-tests,sector-map}.md`（10 檔） | replace（台股相關重寫）／remove（A 股專屬） | 低 |
| B8-2 | `docs/{user-guide,development,market-overview-plan,billboard-seat-mappings,daily-review-oss-sync}.md` | replace / remove | 低 |
| B8-3 | `.github/release-notes/v0.1.0–v0.9.2`（13 檔） | keep（歷史 attribution） | — |
| B8-4 | `frontend/src/lib/hermes.ts` 的 `client: 'easy-stock-frontend'` 協定識別字串 | replace（隨 B1-6／B2-4，需前後端同步） | 低 |

### 建議執行順序

```
B4-1  移除中國 providers 與路由        ← 面積最大、風險最低，先做
B4-2  移除 A 股後端模組
B4-3  移除 A 股前端畫面
B3-3  建立自有更新來源                  ← 供應鏈優先，可與上列併行
B1-1  Go module rename
B1-7  appsettings → B1-2 main.go → B1-3 server.go → B1-4 config/settings
B1-5  foundation 型別（可與 B1-3 併行）
B2-1  backend.ts → B2-6 建置設定 → B2-2 App.tsx → B2-3 styles → B2-5 共用元件
B3-1  userData 遷移 → B4-6 環境變數前綴 → B3-2 appId/productName → B3-4/B3-5
B1-6  Hermes runtime 重寫 → B2-4 前端 AI 連線     ← 最大單項，可獨立排程
B5    scripts / release workflow
B6    測試隨模組
B7    素材原創化
B8    文件
```

### Phase B 整體難度評估

| 區塊 | 檔案量級 | 難度 |
| --- | --- | --- |
| B4 legacy 移除 | 約 114 後端 + 38 前端 + 5 桌面 | 低（機械性，但需逐步驗證 build） |
| B1 後端 runtime 替換 | 約 15 檔（其中 `hermes/runtime.go` 1,418 行） | 中～高 |
| B2 前端 runtime 替換 | 約 12 檔（其中 `App.tsx` 1,306 行） | 中 |
| B3 桌面識別遷移 | 約 35 檔 + 使用者資料遷移 | 中（風險集中在資料遷移與升級路徑） |
| B5–B8 | 約 45 檔 + 20 素材 | 低 |
| **整體** | **約 260 個檔案需 remove 或 replace** | **中～高；`hermes/runtime.go` 與桌面資料遷移為兩個主要風險點** |

---

## Current HEAD ready for relicensing

**NO**

### Reasons

1. `LICENSE` 的著作權人為 jundizhou，本專案維護者沒有變更授權條款的權利。
2. HEAD 仍含 370 個繼承自上游的檔案（約 66,700 行繼承程式碼）。
3. 所有已完成的台股功能——包含 Taiwan Daily Dashboard——都在繼承的 HTTP server、設定層、AI runtime、前端外殼與桌面 shell 之內執行，無法單獨切離。
4. Go module path 仍為 `easy-stock/backend`，230 個 Go 檔案以此匯入。
5. 桌面安裝包內建 AGPL-3.0-only 的第三方服務。
6. 部分素材與資料（圖示、席位對照表、游資心法快取）的原始出處在 repository 中無記錄，provenance 為 `unclear`。

### Remaining blockers

| # | Blocker | 類型 |
| --- | --- | --- |
| 1 | 著作權人同意，或確認 HEAD 已不含上游受著作權保護的表達 | 法律／人工 |
| 2 | B1 runtime-critical 後端替換（含 `hermes/runtime.go`） | 工程 |
| 3 | B2 runtime-critical 前端替換（含 `App.tsx`、`backend.ts`、`styles.css`） | 工程 |
| 4 | B3 桌面 shell 與封裝識別替換（含使用者資料遷移） | 工程 |
| 5 | B4 legacy A 股模組移除（含 AGPL 元件） | 工程 |
| 6 | B7 繼承素材原創化 | 設計 |
| 7 | `unclear` 項目的出處確認：席位對照資料、游資心法快取內容、圖示設計來源 | 人工調查 |
| 8 | 桌面更新來源仍指向上游控制的 OSS bucket | 工程／人工決策 |

---

## 本輪（Phase A）已完成的項目

- 建立本稽核文件。
- Package identity：root `easy-stock` → `mystocktracer`；`easy-stock-frontend` → `mystocktracer-frontend`；`easy-stock-desktop` → `mystocktracer-desktop`；`desktop.author` → `mystocktracer contributors`；`scripts/rebuild-restart.sh` 的 tmux session 名稱。lockfile 僅含對應的 identity 變更。
- `README.md`：修正指向上游的支援連結、改用本機圖示路徑、更新為實際完成的台股能力、新增「專案狀態」章節、保留上游署名與授權章節。
- `CONTRIBUTING.md`：由簡體中文 A 股版本改寫為繁體中文 mystocktracer 貢獻指南，新增台股資料來源要求、資料正確性要求、AI 協助開發揭露、外部程式碼授權要求、安全與隱私章節；授權章節如實反映目前的 PolyForm Noncommercial。
- `ROADMAP.md`：更新為實際完成狀態，分離舊版 A 股能力與台股產品方向，新增 OSS Independence / Licensing Transition 階段。
- `SECURITY.md`：回報路徑改指向本 repository，明列受保護的資料類型，明確要求**不要**提交金鑰、完整持倉或本機資料庫。
- `.github/ISSUE_TEMPLATE/`：bug / feature / config 三份模板改寫為繁體中文台股版本，新增資料來源、`as_of`、授權 provenance、AI 需求欄位與隱私警告。
- `.github/PULL_REQUEST_TEMPLATE.md`：新增 Scope、Data source、Provenance、AI behavior、Migration、Security/privacy、License declaration 章節。

### 本輪刻意未做

| 項目 | 原因 |
| --- | --- |
| 變更 root `LICENSE` | 需著作權人同意；且 provenance 尚未清理完成 |
| 移除 easy-stock attribution | 衍生關係屬事實，必須保留 |
| Go module rename | 影響 230 個 Go 檔案，會使本 PR 無法有效審查；列為 B1-1 |
| 桌面 `app.setName` / `appId` / `productName` 變更 | 會造成使用者資料遺失與安裝升級路徑中斷；列為 B3-1／B3-2 |
| `desktop/update-feed.cjs` 更新來源變更 | 尚無自有更新來源，直接改會使既有安裝無法更新；列為 B3-3 |
| `frontend/src/lib/hermes.ts` 的 `client` 協定字串 | 屬 runtime 協定識別，變更需前後端同步；列為 B8-4 |
| 大規模刪除 legacy A 股模組 | 仍有 runtime 路由與測試覆蓋，屬 Phase B 範圍 |
| 引入任何 OpenStock 或 TWstock 程式碼 | 本輪為稽核，不做程式碼搬遷 |
