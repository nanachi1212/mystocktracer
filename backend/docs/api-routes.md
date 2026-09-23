# mystocktracer 後端路由

這是現有 HTTP 邊界的快速索引；欄位、錯誤碼及大小限制以 `backend/internal/httpapi` 的 handler 與回歸測試為準。`GET /api/health` 可公開存取。若啟動時設定 `MYSTOCKTRACER_TOKEN`，其他路由需送 `Authorization: Bearer <token>`；WebSocket 可使用既有的 `?token=` 相容方式。舊 `A_STOCK_TOKEN` 僅作環境變數讀取 fallback。

## 官方台股資料

| 方法與路徑 | 用途 |
| --- | --- |
| `GET /api/v1/tw/securities?query=` | 查詢 TWSE／TPEx 證券目錄。 |
| `GET /api/v1/tw/quotes?symbols=` | 取得指定台股代碼的報價。 |
| `GET /api/v1/tw/kline?symbol=` | 日 K 與來源時間。 |
| `GET /api/v1/tw/indexes` | 市場指數快照及走勢。 |
| `GET /api/v1/tw/institutional?symbol=` | 三大法人流向。 |
| `GET /api/v1/tw/margin?symbol=` | 融資融券歷史。 |
| `GET /api/v1/tw/fundamentals?symbol=` | 月營收、財報、評價與股利。 |
| `GET /api/v1/tw/data-status` | 各資料領域的新鮮度與可用狀態。 |
| `GET /api/v1/tw/market-breadth?scope=` | 市場廣度。 |
| `GET /api/v1/tw/market-emotion?scope=` | 市場情緒。 |
| `GET /api/v1/tw/industry-radar?scope=` | 官方產業分類雷達。 |
| `GET /api/v1/tw/screener?scope=&sort=` | 官方市場快照篩選。 |

代碼使用如 `2330.TWSE`、`6488.TPEX` 的 canonical 形式；入口亦接受既有的數字或 `.TW`／`.TWO` 寫法並正規化。不可用資料保留其狀態，不能以零表示。

## 研究與使用者資料

| 方法與路徑 | 用途 |
| --- | --- |
| `GET /api/v1/tw/stocks/{symbol}/intelligence`、`/intelligence/core` | 完整及首屏官方證據。 |
| `POST /api/v1/tw/stocks/{symbol}/research` | 使用者明確要求的 AI 研究。 |
| `GET /api/v1/tw/stocks/{symbol}/research-history`、`/{runID}`、`/{runID}/comparison` | 歷次研究與前次比較。 |
| `GET /api/v1/tw/stocks/{symbol}/corporate-events` | 個股公司事件。 |
| `POST /api/v1/tw/corporate-events/sync` | 同步事件與提醒。 |
| `GET /api/v1/tw/dashboard` | 每日六區塊總覽，各區塊有自己的 `as_of`。 |
| `GET`、`POST /api/v1/tw/watchlist`；`DELETE /api/v1/tw/watchlist/{symbol}` | 自選股。 |
| `GET`、`POST /api/v1/tw/portfolio`；`PUT`、`DELETE /api/v1/tw/portfolio/{symbol}` | 持倉。 |
| `GET /api/v1/tw/portfolio/summary` | TWD 總額、損益、權重與集中度。 |
| `GET /api/v1/tw/alerts`；`PUT /api/v1/tw/alerts/read-all`、`/{id}/read` | 提醒收件匣與已讀狀態。 |

## 設定與 AI

- `GET`／`PUT /api/v1/settings`：模型設定、profile 與台股提醒偏好。回應只給密鑰是否已設定，不回傳密鑰；PUT 拒絕未知欄位與多個 JSON 物件。
- `GET`／`PUT /api/v1/settings/agent`：推理程度、Skills 與 MCP server。既有密鑰可在不重新提交時保留。
- `POST /api/v1/settings/llm/models`、`/api/v1/settings/llm/test`：取得模型清單、經隔離 runtime 測試連線。
- `GET /api/v1/ai/ws`：一般對話的產品 WebSocket 協定。前端不接收第三方 adapter 的原始訊框。

成功回應通常包含 `data`；失敗回應包含 `error`。市場資料的 `meta` 提供 `source`、`fetched_at`、`trade_date` 等出處與新鮮度欄位。具體 schema 及相容行為可參照 [B6 契約](../../docs/oss-b6-contract.md)。
