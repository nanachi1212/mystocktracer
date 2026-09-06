# Project Instructions

## Taiwan Market Development Rules

Before implementing or modifying any Taiwan-market-related feature, read:

`docs/TAIWAN_DEVELOPMENT_OWNERSHIP.md`

Treat that document as the authoritative project ownership and anti-duplication policy.

Key rule:

- TWstock owns canonical Taiwan market truth and deterministic market data.
- mystocktracer owns product UX, Watchlist, Screener UX, Alerts, Notifications, AI Research, and research workflows.
- Reuse shared facts before building new Taiwan market-data providers.
- Do not create duplicate primary implementations when TWstock already owns the capability.
- Existing mature mystocktracer providers must not be deleted or rewritten without an explicit migration phase.
- Pure UI / Watchlist / product UX work does not require the cross-project investigation below.
- Any change that touches Taiwan market-layer data (not just product UX) must first perform the mandatory anti-duplication investigation defined in the ownership document.
