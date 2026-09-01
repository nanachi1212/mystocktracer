# Taiwan Industry Radar v1

`taiwan_industry_radar_v1` 是台灣上市、上櫃普通股的可稽核產業盤勢解讀層，不是預測、推薦或黑箱分數。

## 官方分類與身分

- TWSE 使用 Security Directory 的 `產業別` 代碼，名稱依 TWSE「上市公司產業類別劃分暨調整要點」。
- TPEx 使用 Security Directory 的 `SecuritiesIndustryCode`，名稱依 TPEx 官方產業類別表。
- 穩定身分是 `TWSE:<official-code>` 或 `TPEX:<official-code>`。原始 exchange、代碼、名稱與來源網址均保留；兩市場即使同碼同名也不強行合併。
- 未知或未支援代碼不猜測，計入 `unclassified_count`。`classified_count + unclassified_count = eligible_universe_count`。目前 TWSE directory 的特殊代碼 `91` 對應臺灣存託憑證而非一般公司產業；在既有 Security Master 尚未提供獨立 TDR type 前，v1 將其保留為 unclassified，不把「存託憑證」冒充普通股產業。

## Universe 與資料流

Universe 與 M2A `stock_breadth` 相同，只接受 `SecurityTypeStock`。ETF（含槓桿、反向）及未知類型 fail closed。每次請求只取得一次官方 directory 與一次各交易所 bulk daily snapshot，再於記憶體分組；不呼叫逐檔 Quote 或 KLine。

## 指標

每個產業保留 constituent、traded、advance、decline、unchanged、no-trade、unknown、amount 與 missing amount 原始計數。`advance_ratio = advancers / (advancers + decliners)`；`advancing_amount_ratio = advancing_amount / (advancing_amount + declining_amount)`；分母為零時回傳 `null`。

`relative_breadth` 是產業 `advance_ratio` 減目前 response scope 的 M2A market ratio；`relative_capital` 同理。每筆產業以 `benchmark_scope` 明示基準：TWSE view 使用 TWSE，TPEx view 使用 TPEX，COMBINED view 中所有 exchange-qualified industries 統一使用 COMBINED，不混用基準。正值只表示相對市場較高，不代表未來看多、法人買進或權值股拉抬。

## 排名與資料品質

預設排序是 `relative_breadth` 由高到低，`null` 置後，同值（包括兩個 `null`）以 `industry_id` 排序。這是明示的 v1 project policy，不是歷史最佳化、統計校準或預測模型，也沒有複合權重或 total score。

小樣本不刪除、不任意宣告無效。`data_quality` 直接揭露 `sample_size`、方向覆蓋率及金額覆蓋率，不使用未經統計校準的 high/medium/low 門檻。

## Freshness、partial 與 PIT

snapshot freshness、as-of、target、included/missing exchanges 直接繼承 M2A。單一交易所 snapshot unavailable 時該 section unavailable；combined 只包含可用交易所並標為 partial。Snapshot 與 taxonomy 分別由 `snapshot_status`、`taxonomy_status` 揭露，`status` 是整體結果；taxonomy 問題不會被誤列為 missing exchange。

COMBINED 是 TWSE 與 TPEx source industries 的並列集合，不是統一 taxonomy。例如 `TWSE:24` 與 `TPEX:24` 仍是兩個獨立身分，combined industry count 不代表臺灣存在同樣數量的統一產業類別。

分類基準固定揭露為 `current_reference`。歷史日期若使用目前 Security Directory，可能受公司改分類、上市下市及 survivorship/classification drift 影響，因此不宣稱 fully PIT-safe。本版沒有 historical endpoint。

## 限制

本版不使用 limit、法人、融資融券、leader/laggard、return、AI、預測或推薦，也沒有隱藏 total score。官方未對照的代碼（例如目前 directory 中的特殊分類）保持 unclassified。
