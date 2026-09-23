# B6 行為與來源判定契約

本文件記錄 B6 修改前的外部行為。來源掃描只是候選篩選；保留 JSON 鍵、
HTTP 路由、套件宣告及歷史相容識別符，本身不代表保留上游實作表達。

## Runtime

| 邊界 | 必須保留的行為 | 驗證入口 |
| --- | --- | --- |
| `/api/v1/settings` | GET 的 `data`/`agent`/`hermes`/`llm`/`llm_profiles`/`taiwan_alerts` 形狀；PUT 64 KiB 上限、未知欄位及多 JSON 物件拒絕；失敗狀態 400/503/500；金鑰不得寫入一般設定或回應；保留 profile 選擇、timeout、清除密鑰與既有設定讀取 | `backend/internal/httpapi/settings_test.go`、`backend/internal/appsettings/store_test.go` |
| `/api/v1/settings/agent` | GET/PUT 的 `data` 形狀；PUT 256 KiB 上限；保留現有 MCP 私密欄位與未提交的密鑰；驗證失敗 400、服務不可用 503、儲存／讀取失敗 500；不顯示舊 A 股 Skill | `backend/internal/httpapi/settings_agent_test.go` |
| HTTP response | JSON content type、原有 status 與 `error` 鍵；錯誤記錄遮罩；`limit` 範圍與多 JSON 物件拒絕 | `backend/internal/httpapi/server_test.go` 及 settings tests |
| 市場資料 DTO | `Quote`、`KLine`、指數與 `SourceMeta` 的 JSON 鍵、單位、來源／時間／PIT 語意；不更動 provider 計算 | Taiwan provider / market tests |
| AI 與設定 DTO | agent capability、LLM、profile、broker 與 alert 的 JSON/持久化欄位及預設值 | agent、appsettings、httpapi tests |
| 指數畫面 | 同一 index/series 的價格、漲跌、日期、區間高低、來源、狀態、空／載入畫面；不改 backend 數值 | Taiwan market tests 及 B6 元件測試 |
| 前端 helper | chat history 上限、legacy session 只讀相容、provider defaults、local preset、runtime log 遮罩與 best-effort IPC | `frontend/src/lib/*.test.ts` |

## 測試替換範圍

後端測試需保護 server storage/env precedence、設定持久化與舊檔讀取、
WebSocket product protocol、模型列表請求與錯誤、strict persistence、
token/CORS/legacy routes、一般設定與 Agent 設定的成功、失敗及秘密保護。
前端測試需保護 Markdown 安全渲染、loopback backend/token 邊界、
對話歷史、供應商預設值與 renderer log 邊界。替換測試使用合成輸入，
不刪除有效情境；對必要的失敗及邊界條件增加明確斷言。

## 功能性來源

Go/TypeScript DTO 的 JSON 鍵、Vite/React 啟動宣告、go.mod、tsconfig、
package.json 與 .gitignore 的標準語法、dependencies/腳本名稱是介面或工具需求。
各檔仍需逐項判斷有無上游特有表達；只在沒有實質複製表達且有對應
契約／驗證時，才可加入綁定 SHA 的 provenance decision。

## 逐檔判定與替換證據

| 路徑 | 判定與理由 |
| --- | --- |
| `backend/internal/agent/types.go` | 產品能力 interface、傳輸 DTO 與 JSON 名稱；推理程度集合是介面允許值，線性 membership 是功能性判斷。沒有第三方 process protocol 實作。 |
| `backend/internal/appsettings/model.go` | 持久化 schema、逾時上下限與舊 profile 的讀取相容；欄位及預設值不能任意變更。實際檔案讀寫由 B2 的 `store.go` 負責。 |
| `backend/internal/foundation/market_data.go`、`market_index.go`、`source.go` | 市場 API 的型別、JSON 鍵、來源／日期／單位契約；沒有市場計算或 provider 實作。 |
| `backend/internal/httpapi/agent_settings_contract.go`、`settings_contract.go` | Request/response DTO 與舊 UI 相容鍵；API mode 的映射和四碼遮罩是功能性、固定的協定行為。驗證和合併分別位於原創的 validation/handler/model。 |
| `frontend/index.html`、`frontend/src/main.tsx`、`frontend/vite.config.ts` | Vite/React 啟動所需的 HTML root、module script、React root、React plugin、相對 base 及 loopback dev server 宣告；品牌 meta 和 runtime logger 是 mystocktracer 自有內容。 |
| `frontend/src/lib/llm-providers.ts` | 供應商 ID、URL、預設模型和 API mode 是連線互通設定；未知值、Ollama/LM Studio 與錯誤文字為本產品契約。無上游特有的服務呼叫。 |
| `.gitignore`、`backend/go.mod`、`frontend/tsconfig.json`、`frontend/src/global.d.ts`、根與 workspace `package.json` | Git、Go、TypeScript、npm/Electron 的標準宣告、現用依賴版本與 build/test script；不以改依賴或編譯選項製造外觀差異。 |

本輪以新的控制流程與合成 fixture 重寫 `settings_handler.go`、`agent_settings_handler.go`、
`agent_settings_model.go`、`settings_view.go`、`response.go`、`MarketDataViews.tsx`、
`chat.ts`、`runtime-log.ts`，並更新 index 畫面 CSS。設定與 Agent 的 JSON 鍵、
64/256 KiB 大小限制、狀態碼、密鑰保留與 profile 合併有同路徑及新增測試覆蓋。
市場畫面只使用既有 index/series 數值與來源 metadata，額外測試空資料、載入與備援提示。

14 個既有 test 路徑已用獨立案例與合成資料重寫：backend 的 server/main、
appsettings/store、HTTP WebSocket/模型清單/持久化/伺服器/兩類設定，
frontend 的 Markdown/backend/chat/provider/runtime log。新測試針對 malformed JSON、
大小上限、私密欄位、來源不同步與歷史資料相容等負向情況；
對應的原有行為仍由完整 Go、frontend、desktop 及 tooling suite 驗證。

Issue/PR 模板、CONTRIBUTING、README、ROADMAP、SECURITY、四份 backend 文件及
使用指南已針對目前台股產品與 maintenance 狀態重新撰寫。法律授權、
`.github/release-notes/v0.x.x.md` 歷史文字與 Git history 原封保留。
`easy-stock`/`a-stock-ai` profile、`A_STOCK_*`/`VITE_A_STOCK_*`、舊 NSIS GUID 等
只在明確讀取舊安裝或資料時存在；canonical runtime 以 mystocktracer 命名。
