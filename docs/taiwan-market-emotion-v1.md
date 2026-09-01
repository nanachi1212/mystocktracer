# Taiwan Market Emotion v1

`taiwan_emotion_v1` 是可重現、可解釋的市場廣度解讀層。它直接消費 M2A `TaiwanMarketBreadth`，不重新抓取行情、不逐檔查詢，也不預測隔日漲跌。

## Inputs

v1 只使用 M2A 已驗證的 breadth facts：`advance_ratio`、`advance_decline_diff`、`advancing_amount_ratio`、上漲／下跌／平盤／無成交／未知家數、`universe_count`、`traded_count`、`total_amount_twd` 與 `missing_amount_count`。法人、融資融券與漲跌停家數不進入 v1 方向模型。

## Components and state

- Breadth Participation：`advance_ratio > 0.5` 為 positive，`< 0.5` 為 negative，等於 `0.5` 為 balanced。
- Capital Participation：`advancing_amount_ratio` 使用相同的數學多數分界。
- Breadth-Capital Relationship：分為 aligned positive、aligned negative、兩種 divergence、mixed 或 indeterminate。Divergence 只描述家數與成交金額方向不一致，不推論權值股或主力行為。
- State：兩個 ratio 都高於 `0.5` 為 positive，都低於 `0.5` 為 weak，其餘為 mixed；核心 ratio 缺失為 indeterminate，breadth unavailable 為 unavailable。

`0.5` 是上漲家數／金額是否過半的數學分界，不是歷史回測閾值。v1 不提供 0–100 總分，也不宣稱統計校準。

## Coverage and confidence

- `direction_coverage = (advancers + decliners + unchanged) / universe_count`
- `amount_coverage = (universe_count - missing_amount_count) / universe_count`
- `limit_rule_coverage = (universe_count - limit_unknown_count) / universe_count`，僅作稽核證據，不影響 v1 state。

95% 與 80% 是明示的 Taiwan Emotion v1 project policy thresholds，不是學術、統計或歷史校準：

- high：current，兩個核心 ratio 存在，direction 與 amount coverage 皆至少 95%。
- medium：兩個 ratio 存在，兩種 coverage 皆至少 80%。stale 或 partial 最高為 medium。
- low：其餘情況，包括 unavailable 或核心 ratio 缺失。

Confidence 不會改寫 state；`weak + low confidence` 是合法結果。

## Freshness, partial and PIT

Emotion 原樣繼承 M2A `as_of`、`target_latest_trading_date`、`status`、`freshness`、`included_exchanges` 與 `missing_exchanges`。TPEx-only 證據不會被標成 Taiwan combined current。

`unavailable` 表示上游 breadth dataset 不可用；`indeterminate` 表示 dataset 存在，但至少一個核心 ratio 缺失。Partial 是合法結果：API 仍會回傳已納入市場的 evidence，並透過 `included_exchanges` 與 `missing_exchanges` 明示它不是完整 Taiwan market。

v1 沒有另建 historical endpoint；若上游傳入歷史 M2A breadth，Emotion 只解讀該結果，不會查找或倒填其他日期。`classification_basis=current_reference` 原樣保留，因此不宣稱 historical universe fully PIT-safe。

## Known limitations

M2A 目前只暴露 `missing_amount_count`；缺少 snapshot row 與「方向未知但 amount 有效」都可能落在 unknown。因此 amount coverage 只是根據 M2A 明確報告 amount missing 建立的資料完整度 proxy，不是「成交金額市場覆蓋率」的完整統計證明。這是 v1 的明示限制，不以假值修補。法人、融資融券與 limit metrics scoring 皆不支援；`limit_rule_coverage` 僅是稽核證據。
