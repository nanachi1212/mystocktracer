# 台股今日總覽（Taiwan Daily Dashboard MVP）

## 架構與 ownership

Dashboard 是 `mystocktracer` 的產品聚合與導覽層，只讀取既有 Taiwan market、Portfolio、Watchlist、Alert Center 與 AI Research History。TWstock／官方 provider 仍擁有 canonical Taiwan facts；Dashboard 不建立第二套行情、事件、持倉、自選股或研究歷史。

## Aggregation API

`GET /api/v1/tw/dashboard` 回傳 bounded、product-owned DTO：

- `market`：主要指數、市場廣度、情緒、成交額與前五個產業。
- `portfolio`：市值／未實現損益、價格涵蓋、最大持股、Top 3、前三產業與五筆最大絕對漲跌。
- `watchlist`：價格涵蓋與五筆最大絕對漲跌。
- `alerts`：未讀數與最近五筆未讀事件，沿用相同 alert record/read state。
- `research`：最近五筆既有成功研究與可用的 previous comparison 摘要；讀取 Dashboard 不觸發 AI／LLM。
- `attention`：最多八筆 deterministic product-attention 項目。

各 section 獨立執行與降級；單一 provider／SQLite section 失敗仍回 HTTP 200，失敗區顯示 `unavailable`，其他區保留結果。Portfolio 與 Watchlist 的重疊 symbol 在同一 request 內共用 quote；行情 fan-out 使用既有四路 bounded concurrency、request context、45 秒總 timeout。

## 狀態與 freshness

Section status 為 `available`、`stale`、`partial`、`unavailable` 或 `not_queried`。缺少資料維持 `null`／unavailable，不轉成 `0`。每一區保留自己的 `as_of`；行情項目也保留各自 `as_of`，`generated_at` 只代表聚合時間，不代表全站資料最新時間。

手動 Refresh 重新取得 market、Watchlist 與 Portfolio market enrichment，並呼叫既有 Watchlist corporate-event sync；event provider 失敗不阻止其他區更新。初次載入不自動觸發 event sync，不輪詢，也不觸發 AI Research。

## Need Attention

Priority 是產品注意順序，不是投資優先順序：

1. `market_data_unavailable`（100）
2. `stale_market_data`（95）
3. `partial_data`（90；研究資料不完整為 65）
4. `price_unavailable`（85）
5. `new_corporate_event`（80）
6. `research_evidence_changed`（60）
7. `portfolio_concentration`（40，最大單一持股達 50% 時顯示的純描述性資訊）

同 priority 依 reason code、canonical symbol 排序。所有項目包含 `reason_code`、`priority`、`title`、`target`，股票項目另含 canonical symbol。

## UI 與導覽

`#taiwan-dashboard` 是台股預設首頁，以 summary cards 與 bounded lists 顯示【今日市場】【我的持股】【自選股】【事件提醒】【研究更新】【需要注意】。可一鍵前往市場、Portfolio、Watchlist、Alert Center、Stock Research；股票連結直接帶 canonical symbol。Dashboard 的「標記已讀」直接更新既有 `/api/v1/tw/alerts/{id}/read`。

前端以 request revision 防止舊回應覆蓋新 refresh。所有第三方 title/research 內容以 React text rendering 顯示，不使用 trusted HTML；alert source URL 後端只允許 `http`／`https`。

## 限制

- 最多聚合 100 個 Portfolio／Watchlist 不重複 symbols；UI list 維持 5 筆，attention 維持 8 筆。
- Corporate-event 手動 refresh 使用既有 Watchlist sync；Portfolio workspace 仍負責其既有持股事件同步流程。
- 不含投資推薦、Buy/Sell/Hold、目標價、自動每日 AI 分析、背景 LLM、broker／交易或 desktop notification。
