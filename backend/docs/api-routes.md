# Backend API Routes

Phase B1 removed the inherited A-share surface. Everything the backend serves now is either the
Taiwan product (`/api/v1/tw/…`), the market-neutral settings/AI plumbing, or the health check.

## Auth

`GET /api/health` is always public. Every other route requires a token when the server is started
with `A_STOCK_TOKEN`:

```text
Authorization: Bearer <token>
```

The AI WebSocket accepts the token as a query parameter:

```text
/api/v1/ai/ws?token=<token>
```

With no `A_STOCK_TOKEN` set, all local development routes stay open.

## Taiwan market data

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/v1/tw/securities?query=2330` | Cached TWSE/TPEx official security directory; searches code, canonical symbol, short name, or full name. |
| `GET` | `/api/v1/tw/quotes?symbols=2330.TWSE` | Official quotes for canonical Taiwan symbols. |
| `GET` | `/api/v1/tw/kline?symbol=2330.TWSE` | Daily OHLCV history. |
| `GET` | `/api/v1/tw/indexes` | Taiwan index snapshots and series. |
| `GET` | `/api/v1/tw/institutional?symbol=2330.TWSE` | Three-institution net flow history. |
| `GET` | `/api/v1/tw/margin?symbol=2330.TWSE` | Margin financing / securities-lending history. |
| `GET` | `/api/v1/tw/fundamentals?symbol=2330.TWSE` | Monthly revenue, financial statements, valuation and dividends. |
| `GET` | `/api/v1/tw/data-status` | Per-domain freshness for the Taiwan data layer. |
| `GET` | `/api/v1/tw/market-breadth?scope=combined` | Advancers/decliners and traded amount direction. |
| `GET` | `/api/v1/tw/market-emotion?scope=twse` | Deterministic market participation and signal-divergence view. |
| `GET` | `/api/v1/tw/industry-radar?scope=tpex` | Relative breadth by official industry classification. |
| `GET` | `/api/v1/tw/screener?scope=combined&sort=amount` | Filter and sort securities from the official market snapshot. |

## Taiwan research

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/v1/tw/stocks/{symbol}/intelligence` | Full official-evidence view for one security. |
| `GET` | `/api/v1/tw/stocks/{symbol}/intelligence/core` | First-paint subset of the above. |
| `POST` | `/api/v1/tw/stocks/{symbol}/research` | Generate one grounded AI research run. Always user-triggered; never called automatically. |
| `GET` | `/api/v1/tw/stocks/{symbol}/research-history` | List saved research runs for one security. |
| `GET` | `/api/v1/tw/stocks/{symbol}/research-history/{runID}` | Read one saved run. |
| `GET` | `/api/v1/tw/stocks/{symbol}/research-history/{runID}/comparison` | Compare one run against the previous run's evidence. |
| `GET` | `/api/v1/tw/stocks/{symbol}/corporate-events` | Corporate-event feed for one security. |
| `POST` | `/api/v1/tw/corporate-events/sync` | Sync corporate events and create alerts. |

## Taiwan product state

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/v1/tw/dashboard` | Daily dashboard: market, holdings, watchlist, alerts, research updates, needs-attention. Each section keeps its own `as_of`. |
| `GET` `POST` | `/api/v1/tw/watchlist` | List or add saved securities. |
| `DELETE` | `/api/v1/tw/watchlist/{symbol}` | Remove one saved security. |
| `GET` `POST` | `/api/v1/tw/portfolio` | List holdings, or add/idempotently update one canonical holding. |
| `PUT` `DELETE` | `/api/v1/tw/portfolio/{symbol}` | Update or delete one holding without changing Watchlist or alert history. |
| `GET` | `/api/v1/tw/portfolio/summary` | TWD totals, P/L, weights and descriptive concentration. |
| `GET` | `/api/v1/tw/alerts` | Corporate-event alert inbox. |
| `PUT` | `/api/v1/tw/alerts/read-all` | Mark every alert read. |
| `PUT` | `/api/v1/tw/alerts/{id}/read` | Mark one alert read. |

## Settings, AI and health

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/health` | Backend health check. |
| `GET` `PUT` | `/api/v1/settings` | Read or update model settings and Taiwan alert preferences. Secrets are never returned; only a masked/configured status. |
| `GET` `PUT` | `/api/v1/settings/agent` | Read or update Hermes agent settings (reasoning effort, skills, MCP servers). |
| `POST` | `/api/v1/settings/llm/models` | Query the configured model service for its model list. |
| `POST` | `/api/v1/settings/llm/test` | Send a minimal prompt through Hermes to verify the model connection. |
| `GET` | `/api/v1/ai/ws` | WebSocket bridge to the local Hermes runtime for the general AI chat workspace. |

## Response shape

Most successful data routes return:

```json
{
  "data": [
    {
      "symbol": "2330.TWSE",
      "price": 1050,
      "meta": {
        "source": "twse",
        "source_url": "https://openapi.twse.com.tw/...",
        "fetched_at": "2026-09-18T14:30:01+08:00",
        "latency_ms": 188,
        "trade_date": "2026-09-18",
        "stale": false
      }
    }
  ]
}
```

Errors return:

```json
{
  "error": "symbols is required"
}
```

Unavailable data is reported as unavailable — it is never returned as `0`. Every response keeps its
own source, trade date and freshness so callers can tell `available`, `stale`, `partial`,
`unavailable` and "not queried" apart.

## Symbols

Taiwan symbols are canonicalized to `<code>.<exchange>`:

| Input | Canonical |
| --- | --- |
| `2330` | `2330.TWSE` |
| `2330.TW` | `2330.TWSE` |
| `6488` | `6488.TPEX` |
| `6488.TWO` | `6488.TPEX` |
