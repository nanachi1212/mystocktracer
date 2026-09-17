# Taiwan corporate-event alert center MVP

The alert center is a product workflow over normalized corporate events. It does
not change provider ownership: TWstock/shared data remains the canonical owner
of Taiwan facts, while optional providers such as ToAlpha remain enrichment.

## Persistence and semantics

- The existing Taiwan Watchlist SQLite database stores provider-neutral inbox
  rows keyed uniquely by canonical symbol, provider, and normalized event ID.
- The first successful sync establishes a baseline and creates no alerts.
- Only unseen events published after the previous successful sync are eligible.
- Duplicate, late-arriving older, stale, partial, unavailable, and unsupported
  data never creates an inbox item.
- Disabling alerts still advances seen-event and successful-sync state. Turning
  alerts back on therefore observes only future eligible events and never
  backfills events seen while disabled.
- Read state and alert history survive application restarts. Removing a security
  from the Watchlist does not erase its existing alert history.

## Product surfaces

- `GET /api/v1/tw/alerts` lists a bounded, deterministically ordered inbox and
  returns the unread count and current preference.
- `PUT /api/v1/tw/alerts/{id}/read` marks one item read.
- `PUT /api/v1/tw/alerts/read-all` marks all unread items read.
- The Taiwan Watchlist loads the inbox independently from quotes and event sync,
  so an inbox API failure cannot hide or corrupt the Watchlist.
- The system settings page persists the corporate-event alert preference.

## Desktop notification follow-up

The Electron shell currently has no mature native-notification abstraction.
This MVP deliberately keeps the durable in-app inbox and does not add a new
cross-platform notification framework. A future phase may add native desktop
notifications after a tested abstraction exists; it must reuse inbox identity,
respect the preference, and never notify the same alert twice.
