# Taiwan AI research v2: bounded corporate-event evidence

`taiwan_ai_research_v2` extends the existing closed-evidence research contract with a seventh `corporate_events` section. It does not give the model network, browser, MCP, skill, or tool access. ToAlpha is called by the product-owned provider boundary before AI generation; the model sees only normalized JSON.

## Compatibility

- `TaiwanAIResearchVersionV1` and `BuildTaiwanResearchPayloadV1` preserve the pre-event payload for explicit legacy consumers and regression tests.
- The active generator emits `taiwan_ai_research_v2` and requires exactly seven output sections: `price`, `market`, `industry`, `institutional`, `margin`, `fundamentals`, and `corporate_events`.
- Existing official evidence keys and validation rules are unchanged.

## Event evidence bounds

- At most eight normalized events enter the payload.
- Titles are limited to 160 Unicode code points and details to 320.
- The combined encoded event items are limited to 6,000 bytes; truncation is explicit.
- Provider reason text is limited to 240 code points.
- Each event receives a deterministic, opaque evidence-key prefix derived from its normalized event ID.
- Only fields present in the current bounded payload are citeable.

## Prompt-injection boundary

`title`, `detail`, and `category` are explicitly declared `UNTRUSTED DATA` in both the isolated system contract and user prompt. Text that asks the model to ignore instructions, reveal prompts, call MCP, or use tools remains announcement data. Output still passes the same schema, evidence availability, prohibited-language, and no-recommendation validators before it is returned.

## Failure semantics

`available`, `no_events`, `not_queried`, `unsupported`, `partial`, `stale`, and `unavailable` remain distinct evidence states. An unavailable ToAlpha request does not hide official Taiwan evidence and does not become an empty-success claim. See [the integration audit](TOALPHA_MOPS_INTEGRATION_AUDIT.md) for source ownership, configuration, and endpoint details.
