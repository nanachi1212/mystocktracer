# OSS 來源稽核

本文件記錄 mystocktracer 目前 tree 的來源分類與獨立化進度。它是工程稽核，不是法律意見；root `LICENSE`、上游署名與完整 Git 歷史均未變更。

## 稽核基準與可重現方法

- 上游分叉點：`b969d05984dda736de9ee3dc2a881e8c65a373c5`。
- Phase B2 起點：`1b4310da00607a9431d18ccdddd6347f0b94d204`。
- 執行 `node scripts/provenance-audit.mjs` 可重算下表。腳本以 Git blob identity 比對分叉點與目前 tree，另對本階段依 behavioral contract 完整替換的同路徑檔案套用人工審核清單。
- `node_modules`、`vendor`、`dist`、`build`、`out`、`coverage`、`package-lock.json`、`go.sum` 排除；LOC 僅計 `.go/.ts/.tsx/.cjs/.mjs/.css/.html`。
- 重新命名或搬移可能無法只靠 blob identity 判斷；因此「modified」不自動等同 `original`，同路徑 replacement 也必須有本輪 contract、diff 與測試證據。
- 二進位圖示無可用 repository 內來源證據，維持 `unclear`，不以零 LOC 誤解為已釐清。

| 分類 | 判定 |
| --- | --- |
| `confirmed-inherited` | 分叉點存在，且目前 blob 未變 |
| `likely-inherited` | 分叉點存在且內容有修改，但沒有足夠證據證明為完整原創 replacement |
| `original` | 分叉後新增的原創實作，或本階段依獨立 behavioral contract 完整替換並人工確認 |
| `unclear` | repository 證據不足，無法確認原始表達來源 |

## Current-tree 統計

| 指標 | Phase B2 前 | Phase B3 後 |
| --- | ---: | ---: |
| 納入統計檔案 | 278 | 297 |
| 繼承檔案合計 | 128 | 100 |
| `confirmed-inherited` | 66 | 42 |
| `likely-inherited` | 62 | 58 |
| `original` | 134 | 181 |
| `unclear` | 16 | 16 |
| inherited code LOC | 19,153 | 5,142 |
| original code LOC | 35,174 | 40,775 |

新增檔案數包含把大型檔案拆成小型責任邊界的結果，不能用總檔案數推論來源狀態。

## Phase B1 與 B2 結果

Phase B1 已移除 legacy China/A-share provider、methodology、portfolioinspection、review、strategy/inflection、舊前端 workspace、登入 bridge、上游 A 股文件／素材與 AGPL-only WeChat 元件，並把更新來源切到本 repository GitHub Releases。

Phase B2 完成下列原創 runtime replacement：

- Go module：`easy-stock/backend` → `github.com/nanachi1212/mystocktracer/backend`，70 個 runtime import 已遷移。
- `cmd/server` bootstrap 與 `appsettings` persistence：保留既有路徑、JSON schema、secret 與 atomic write 行為。
- `httpapi` server/router/middleware、一般 settings 與 Agent/MCP settings、AI WebSocket outer bridge。
- `foundation` 移除 China symbol abstraction，只留下台股功能已使用的薄型 runtime primitives；TWstock 仍是 canonical Taiwan market truth。
- 前端 `backend.ts` transport、Taiwan-only `App.tsx` shell／navigation、global styling foundation。
- `MYSTOCKTRACER_*` 為 canonical runtime 環境變數；未設定時才讀取 deprecated `A_STOCK_*`。桌面啟動後端只寫 canonical 名稱。

本階段沒有修改 Electron `app.setName`、`appId`、`productName` 或 `userData` root；`settings.json`、`taiwan-watchlist.db`、`taiwan-portfolio.db`、研究歷史與 alert inbox 路徑保持相容。

## Phase B3 結果

- 移除 `backend/internal/hermes` 的 inherited runtime，改由 `internal/agent` 的產品契約與 `internal/agent/hermesadapter` 可替換 adapter 承接；HTTP、台股研究與 bootstrap 僅依賴產品窄介面。
- 前端對話改採產品版 WebSocket v1，renderer 不接觸第三方 runtime frame；一般聊天與台股 AI Research 的 capability boundary 分離，並以 packaged fake-runtime 完成多段 streaming smoke。
- 移除舊 Hermes settings panel 與前端 transport；`SettingsDrawer` 僅負責產品設定組合，模型與 Skill/MCP 各自抽成產品面板，secret 僅顯示遮罩狀態，支援保留與明確清除。
- 模型清單與連線探測移到 `internal/agent` 產品服務；HTTP handler 不再實作 provider discovery 或 runtime probe 規則。
- 封裝 Hermes Agent `0.18.2` 時，隨 runtime 放置其 MIT `LICENSE`，並隨 release resources 放置 `THIRD_PARTY_NOTICES.md`；封裝驗證器會要求兩者存在。
- 本輪完整替換的 AI 邊界與 Markdown safe-link renderer 已列入 provenance audit 的 replacement contract；其餘 desktop lifecycle 與非 AI runtime 仍維持原分類。

## 目前原創且可獨立維護的 runtime 區域

- 官方台股 provider、ToAlpha/MOPS 補充、PIT／freshness contracts。
- Phase B4 的桌面 identity（`com.nanachi1212.mystocktracer`）、啟動 lifecycle、使用者資料 migration 與品牌素材產生流程。
- Dashboard、Watchlist、Portfolio、Alerts、Stock Research、Research History、Previous Comparison 與 Screener。
- 本輪的 Go bootstrap、settings persistence、HTTP boundary、frontend transport、Taiwan shell 與 styling foundation。
- updater 已由 Phase B1 改為本 repository GitHub Releases。

