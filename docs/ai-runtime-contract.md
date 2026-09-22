# AI runtime contract

This is the product contract for mystocktracer AI features. An optional
third-party adapter may implement it, but neither the frontend nor product
settings are coupled to that adapter's private protocol.

## General AI Chat

`GET /api/v1/ai/ws` is authenticated using the local backend token and speaks
protocol version `1`. It accepts only `session.start`, `prompt.submit`, and
`session.interrupt`; malformed, unknown, or oversized frames terminate or
reject the session without forwarding them to a runtime. The server emits
`runtime.ready`, `session.ready`, `message.delta`, `message.complete`, or a
sanitized `runtime.error`.

The client may provide a bounded prior session identifier and no more than 32
seed messages. If the runtime cannot resume a prior session, the backend
creates a fresh one. Prompts are bounded at 64 KiB, session identifiers at 256
bytes, and a socket disconnect cancels the child process. API keys, provider
headers, and subprocess output are never included in an error event.

The normal chat is available only when the optional runtime is installed and a
model is configured. It uses explicitly saved profiles, retains a secret when
the update is absent, clears it only on an explicit empty update, and never
returns a secret through the settings API.

## Taiwan AI Research

Taiwan AI Research runs only after an explicit user request. It uses its own
evidence-only system context and bounded evidence payload; it must not inherit
normal chat history, agent memory, Skills, MCP servers, browser state, files,
or general-agent system instructions. It has no arbitrary web or tool access.

The existing `taiwan_ai_research_v2` result schema, provenance, freshness,
partial/stale handling, idempotent run ID, history persistence, and previous
comparison behavior remain unchanged. Only successful research is persisted.
Dashboard loading never invokes an LLM.

## Model and capability settings

Profiles preserve provider, base URL, model, API mode, and response timeout.
Supported API modes include `chat_completions`, `codex_responses`, and
`anthropic_messages`; provider translation belongs at the adapter boundary.
Custom endpoints must be bounded HTTP(S) URLs without embedded credentials,
newlines, query, or fragment. Connection testing and model discovery use
bounded requests and redact provider failures.

Skills and MCP are explicit user configuration for general chat only. MCP
supports stdio, Streamable HTTP, and SSE where the selected adapter supports
them, with arguments, timeout, connection timeout, optional parallel calls,
and masked environment/header secrets. They are never inherited by Taiwan AI
Research.
