# ADR-0002: The gem-agent features lagent does not reproduce

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-10 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | ADR-0001 forbids a port from carrying these features' code, config keys, error text and docs; the list has to exist before the first port |

## Context

gem-agent's direct dependency on Vertex AI and Google Cloud is confined
to four files (`internal/llm/vertex.go`, `internal/llm/web.go`,
`internal/mediastore`, `internal/telemetry/gcp.go`), but the features
those files serve reach into tools, config, error text, the `/usage`
panel and the reference docs. The discarded fork kept those seams; this
record names each feature so a port can recognise and drop it.

"Not reproduced" is accepted as the specification (RFP §1). None of
these is a Phase 2 candidate under another name.

## Decision

lagent does not reproduce the following. For each: what binds it to the
provider, and what lagent does instead.

| gem-agent feature | Bound by | lagent |
|---|---|---|
| `web_search` (Google Search grounding) | a Vertex built-in tool; the search runs provider-side | no web search tool. A project that needs one connects an MCP server |
| `web_fetch` (URL context) | a Vertex built-in tool; the page is fetched provider-side, which is also what made it SSRF-safe | no fetch tool. A local fetch would reach localhost and the LAN from the operator's machine; absent is safer than reinvented |
| `[gcp].bucket` media route (GCS upload for audio/video attachments, ADR-0027 there) | `gs://` URIs are read natively by Vertex only | images attach inline as base64 (measured working); audio and video are not attachments |
| `[telemetry].backend = "gcp"` (Cloud Logging) | ADC and the Cloud Logging client | no telemetry in Phase 1. If audit events return, OTLP only, by ADR |
| thought-signature capture and replay | Gemini 3 wire format | nothing to replay. OpenAI-compatible tool calls carry `id` and the loop echoes `tool_call_id` |
| thought summaries in the TUI (`StreamEvent` kind `thought`) | Gemini thinking output | the `reasoning_content` field, if the server sends it, is display-only; whether to request thinking at all is a Phase 2 measurement |
| `[model].safety` (content-filter thresholds) | Vertex safety settings | no key. A local server has no provider filter to configure |
| `[model].summary` (a lighter model for `summarize_file`) and `agentic_file_search` (a child agent) | cheaper delegated calls on a cloud price list | not in Phase 1. On one local model a delegated call costs a full prompt pass; whether it pays is measured before either returns |
| `tool_prompt` usage bucket (`toolUsePromptTokenCount`) | the two built-in tools above | the field is written, always `0`, so gem-usage-lens reads both runtimes with one schema |
| `GOOGLE_CLOUD_*` precedence layer, `[gcp]` section | Vertex auth and location | `[llm]` section, `LAGENT_*` environment; no ADC lookup anywhere |
| The `us` / `eu` / `global` location rules | Vertex regional serving | none |

A port that finds a reference to a row above removes it — the code, the
config key, the error message that points at it, the doc sentence that
describes it — rather than stubbing it.

## Consequences

- lagent's config has no `[gcp]` section and its startup never touches
  the network before the first model call, trivially: there is no
  metadata server, no ADC, no bucket to probe.
- The tool roster the model sees is smaller by two (`web_search`,
  `web_fetch`) in Phase 1 and by four counting the delegation tools.
  Comparisons with gem-agent must use tasks that need none of them.
- A future "we need web access" is an MCP server, not a lagent feature.
- The usage schema stays lens-compatible with a bucket that is always
  zero; the lens must not read a zero as "derived" (it distinguishes an
  absent key from a zero value — ADR-0066 there).

## Alternatives considered

- **Reproduce web tools locally** (fetch from the operator's machine) —
  rejected: reverses the SSRF property gem-agent got for free, and
  hands the model a route to intranet and authenticated pages.
- **Keep the config keys as accepted-but-ignored** for drop-in config
  compatibility with gem-agent — rejected: a key that does nothing is a
  seam, and gem-agent's config is not a drop-in target (its project
  files AGENTS.md / CLAUDE.md / .mcp.json are).
- **A stub `telemetry` package with only OTLP** — deferred: nothing in
  Phase 1 emits audit events; a package with no producer is dead code.

## References

- ADR-0001 — porting sources and the no-seam rule
- RFP §3 "Out of scope", §7 measured constraints
- gem-agent ADR-0027 (media route), ADR-0035 (telemetry), ADR-0066
  (usage buckets) — for the design each row replaces, at the pinned commit
