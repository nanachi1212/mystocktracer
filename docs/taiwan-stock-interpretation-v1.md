# Taiwan Stock Interpretation v1

`taiwan_stock_interpretation_v1` is a deterministic interpretation policy over the existing M4A `TaiwanStockIntelligence` evidence. The calculator performs no network I/O and does not mutate or replace raw evidence.

## Components

- **Price** uses completed-KLine `return_5d_percent` and `return_20d_percent`. Both positive is `positive`; both negative is `negative`; different signs or either value exactly zero is `mixed`; a missing value is `indeterminate`. This is signed-direction consistency project policy, not historical calibration. An unfinished or future-dated quote is not used.
- **Market** preserves the M2B state, confidence, status, freshness, and as-of semantics. M4B does not recalculate emotion or alter its thresholds.
- **Price/market relationship** describes alignment only. It is not outperformance or relative strength because no market price-return benchmark is present.
- **Industry** compares M3 `relative_breadth` and `relative_capital` with the exact mathematical zero boundary. Both positive is `supportive`, both negative is `weak`, and different signs or equality to the benchmark is `mixed`. Missing metrics are `indeterminate`.
- **Institutional** reports the latest trade date's official foreign, investment-trust, and dealer net fields independently as `net_buy`, `net_sell`, or `neutral`. The official TWSE/TPEx parser treats these three categories as one atomic observation and rejects the entire row if any required field is missing or malformed. M4B additionally requires the row provenance to mark official `raw_net` fields; without that proof the component is `indeterminate`. An explicit official zero is neutral and is not treated as missing. No unsupported aggregate, multi-day window, bullish meaning, or smart-money meaning is inferred. The component is only direction-consistent when all three categories agree; otherwise it is `mixed`.
- **Margin** reports latest official `margin_change` and `short_change` independently as increased, decreased, or unchanged. It does not infer retail behavior, short squeezes, or price direction.
- **Fundamentals** only interprets the latest available official monthly-revenue YoY sign as `positive`, `negative`, or `flat`. It does not grade valuation, profitability, ROE, PE, PB, or dividend yield. Missing official YoY is `indeterminate`.

Every component contains separate `state` and evidence `status`, deterministic `reasons`, and machine-readable `evidence_keys`. Freshness and as-of are inherited from the relevant M4A section. Components in one response may have different legal timestamps and must not be treated as one common PIT timestamp. Fundamentals use only the official monthly-revenue YoY pointer already admitted by M4A: explicit zero is `flat`, while missing YoY or an unavailable section is `indeterminate`. They retain the strict `query_at > available_at` rule and historical universe classification remains `current_reference` where inherited.

ETFs may use completed price and market components. Ordinary-stock fundamentals and industry interpretation are `not_applicable`; leveraged and inverse ETF price direction is not company analysis.

`data_quality` lists available, indeterminate, unavailable, stale, and partial component names. Availability is the primary classification; stale and partial are independent modifiers, so a component may intentionally appear in both `available_components` and `stale_components` or `partial_components`. The lists have no hidden numeric score or high/medium/low threshold.

The existing intelligence endpoint includes interpretation alongside the complete raw M4A evidence:

`GET /api/v1/tw/stocks/{symbol}/intelligence`

Version 1 intentionally provides no overall verdict, score, grade, prediction, recommendation, buy/sell action, target price, position sizing, stop loss, take profit, AI, LLM narrative, or news synthesis.
