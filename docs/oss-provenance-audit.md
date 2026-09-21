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

| 指標 | Phase B2 前 | Phase B2 後 |
| --- | ---: | ---: |
| 納入統計檔案 | 278 | 292 |
| 繼承檔案合計 | 128 | 113 |
| `confirmed-inherited` | 66 | 54 |
| `likely-inherited` | 62 | 59 |
| `original` | 134 | 163 |
| `unclear` | 16 | 16 |
| inherited code LOC | 19,153 | 9,069 |
| original code LOC | 35,174 | 38,859 |

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

## 目前原創且可獨立維護的 runtime 區域

- 官方台股 provider、ToAlpha/MOPS 補充、PIT／freshness contracts。
- Dashboard、Watchlist、Portfolio、Alerts、Stock Research、Research History、Previous Comparison 與 Screener。
- 本輪的 Go bootstrap、settings persistence、HTTP boundary、frontend transport、Taiwan shell 與 styling foundation。
- updater 已由 Phase B1 改為本 repository GitHub Releases。

## 仍存在的 inherited runtime 表達

以下是對「是否可 relicensing」仍具實質影響的 runtime 模組；測試、歷史 release notes 與一般文件另依腳本逐檔列入統計，但不是產品 runtime replacement blocker。

| 區域 | 目前分類 | 狀態／後續 |
| --- | --- | --- |
| `backend/internal/hermes/runtime.go` | likely-inherited | AI model runtime 核心；已隔離在 `httpapi.AgentRuntime` 窄介面後，Phase B3 原創替換 |
| `frontend/src/lib/hermes.ts`、`AIChatWorkspace.tsx` | confirmed-inherited | AI transport／對話 UI，Phase B3 |
| `SettingsDrawer.tsx`、`HermesAgentSettingsPanel.tsx` | likely-inherited | 高度依賴 Hermes 設定 schema，Phase B3 |
| `frontend/src/components/MarkdownContent.tsx` | confirmed-inherited / trivial | 30 行標準 Markdown wrapper；不是目前 runtime independence blocker，但 relicensing 前仍需替換或取得權利 |
| `AppUpdatePanel.tsx` | likely-inherited | 與現有 desktop identity／更新 UX 綁定，Phase B4 |
| `backend/internal/httpapi/llm_connection.go`、`llm_models.go` | likely-inherited | Hermes provider boundary；本輪已修正 secret／provider error 外洩，完整替換併入 Phase B3 |
| `backend/internal/narrative/narrative.go` | confirmed-inherited | AI narrative helper，Phase B3 評估或替換 |
| `backend/internal/runtimelog/writer.go` | confirmed-inherited | 共用 bounded/redacted logger；桌面/runtime phase 原創替換 |
| `backend/internal/marketemotion/types.go` | confirmed-inherited | 小型 shared types；台股實作仍使用，後續薄化或原創替換 |
| `desktop/main.cjs`、`preload.cjs`、`backend-process.cjs`、user-data/update/package scripts | confirmed／likely-inherited | 桌面 lifecycle、封裝與資料 migration，Phase B4；本輪只做 canonical env emission |
| `desktop/assets/easy-stock.*`、`frontend/public/easy-stock-mark.*` | `unclear` | 原始設計來源無 repository 證據，需原創素材替換 |

## 第三方相依與授權狀態

- npm／Go dependencies 仍依各自授權；本輪未升級或新增 dependency。
- Phase B1 已移除套件中的 AGPL-3.0-only WeChat download API。
- 上游 easy-stock 的衍生關係、作者署名、Git 歷史與 PolyForm Noncommercial root license 持續保留。

## Current HEAD ready for relicensing

**NO**

原因：

1. 目前仍有上表所列的 Hermes、AI UI、desktop shell 與共用 runtime inherited expression。
2. 圖示／favicon provenance 仍為 `unclear`。
3. 目前維護者沒有單方面重新授權上游著作權表達的權利；工程替換完成後仍需合法權利基礎與人工法律判斷。

推薦下一階段：**Phase B3 — AI Runtime Independence**。其後進行 **Phase B4 — Desktop Identity Independence**，並以可回復的使用者資料 migration 保護既有安裝。
