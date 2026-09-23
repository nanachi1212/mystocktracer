# mystocktracer Runtime Behavioral Contract

本文件只定義產品對外可觀察的 runtime 行為，不描述、複製或規定任何歷史實作結構。替換 runtime infrastructure 時必須保持以下契約。

## Backend startup

- 後端預設監聽 `127.0.0.1:20081`；`MYSTOCKTRACER_ADDR` 可覆寫，過渡期在新名稱未設定時接受 `A_STOCK_ADDR`。
- 原始碼 backend 依序選擇有既有狀態的 `mystocktracer`、`easy-stock`、`a-stock-ai` 使用者設定目錄；狀態包含 settings、任一台股資料庫或非空 hermes-home。單獨 backend 不搬動資料，沒有既有狀態則使用 canonical 目錄。Electron 的 COPY migration 另依 [桌面身份遷移](./desktop-identity-migration.md) 執行。
- 設定檔預設為資料目錄下的 `settings.json`；自選股與持倉分別繼續使用 `taiwan-watchlist.db` 與 `taiwan-portfolio.db`。現金流量快取若啟用，路徑不得在無 migration 時變更。
- `MYSTOCKTRACER_*` 是新的 canonical 環境變數前綴；對台股 runtime 仍需要的設定，若新名稱未設定，才回退讀取對應的 `A_STOCK_*`。新名稱永遠優先。
- 啟動時只建立當前台股產品所需的 settings、Watchlist、Portfolio、Taiwan provider、cashflow cache、Hermes gateway 與 HTTP server。
- 收到終止訊號後停止接收新請求，在有限時間內完成正在處理的請求並關閉可關閉的 store/runtime。
- `GET /api/health` 不需 token，成功回應 HTTP 200 與 JSON 健康狀態。

## Settings persistence

- 設定檔不存在時以安全預設值啟動；記憶體模式不寫檔。
- 指定路徑時會建立父目錄，並以同目錄暫存檔加 rename 的方式原子替換；在支援的平台上限制檔案權限。
- 無效 JSON 必須使開啟失敗並回報原因，不得靜默覆寫或當成空設定。
- 同一 process 的讀寫為 concurrency-safe；讀者不會觀察到半完成的更新。
- 舊版 `settings.json` 可讀取：未知與已淘汰欄位不影響已知設定載入；新寫入的檔案只包含 mystocktracer 仍支援的 schema，不重生 China-only 欄位。
- 舊版單一 LLM 設定可移轉為可選 profile；未設定的 timeout 與台股公司事件提醒套用相容預設值。
- 一般 `settings.json` 不儲存模型 API key；secret 交給專用 runtime boundary，且不出現在 log 或 API response。

## HTTP

- 除 health 與 CORS preflight 外，當 server 設有 token 時，API 需要 `Authorization: Bearer <token>`；現有 WebSocket 相容性所需的 query token 仍可使用。未設 token 的本機開發模式保持開放。
- token 比對使用 constant-time comparison。未授權回應 HTTP 401 JSON，不回傳 token 細節。
- CORS 只回映允許的 localhost/loopback app origin，並支援現行的 `GET, POST, PUT, DELETE, OPTIONS` 與 `Authorization, Content-Type, X-Request-ID` headers；preflight 回應 204。
- JSON 成功與錯誤回應都設定 JSON content type。錯誤訊息對使用者安全，不外洩 provider response body、secret、本機路徑或 stack trace。
- 會讀取 request body 的 endpoint 有明確上限，超過上限或多餘 JSON 均拒絕；WebSocket 訊息亦有大小上限。
- request log 包含 method、path、status、duration、feature 與 correlation/request ID，但不記錄 query string、token、authorization header、API key 或 request body。
- 路由僅註冊 health、settings、Hermes status/model/agent config、AI chat bridge 與現行 `/api/v1/tw/*` 台股 API。Phase B1 已移除的 A-share routes 持續回應 404。
- settings API 保持現行 path 與 response envelope，驗證 URL、provider、model、timeout、profile、Taiwan corporate-event preference 與 Hermes agent config。secret 只回應 `configured` 狀態，不回應實值。
- 模型連線測試經 Hermes boundary 執行，成功與失敗保持現有狀態碼與安全分類。

## AI WebSocket bridge

- HTTP layer 只依賴窄化的 `AgentRuntime` 介面：狀態、設定同步、prompt/session 操作與取消。Hermes 是本階段的 adapter，其內部實作不是 HTTP contract。
- upgrade 前執行與一般 API 相同的驗證；不合法 origin、過大訊息與無效 JSON 會被拒絕。
- client 斷線、request context 取消或 server shutdown 會取消對應 runtime 工作，不留下孤兒 session。
- 保持現行前端 chat event、session ID、內容與錯誤分類相容；不在 bridge 改變 LLM protocol、tool calling 或 AI Research schema。

## Frontend transport

- backend base URL 由桌面 runtime 注入值或 `VITE_MYSTOCKTRACER_BACKEND_URL` 取得，過渡期 fallback 為 `VITE_A_STOCK_BACKEND_URL`，最後預設 `http://127.0.0.1:20081`。token 同樣以新名稱優先。
- 一般 HTTP token 使用 Bearer header；WebSocket 僅在瀏覽器 API 限制下使用現有 query-token 相容方式。backend URL 會被正規化，不接受不安全或非 HTTP(S) 的任意 scheme。
- 集中 request helper 支援 `AbortSignal`、JSON request/response、204/205 或空 response body，並在非 2xx、錯誤 JSON 與網路失敗時拋出具 status/code 的 typed error。
- 錯誤不得被轉成 `0`、空成功物件或無訊息的成功。`available`、`stale`、`partial`、`unavailable` 與 not queried 在 transport 層保留原始語意。
- caller 可主動取消 request；UI 的 scoped request/request ID 機制必須阻止過時回應覆寫新選擇。
- 現有 Taiwan endpoint helper 與主要 export 名稱保持相容，讓 feature components 不需因 transport replacement 大幅改寫。

## Frontend application shell

- 預設落地頁為「每日總覽」；主要工作區為每日總覽、市場、條件選股、自選股、持倉、事件提醒與個股研究，並保留設定入口。
- 研究歷史與前次比較留在個股研究 workflow；不復活任何已移除的 A-share route 或 navigation。
- 工作區間的個股導覽使用 canonical symbol（例如 `2330.TWSE`），不以裸代號取代。Screener、Watchlist、Portfolio 與 Alerts 都可導向同一研究入口。
- shell 提供載入、後端不可用、component error boundary 與窄螢幕導覽狀態；文案使用繁體中文。
- 全域樣式保持高密度金融資訊可讀性、明確 focus 狀態、可用對比、響應式導覽與現有 component class 的相容性；不要求與上游外觀 pixel-identical。

## Persisted product data

- 本階段不改 DB schema 或預設路徑。既有 `settings.json`、`taiwan-watchlist.db`、`taiwan-portfolio.db`、研究歷史、研究比較資料與 alert inbox/history 必須可直接繼續讀取。
- Phase B4 已將 Electron name、appId、productName 與 userData identity 切換至 mystocktracer。舊 profile 僅複製並驗證，來源保留；NSIS GUID 為既有安裝升級相容性保留，不代表使用舊 runtime。完整規則見桌面身份遷移文件。
