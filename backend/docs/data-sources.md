# Taiwan Data Sources

本文件記錄 mystocktracer 台股資料層的來源、單位、Point-in-Time（PIT，當時可得資料）與降級邊界。TWstock／交易所官方資料擁有台灣市場真值；第三方來源只能補充、驗證或 fallback。

## 已實作來源

| 來源 | Domain | API | 資料 |
| --- | --- | --- | --- |
| TWSE OpenAPI | `openapi.twse.com.tw` | `/api/v1/tw/securities` | 上市公司與基金名錄 |
| TPEx OpenAPI | `www.tpex.org.tw` | `/api/v1/tw/securities` | 上櫃公司名錄 |
| TWSE OpenAPI / exchange report | `openapi.twse.com.tw`、`www.twse.com.tw` | `/api/v1/tw/quotes`、`/api/v1/tw/kline`、`/api/v1/tw/indexes` | 官方收盤報價、日 K 與加權指數 |
| TPEx OpenAPI / after trading | `www.tpex.org.tw` | `/api/v1/tw/quotes`、`/api/v1/tw/kline`、`/api/v1/tw/indexes` | 官方收盤報價、日 K 與櫃買指數 |
| TWSE T86 / MI_MARGN | `www.twse.com.tw` | `/api/v1/tw/institutional`、`/api/v1/tw/margin` | 上市三大法人與融資融券 |
| TPEx dailyTrade / margin balance | `www.tpex.org.tw` | `/api/v1/tw/institutional`、`/api/v1/tw/margin` | 上櫃三大法人與融資融券 |
| TWSE / TPEx MOPS OpenAPI | `openapi.twse.com.tw`、`www.tpex.org.tw/openapi` | `/api/v1/tw/fundamentals` | 月營收、財報、評價與股利決議 |
| FinMind | `api.finmindtrade.com` | `/api/v1/tw/fundamentals` | 僅補月營收歷史；標記 `third_party_fallback`，同期官方資料優先 |
| ToAlpha MOPS | `toalpha.tw/mcp/mops` | `/api/v1/tw/stocks/{symbol}/corporate-events` | 可選公司事件補充；不取代官方／共用 canonical facts |

## 正規化與 PIT 規則

- 官方收盤價與指數明確設定 `is_realtime=false`；每筆資料保留 `source`、`source_url`、`trade_date`、`fetched_at`、`stale` 與可用狀態。
- 報價失敗只能 fallback 到同交易所的官方月報；錯誤、過期、部分與未查詢不得偽裝成值 0 或「沒有資料」。
- 法人買賣超以股為單位，不縮放。TWSE／TPEx 融資融券報表的張數乘以 1,000 轉為股，metadata 保留 `raw_unit:lots` 與 `lot_multiplier:1000`。
- 官方財務金額由仟元轉為 TWD，同時保留 raw value 與 raw unit。若 OpenAPI 無法確認公司實際公告時間，`published_at` 與 `available_at` 必須留空，不得以 `period_end` 或 feed 匯出日代替。
- ETF 不支援公司月營收與財報；官方評價若未包含 ETF，回報 `data_insufficient`，不使整個 bundle 失敗。

## Cache 與降級

- 台股名錄 cache 12 小時；日頻官方 response 依完整 source URL 在記憶體 cache 6 小時，重啟後清空。
- 基本面 process-local cache：月營收／評價 1 日，財報／股利 7 日；網路請求不得在持有 cache mutex 時執行。
- ToAlpha 只提供有界限的正規化事件證據。`partial` 或 `unavailable` 不得污染成功 cache 或 `last_successful_sync`。

## 來源可靠性

- 每個 provider 需有 mocked unit tests；連外網的 provider 需提供 opt-in live tests。
- Live test 失敗需區分 HTTP status、parse、auth、no data 與 timeout。
- Consumer 必須讀取 response provenance，不可假設固定 upstream。
