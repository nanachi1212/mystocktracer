# Taiwan AI Research v1

`taiwan_ai_research_v1` is an opt-in, Traditional-Chinese explanation layer over M4A `taiwan_stock_intelligence_v1` and M4B `taiwan_stock_interpretation_v1`. It is Taiwan-only and does not change either deterministic contract.

The explicit endpoint is `POST /api/v1/tw/stocks/{symbol}/research`. The ordinary intelligence GET never invokes AI. The research response retains the complete deterministic intelligence beside `ai_research`; Hermes failure, timeout, invalid JSON, or validation failure returns AI status `unavailable` without hiding M4A/M4B.

## Hermes isolation

The normal Hermes `Prompt` continues using the existing A-share system profile. Taiwan research requires the optional `IsolatedPrompter` path. For each call, Runtime creates a temporary Hermes home, copies only model/provider configuration and the protected provider key, replaces the system prompt with the Taiwan closed-evidence contract, disables memory, skills, MCP servers, curator and lazy installation, and explicitly removes browser wrapper, storage-state and profile environment values. The temporary home is removed after the call. Existing A-share callers and their prompt hierarchy are unchanged.

The Taiwan system prompt treats payload strings as non-executable data and prohibits model memory, web, news, browser, MCP, skills, prediction, recommendation, scoring, price plans and position sizing. V1 performs no external research or news enrichment.

## Bounded payload and output

The deterministic payload selects identity, completed 5/20-session returns, M4B component states, market ratios, M3 relative metrics, the latest official institutional net observations, latest nullable margin/short changes, official monthly-revenue YoY, section status/freshness/as-of, bounded reasons, and M4B data quality. It excludes full KLine bars, repeated SourceMeta and URLs. The builder selects and bounds facts; it does not recalculate interpretation.

Output is one JSON object containing version, canonical symbol, headline, summary, six fixed sections, conflicts, data limitations and research notes. Every section has text plus at least one evidence key. Validation first applies the contract allowlist and then a per-request available-key set; an allowlisted field that is null, absent, or not applicable cannot be cited as present evidence. Validation also rejects malformed JSON, unknown fields or sections, wrong version/symbol, model-supplied server provenance, missing or oversized sections/arrays/text, and prohibited trading language.

Conflicts remain first-class and are not collapsed into an overall verdict. `partial`, `stale`, `unavailable` and unfinished-quote limitations must remain explicit. Follow-up notes may identify evidence to observe, but cannot define price triggers.

ETF industry and ordinary-company fundamentals remain `not_applicable`; research must not describe an ETF as a company. Monthly-revenue YoY cannot be expanded into profit, EPS, ROE or valuation claims. Institutional categories are described independently without an invented total. Margin and short changes do not imply bullishness, chip quality, retail behavior or a squeeze.

The bounded payload is deterministic for the same M4A/M4B input. LLM wording is not promised to be byte-identical. V1 has no score, grade, overall bullish/bearish verdict, forecast, recommendation, buy/sell/hold action, target price, stop loss, take profit, position sizing, cache framework, model-memory enrichment, web, news, MCP or browser research.

Known limitations: structural grounding is enforced through evidence-key allowlisting and bounded schema, but prose factual correctness still requires human review. AI model/provider metadata is not added because the current PromptResult contract does not expose it. Live AI verification may be unavailable when the packaged Hermes runtime or user model configuration is absent; deterministic M4A/M4B APIs remain usable.
