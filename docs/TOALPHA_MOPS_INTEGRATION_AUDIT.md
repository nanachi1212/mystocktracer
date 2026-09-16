# ToAlpha MOPS integration audit

Audit date: 2026-09-16
ToAlpha documentation reviewed: [ToAlpha MOPS MCP](https://toalpha.tw/mcp/mops), last updated 2026-09-15
TWstock reference reviewed: `nanachi1212/TWstockfor_tick-stock-panel` `main` at `fd79b634c17f76144bac5828708603eb65784684`

## Decision

ToAlpha is an optional third-party enrichment and verification source. It is not a canonical Taiwan market-data provider and does not replace the existing TWSE, TPEx, MOPS, FinMind, or shared TWstock paths.

The first production use case is the recent MOPS material-announcement feed. It fills a product-facing event gap while preserving TWstock as the owner of deterministic Taiwan market truth. ToAlpha-produced category and `important` fields are explicitly third-party enrichment, not official MOPS facts.

## MCP protocol audit

| Item | Verified behavior |
| --- | --- |
| Transport | MCP Streamable HTTP |
| Endpoint | `https://toalpha.tw/mcp/mops` |
| Authentication | Public endpoint; no account or API key documented |
| Protocol | Initialization negotiated `2025-03-26` |
| Server | `toalpha-mops` `1.30.0` |
| Session | `initialize` returns `Mcp-Session-Id`; client sends `notifications/initialized`, then `tools/call` |
| Response framing | `text/event-stream`; JSON-RPC responses arrive in `event: message` / `data:` records |
| Tool output | `result.content[]` text blocks containing JSON. No MCP `outputSchema` is advertised, so the adapter validates the observed JSON itself. |
| Errors | Tool errors return a normal JSON-RPC result with `isError: true` and text content. Transport and HTTP failures remain separate errors. |
| Pagination | No cursor protocol is advertised. Bounded `limit` parameters are used instead. `material_news` allows up to 100 rows. |
| Dates | `material_news.date` and `time` are the issuer announcement time in Asia/Taipei; `fact_date` is a date-only event fact. |
| Updates | Material-news list is documented as updating every 10 minutes. Historical full text is still being backfilled. Other datasets are generally daily or filing-cycle based. |
| Rate limits | The site says daily tool counts are recorded for rate limiting, but publishes no numeric quota. The integration therefore uses a bounded cache, timeout, and at most one retry. |
| Privacy | The site says it records tool name and daily count, not query content. Requests still disclose the queried public stock code to ToAlpha. |

The selected `material_news` input schema is `stock_id` (required string), `days` (bounded integer), and `limit` (bounded integer). The adapter requests at most 12 rows over 90 days. Its observed text-JSON output contains stock identity, `count`, announcement rows (`date`, `time`, `seq`, `subject`, `category`, `important`, `fact_date`, `excerpt`, `full_text`), update time, source, and link. The MCP server advertises no formal output schema, so unknown, missing, trailing, or malformed structures fail closed. A live smoke test on 2026-09-16 confirmed the public endpoint was available for `2330.TWSE`.

Runtime discovery on 2026-09-16 returned 17 tools:

`resolve_stock`, `company_profile`, `financial_statements`, `financial_ratios`, `monthly_revenue`, `dividend_policy`, `investor_conferences`, `insider_holdings`, `material_news`, `material_news_detail`, `insider_trades`, `audit_opinions`, `dividend_dates`, `overseas_investments`, `equity_changes`, `company_documents`, and `data_status`.

The implemented adapter deliberately calls only `material_news`. It does not expose an arbitrary MCP tool-call surface to product code or AI.

## Capability and ownership matrix

| Capability | Existing/canonical capability | Decision | Source priority and notes |
| --- | --- | --- | --- |
| Security/company resolution | TWstock Security Master and mystocktracer Taiwan directory | `canonical_reuse` | TWstock/shared identity first; ToAlpha `resolve_stock` and `company_profile` rejected as duplicate primary sources. |
| Monthly revenue | TWstock and `providers/taiwan/fundamentals.go` official bulk data | `toalpha_verification_only` | Never replaces the canonical official value. Not implemented in this phase. |
| Financial statements | TWstock and mature mystocktracer MOPS/XBRL paths | `toalpha_verification_only` | Duplicate primary pipeline rejected. |
| Financial ratios | Canonical deterministic facts belong in TWstock/shared data | `toalpha_verification_only` | ToAlpha-derived ratios must not become canonical factors. Not implemented. |
| Dividends | TWstock contract v1.1 and official dividend lifecycle events; mystocktracer official aggregates | `toalpha_verification_only` | ToAlpha `dividend_policy` / `dividend_dates` do not override official lifecycle records. |
| Material announcements | TWstock has dividend-specific MOPS events but no general corporate-event feed | `toalpha_product_enrichment` | Implement recent events as bounded, provenance-safe product evidence. General official event ingest remains a TWstock follow-up. |
| Investor conferences | Product-relevant, but also a deterministic filing domain without a shared canonical feed | `canonical_gap_followup_required` | Do not silently make ToAlpha canonical. May later be added as marked third-party enrichment after a separate product decision. |
| Insider holdings | Taiwan market truth; no canonical shared implementation found | `canonical_gap_followup_required` | ToAlpha may be used for future verification/enrichment only, with explicit third-party status. |
| Insider transactions | Taiwan market truth; no canonical shared implementation found | `canonical_gap_followup_required` | Not implemented here. |
| Audit opinions | Taiwan financial-report truth; no reusable shared normalized stream found | `canonical_gap_followup_required` | Not implemented here. |
| Overseas investments | Taiwan filing truth; no reusable shared normalized stream found | `canonical_gap_followup_required` | Not implemented here. |
| Corporate events | TWstock owns canonical events; mystocktracer owns watchlist, portfolio, alerts, and presentation | `toalpha_product_enrichment` | Implement only normalized recent announcements with explicit ToAlpha/MOPS provenance. |
| Equity changes / company documents | Deterministic filing records | `canonical_gap_followup_required` | Not implemented here. |

## Source priority

1. Shared/canonical TWstock facts and contracts.
2. Existing mystocktracer official TWSE/TPEx/MOPS providers while the shared layer is unavailable.
3. ToAlpha only for the approved material-announcement enrichment path.
4. Explicit `unavailable`, `partial`, `stale`, `not_queried`, or `no_events`; never a fabricated zero or an empty list presented as success.

No ToAlpha response can overwrite canonical monthly revenue, financial statements, valuation, dividends, or cash flow. There is no `last response wins` merge.

## Implemented boundary

- A single `backend/internal/providers/toalpha` integration boundary using the Go standard library.
- Configuration: `A_STOCK_TOALPHA_MOPS_ENABLED` and `A_STOCK_TOALPHA_MOPS_ENDPOINT`.
- Disabled by default; startup and all existing Taiwan evidence remain functional when disabled or unavailable.
- A normalized `foundation.TaiwanCorporateEvent` model preserves provider, source, URL, retrieval time, publication time, status, partial/stale state, and reason.
- Stable identity prefers the MOPS issuer/date/sequence tuple exposed by ToAlpha. A deterministic hash fallback is used only when the tuple is incomplete.
- ToAlpha categories and importance are marked `third_party_enrichment`.
- A small in-memory TTL cache reduces third-party calls; cache failure never affects canonical providers.
- Event-change state reuses the existing Taiwan watchlist SQLite database rather than creating another database.
- AI receives only a bounded normalized event payload. It cannot call MCP and cannot see raw MCP responses.

## Runtime configuration and product surfaces

The integration is opt-in and disabled by default:

```text
A_STOCK_TOALPHA_MOPS_ENABLED=true
A_STOCK_TOALPHA_MOPS_ENDPOINT=https://toalpha.tw/mcp/mops
```

Restart the backend after changing these process environment variables. No API key or browser cookie is used. Disabling the flag produces `not_queried`; unsupported non-stock securities produce `unsupported`; a timeout, transport failure, malformed payload, or MCP tool error produces `unavailable` unless an expired cache entry can be returned explicitly as `stale`. A successful empty query produces `no_events`. These states are never collapsed into one another.

The normal offline suite never calls ToAlpha. The opt-in live smoke test is:

```powershell
$env:A_STOCK_LIVE_TEST='1'
go test ./internal/providers/toalpha -run Live -v
```

- `GET /api/v1/tw/stocks/{canonical}/corporate-events` reads the bounded recent feed for one Taiwan stock.
- `POST /api/v1/tw/corporate-events/sync` accepts up to 20 canonical `.TWSE` / `.TPEX` stock symbols. An omitted or empty list syncs the saved Taiwan watchlist.
- The Taiwan Research workspace renders the normalized feed before any AI action.
- The Taiwan Watchlist invokes the persistent change detector when that workspace loads or is explicitly refreshed. First observation establishes a baseline and does not notify; only later unseen events published after the last successful sync are returned as new.
- The sync endpoint also accepts an explicit symbol list for a future or external Taiwan portfolio consumer. The repository's existing Portfolio Inspection feature is China A-share specific, so this change deliberately does not mix Taiwan symbols into that unrelated contract.
- `taiwan_ai_research_v2` receives no more than eight normalized events, with per-field and aggregate size bounds. Event text is marked untrusted and can only be cited through the closed evidence allowlist. `BuildTaiwanResearchPayloadV1` remains available for explicit v1 contract compatibility.

## Rejected duplicate implementations

- A second monthly-revenue provider.
- A second financial-statement or cash-flow pipeline.
- A second valuation or dividend truth source.
- A general-purpose MCP proxy exposed to the AI model.
- Persisting ToAlpha raw responses or building another Taiwan historical warehouse.
- Treating ToAlpha category, importance, summary, or ranking as an official MOPS fact.

## Known limitations

- ToAlpha does not publish a numeric rate limit.
- MCP tool output has no advertised output schema; the adapter must reject malformed or changed shapes.
- Historical material-announcement full text is still being backfilled, so `full_text: false` is represented as partial evidence.
- The material-news tool is limit-based rather than cursor-paginated. The product fetch is intentionally bounded and does not claim exhaustive history.
- ToAlpha is a third party. Important facts must remain traceable to the returned MOPS source link and should be verified against the official announcement for high-stakes use.