## 仍存在的 inherited runtime 表達

以下是對「是否可 relicensing」仍具實質影響的 runtime 模組；測試、歷史 release notes 與一般文件另依腳本逐檔列入統計，但不是產品 runtime replacement blocker。

| 區域 | 目前分類 | 狀態／後續 |
| --- | --- | --- |
| `backend/internal/hermes/runtime.go`、`frontend/src/lib/hermes.ts`、`HermesAgentSettingsPanel.tsx` | removed | Phase B3 已刪除；產品契約與 adapter 取代其 runtime/transport/settings responsibility |
| `AIChatWorkspace.tsx`、`SettingsDrawer.tsx`、`MarkdownContent.tsx` | original | Phase B3 依產品 contract 完整替換；對話協定、設定組合與 safe-link renderer 均有回歸測試 |
| `AppUpdatePanel.tsx` | likely-inherited | Phase B4 已改綁 canonical identity 與 GitHub Releases 更新流程；UI 表達仍待原創替換 |
| `backend/internal/httpapi/llm_connection.go`、`llm_models.go` | original | Phase B3 完整替換成薄 HTTP handler；模型 discovery 與 probe 規則由產品 `internal/agent` 服務持有 |
| `backend/internal/narrative/narrative.go` | removed | 沒有 production consumer，Phase B3 已移除 |
| `backend/internal/runtimelog/writer.go` | confirmed-inherited | 共用 bounded/redacted logger；桌面/runtime phase 原創替換 |
| `backend/internal/marketemotion/types.go` | confirmed-inherited | 小型 shared types；台股實作仍使用，後續薄化或原創替換 |
| `desktop/main.cjs`、`app-lifecycle.cjs`、`identity.cjs`、`user-data-migration.cjs`、packaging scripts | original | Phase B4 依產品契約重寫桌面 lifecycle、identity 與資料 migration；`main.cjs` 僅保留薄 entry point |
| `desktop/preload.cjs`、`backend-process.cjs`、`hermes-runtime-root.cjs`、`runtime-logger.cjs`、`data-protection.cjs`、`update-*.cjs` | likely-inherited | 檔案結構沿用上游輪廓，內容已隨 B1–B4 大幅替換；剩餘表達量小 |
| `desktop/assets/easy-stock.*`、`frontend/public/easy-stock-mark.*` | removed | Phase B4 已移除，改用本 repository 產生的 `mystocktracer.*` / `mystocktracer-mark.*` 原創素材（見 `docs/brand-assets.md`） |
| `desktop/scripts/*`、`desktop/test/*`、部分 `frontend/src` 測試 | likely-inherited | 封裝流程與測試骨架；非產品 runtime，不是 relicensing 的主要 blocker |

## 第三方相依與授權狀態

- npm／Go dependencies 仍依各自授權；本輪未升級或新增 dependency。
- Phase B1 已移除套件中的 AGPL-3.0-only WeChat download API。
- 上游 easy-stock 的衍生關係、作者署名、Git 歷史與 PolyForm Noncommercial root license 持續保留。

## Phase B4 結果

Phase B4 完成 desktop identity independence：

- canonical app identity：`mystocktracer` / `com.nanachi1212.mystocktracer`，後端執行檔改名為 `mystocktracer-backend`。
- 使用者資料改用 canonical 目錄，並以 staging→verify→atomic rename 的 migration 承接既有 `easy-stock` / `desktop` 安裝；來源目錄永不刪除，任何驗證失敗都 fail-closed 並保留原資料（13 項 migration 測試涵蓋 symlink escape、SQLite 完整性、並行啟動與中斷重建）。
- 品牌素材改為本 repository 產生的原創 `mystocktracer` icon／mark，舊 `easy-stock` 素材已移除，`unclear` provenance 項目歸零。

統計（以 fork 起點比對，見 `scripts/provenance-audit.mjs`）：

| 指標 | Phase B3 後 | Phase B4 後 |
| --- | --- | --- |
| tracked 檔案 | 278 | 295 |
| inherited 檔案 | 128 | 97 |
| confirmed-inherited | 66 | 23 |
| likely-inherited | 62 | 74 |
| original | 134 | 198 |
| unclear | 16 | 0 |
| inherited LOC | 19,153 | 3,310 |
| original LOC | 35,174 | 41,245 |

仍具 runtime 影響的 inherited 檔案為 60 個（54 likely／6 confirmed），集中在封裝腳本與測試骨架，不再有 A 股或上游產品語意。

## Current HEAD ready for relicensing

**NO**

原因：

1. 仍有 97 個 inherited 檔案（約 3,310 LOC），其中 6 個為 confirmed-inherited；雖已無 A 股語意與上游品牌，但著作權表達仍源自上游。
2. 目前維護者沒有單方面重新授權上游著作權表達的權利；工程替換完成後仍需合法權利基礎與人工法律判斷。

相較 Phase B3，`unclear` provenance 已歸零，inherited LOC 由 19,153 降至 3,310。

推薦下一階段：**Phase B5 — Packaging & Test Scaffold Independence**，替換 `desktop/scripts/*` 與剩餘 confirmed-inherited 測試骨架，之後再進行人工法律判斷。
