# Project Instructions

## Cross-Project Taiwan Ecosystem Rules

@~/.claude/rules/twstock-ecosystem.md

The shared rules above define the cross-project ownership boundary,
canonical Taiwan market-data ownership, anti-duplication principles,
Point-in-Time semantics, provenance requirements, and reuse-first policy.

## mystocktracer Repository Rules

Before implementing or modifying Taiwan-market-related functionality, also read:

`docs/TAIWAN_DEVELOPMENT_OWNERSHIP.md`

This repository-local document refines the shared ecosystem rules for
mystocktracer's existing architecture, migration strategy, fallback providers,
and product behavior.

## Rule Precedence

The two documents are complementary, not duplicate implementations.

- Cross-project canonical ownership and Taiwan market-truth semantics
  are defined by the shared ecosystem rules.
- mystocktracer-specific product behavior, migration details, adapters,
  fallbacks, and existing-provider preservation are refined by the
  repository-local ownership document.
- TWstock owns canonical Taiwan market truth and deterministic market data.
- mystocktracer owns product UX, Watchlist, Screener UX, Alerts,
  Notifications, AI Research presentation, and research workflows.
- Reuse shared facts before building new Taiwan market-data providers.
- Existing mature mystocktracer providers must not be deleted or rewritten
  without an explicit migration phase.
- Pure product/UI work does not require unnecessary cross-project data-layer
  investigation unless it touches canonical Taiwan market data, factors,
  normalization, provenance, PIT, or other shared market-truth domains.

If a future rule appears to conflict on canonical market-data ownership,
Point-in-Time semantics, provenance, deterministic factors, or duplicate
primary implementations, STOP and explicitly reconcile the rules before coding.
