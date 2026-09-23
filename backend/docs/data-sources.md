# 台股來源與時間邊界

mystocktracer 讀取 TWSE、TPEx、MOPS 等官方資料，並以共用台股權責為 canonical 事實基礎。資料來源的完整開發規則見 [台股開發權責](../../docs/TAIWAN_DEVELOPMENT_OWNERSHIP.md)。此文件只描述目前 backend 已連接的來源。

| 資料 | 主要來源 | 接口領域 |
| --- | --- | --- |
| 證券名錄、收盤行情、日 K 與指數 | TWSE／TPEx 官方 OpenAPI 或交易後報表 | `/api/v1/tw/securities`、`quotes`、`kline`、`indexes` |
| 法人買賣超、融資融券 | TWSE T86／MI_MARGN、TPEx 對應交易後資料 | `/api/v1/tw/institutional`、`margin` |
| 月營收、財報、評價、股利 | TWSE／TPEx 與 MOPS 官方資料 | `/api/v1/tw/fundamentals` |
| 月營收歷史補充 | FinMind，僅第三方 fallback | `/api/v1/tw/fundamentals` |
| 公司事件補充 | 可選的 ToAlpha MOPS | `/api/v1/tw/stocks/{symbol}/corporate-events` |

價格與日 K 保留官方交易日、擷取時間、來源 URL 及是否過期；官方收盤資料標為非即時。報價失敗時，只能依既有規則使用同交易所官方月報作降級。法人數值以股計；張數來源須乘 1,000 並保留原始單位證據。官方財務金額若從仟元換算，仍須保留 raw value 與 raw unit。

歷史資料遵守 Point-in-Time：沒有可靠公告時間時，`published_at`／`available_at` 不得依期末日猜測。ETF 無公司月營收或財報時回報能力不足；不能填零。`available`、`stale`、`partial`、`unavailable` 和未查詢須保持可區分。

ToAlpha 只提供有界的事件佐證；`partial` 或 `unavailable` 回應不得覆蓋官方事實、成功 cache 或成功同步時間。provider 回歸以合成資料測試；連線真實端點需明確啟用 [live tests](live-tests.md)。
