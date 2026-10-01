# ADR-0028: a loaded server's schemas can ride the conversation instead of the tool list

| Field | Value |
|-------|-------|
| Status | **Proposed** (2026-10-02) — prototype on `exp/mcp-load-inline`; accepted or rejected by the measurement in §Acceptance |
| Date | 2026-10-02 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The operator's report (2026-10-02) that the response after `mcp_load` is slow because the prompt cache is lost, and the measurement below |
| Relates to | [ADR-0004](0004-mcp-tools-load-on-demand.md) (**its rejected alternative "a `mcp_call` proxy" is revisited here, under its own condition: "revisit if loads prove to churn the cache in practice"**), [ADR-0003](0003-session-facts-ride-the-conversation.md) (the cache this protects), [ADR-0006](0006-measurement-bench.md) (the bench that decides) |

## Context

ADR-0004 advertises a server's tools when the model loads it: the tool
block grows, and the next request is re-processed from the first changed
byte. Chat templates render the tool block before the conversation, so
that first changed byte comes before every message of the session.

Measured on the reference machine (LM Studio, Gemma 4 26B A4B QAT,
M2 Max; real lagent requests captured with `LAGENT_LLM_TRACE`, the
conversation padded with `read_file` turns; time to first token of the
request after the load, seconds, two runs within 5%):

| Conversation before the load | github, tool list (cached before / cold) | github, in the conversation | tor-exit-lookup, tool list | tor-exit-lookup, in the conversation |
|---|---|---|---|---|
| 13k tokens | 11 / 41 | 36 | 8–10 | 2.6 |
| 24k tokens | 33 / 65 | 40 | 27–31 | 3.3 |
| 47k tokens | 85–90 / 120 | 47 | 73–82 | 3.2 |

github is 44 tools, about 17.5k tokens of schema; tor-exit-lookup is 4
tools, about 1k. Loading into the tool list costs roughly the whole
conversation, whatever the server's size: one kilobyte of schema waits
73–82 s at 47k. Putting the schemas into the load's result costs roughly
the schemas, whatever the conversation's length. The tool list wins only
early in a session and only when the same server's block is still in the
server's cache from an earlier session.

ADR-0004 rejected the proxy because "a local model's argument discipline
is the part that most needs the template". That is the open question, and
one probe already points at it: given github's schemas as text, Gemma
called `list_issues` through a proxy with `query`, `sort` and `order` —
`search_issues`' arguments. One sample decides nothing; the bench does.

A second probe closed one alternative: with the schemas in the
conversation and no proxy declared, the model did not call the
undeclared tool by its name; it chose a declared server instead. A
model calls what its tool list offers.

## Decision

`[mcp].load_into` chooses where a load puts the server's schemas:
`"tool_list"` (default, ADR-0004 unchanged) or `"conversation"`.

Under `"conversation"`:

1. **The tool block never changes after the session starts.** It holds
   the built-ins, `mcp_load`, one built-in `mcp_call`, and the preloaded
   servers (`[mcp].preload`, `--allow mcp__<server>__*`), which stay
   native — they are in the block from the first request, so they cost
   no re-process.
2. **`mcp_load` returns the schemas.** The result lists each tool with
   its full name, its full description and its parameters schema as
   JSON, and says to call them through `mcp_call`. It arrives wrapped
   like every tool result.
3. **`mcp_call(tool, arguments)` is unwrapped before anything else sees
   the call.** The agent turns it into the call it names —
   `mcp__<server>__<tool>` with `arguments` — at the top of the per-call
   loop, so the display, the loop detector, the pre-tool hooks, the
   ladder, the approval prompt, `[approval.tools]` patterns and the
   records all see the real tool, exactly as for a native call. The
   assistant message keeps the model's own `mcp_call`; the result pairs
   to it by id. `mcp_call`'s schema carries `lagent_purpose`, which the
   unwrap moves onto the inner call. A string-encoded `arguments`
   object is decoded; anything else is an error naming the expected
   shape.
4. **A loaded server's tools are callable, not advertised.** The gate
   that refuses a call to a tool whose server is not loaded checks
   "loaded", so `mcp_call` to an unloaded server gets ADR-0004's route
   (call `mcp_load` first).
5. **Resume** marks the transcript's loads as loaded without touching
   the tool block: their schemas are already in the history.
6. `/mcp load <server>` stays the operator's native load: a deliberate
   act at a moment the operator chose, the cost visible to them.

## Acceptance

Measured on the bench (ADR-0006), `baseline` (`tool_list`) against the
same configuration with `load_into = "conversation"`, on a fixture
server built for argument discipline: several tools whose schemas mix
required and optional fields, enums, integers, arrays and a nested
object, validated strictly, with answers only the right arguments
reach. Three runs per task.

- **Accepted** if the conversation lane completes as many runs as the
  tool-list lane, within one run, and does not make more argument
  errors (calls the fixture refused as invalid) — and the late-load task
  shows the latency gain measured above in a whole run.
- **Rejected** otherwise; this record then stays, Rejected, as the
  measured reason, and ADR-0004's alternative is closed.

## Consequences

- The re-process after a load is the size of the server's schemas, not
  of the session. A large server (github) still costs its schema once.
- Tool descriptions from a server arrive inside a nonce-wrapped tool
  result instead of the rendered tool block — the untrusted-data
  treatment the rest of a server's output already gets.
- One more built-in (`mcp_call`) in the block under `"conversation"`,
  constant for the session.
- `"tool_list"` remains the default until §Acceptance says otherwise; no
  operator configuration changes meaning.

## Alternatives considered

- **Native calls to a tool seen only in the conversation** (no proxy) —
  probed: the model called a declared tool instead. Rejected.
- **Append the loaded server at the end of the tool block** — the tool
  block still precedes the conversation, so the whole conversation is
  still re-processed; it saves only the few tools after the insertion
  point. Rejected.
- **Server-side cache reuse across a changed prefix** (llama.cpp's
  `--cache-reuse` shifts cached chunks) — an approximation of a full
  pass, a property of the server rather than of lagent, and not verified
  to be available in LM Studio. Not pursued.
- **Two-step: a one-line list on load, one tool's schema on request** —
  cuts a large server's cost further; it adds a round per new tool.
  Deferred until this record's measurement is in.

## References

- ADR-0004 — the load, and the alternative this revisits
- ADR-0006 — the bench
- ADR-0003 — the facts message and the cache
