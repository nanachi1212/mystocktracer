# Taiwan AI Research History and Comparison

## Architecture and ownership

Taiwan AI Research history is a mystocktracer product workflow. It stores the
research that the user actually generated and the bounded evidence presented to
that run. It is not canonical Taiwan market data and does not replace TWstock,
official facts, PIT rules, provenance, or reconciliation.

Successful runs reuse `BuildTaiwanResearchPayload` and its existing evidence
bounds. Raw provider responses, full HTML, and unrestricted third-party text are
never stored. Failed AI runs are returned to the UI but are not recorded as
successful history.

## Persistence and versioning

The existing `taiwan-watchlist.db` product-state database contains
`taiwan_ai_research_history`. Each row has a unique request-owned `run_id`,
canonical symbol, security name, timestamps, evidence as-of, research and
payload versions, available model metadata, bounded evidence JSON, structured
research JSON, provenance, validity, completeness, and stale/partial flags.

The migration is additive and transactional. History has no automatic retention
or deletion policy. Duplicate run IDs are idempotent, including concurrent
requests. Version dispatch recognizes `taiwan_ai_research_v1` and
`taiwan_ai_research_v2`; unknown versions return `not_comparable` instead of
being guessed or decoded as the current schema.

## APIs

- `POST /api/v1/tw/stocks/{symbol}/research` saves only an available result and
  accepts `X-Research-Run-ID` for idempotency.
- `GET /api/v1/tw/stocks/{symbol}/research-history` returns newest-first bounded
  summaries with `limit` and `offset`.
- `GET /api/v1/tw/stocks/{symbol}/research-history/{runID}` returns one
  structured result, evidence snapshot, provenance, validity, and versions.
- `GET /api/v1/tw/stocks/{symbol}/research-history/{runID}/comparison` compares
  that run with the immediately preceding successful run for the same symbol.

## Comparison semantics

The backend compares price, market, industry, institutional, margin/short,
fundamentals/revenue/valuation, corporate events, and AI sections. Domain states
are `unchanged`, `changed`, `newly_available`, `no_longer_available`, `stale`,
`partial`, or `unavailable`.

Numeric deltas are calculated only for an explicit same-field allowlist whose
units and meaning remain the same. Fiscal periods and unrelated values never
receive percentage deltas. Corporate events compare normalized event IDs.

Prior structured sections, strengths, and risks use conservative statuses:
`still_supported`, `weakened`, `contradicted_by_new_evidence`,
`insufficient_evidence`, or `not_comparable`. A contradiction is reported only
when the same evidence-key set moves between structured strength and risk
groups; free-text differences alone never create that claim. These states are
research comparison semantics, not Buy/Sell/Hold or other trading advice.

## UI and limitations

Taiwan Stock Research provides a Traditional Chinese history panel, a paged
persisted-run list, old-result viewing, return-to-latest action, evidence
completeness, and previous-run comparison. Text labels accompany every state;
color is not the only signal. The MVP loads summaries in bounded pages of 20
without deleting older history, and does not add generic cross-market history,
AI chat memory, portfolio integration, or background scheduling.
