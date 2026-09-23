# mystocktracer Repository Instructions

These repository rules supplement the global Codex instructions. They do not
repeat general security, Git, testing, communication, or Windows rules.

## Repository Identity and Branch Governance

- `origin/main` is the canonical mystocktracer product branch.
- `upstream/main` is the easy-stock upstream reference only.
- Do not pull, merge, or rebase `upstream/main` into this repository without
  explicit review and approval.
- Before starting work, check the current branch, working tree, and remote
  URLs. Do not select a remote solely from Git tracking configuration.
- Do not push to any remote unless the user explicitly requests it.

## Taiwan Ecosystem Ownership

- 本 repository 採 OSS transition / maintenance，維護既有行為與資料。
  `TWstockfor_tick-stock-panel` 是唯一長期台股主產品；新台股核心功能優先放在該專案，
  不在此建立平行功能主線。下列 ownership 僅約束既有程式維護，不授權產品擴張。
  技術獨立化仍以 `docs/oss-provenance-audit.md` 的 gate 為準，不能由維護定位推定完成。

- TWstock owns canonical Taiwan market truth and deterministic market data.
- mystocktracer owns product UX, Watchlist, Screener, Alerts, Notifications,
  AI Research presentation, and research workflows.
- Before modifying Taiwan market data, normalization, Point-in-Time (PIT)
  behavior, or provenance, read
  `docs/TAIWAN_DEVELOPMENT_OWNERSHIP.md`.

## Reuse-First and Migration Safety

- Reuse shared facts, existing contracts, and mature providers before adding
  a new Taiwan market-data implementation.
- Do not delete or rewrite an existing mature provider without an explicit
  migration phase.
- Preserve fallback behavior, compatibility, existing user data, and
  recoverability during migrations.

## PIT and Provenance Safety

- Preserve Point-in-Time semantics and do not introduce future-data leakage.
- Preserve source attribution, timestamps, scope, and provenance for market
  data and derived facts.
- If ownership, PIT, provenance, or duplicate-primary-implementation rules
  conflict, stop and clarify the conflict before coding.
