# ADR-0028: a loaded server's schemas can ride the conversation instead of the tool list

| Field | Value |
|-------|-------|
| Status | **Proposed** (2026-10-02) — prototype on `exp/mcp-load-inline`; accepted or rejected by the measurement in §Acceptance |
| Date | 2026-10-02 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The operator's report (2026-10-02) that the response after `mcp_load` is slow because the prompt cache is lost, and the measurement below |
| Relates to | [ADR-0004](0004-mcp-tools-load-on-demand.md) (**its rejected alternative "a `mcp_call` proxy" is revisited here, under its own condition: "revisit if loads prove to churn the cache in practice"**), [ADR-0003](0003-session-facts-ride-the-conversation.md) (the cache this protects), [ADR-0006](0006-measurement-bench.md) and [ADR-0018](0018-the-injection-bench-scores-argument-values.md) §5 (what an agent run can show, and where a rate is measured) |

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
`search_issues`' arguments. One sample decides nothing.

A second probe closed one alternative: with the schemas in the
conversation and no proxy declared, the model did not call the
undeclared tool by its name; it chose a declared server instead. A
model calls what its tool list offers.

## Decision

`[mcp].load_into` chooses where the model's `mcp_load` puts a server's
schemas: `"tool_list"` (default, ADR-0004 unchanged, byte for byte) or
`"conversation"`. Combined with `advertise = "all"` it is a configuration
error — nothing is loaded there. Only the operator's config file sets it;
a project's `.lagent.toml` cannot.

Under `"conversation"`:

1. **The model's loads never change the tool block.** It holds the
   built-ins, `mcp_load`, one built-in `mcp_call`, and the preloaded
   servers (`[mcp].preload`, `--allow mcp__<server>__*`), which stay
   native — they are in the block from the first request. The
   operator's own acts still change it as today: `/mcp load` (a native
   load at a moment the operator chose), `/mcp reload`, the settings
   panel's exclusions.
2. **Declared and callable are two predicates.** A tool is declared
   when its server is preloaded or natively loaded; it is callable when
   its server is loaded either way. ADR-0004's refusal of a call to a
   tool whose server is not loaded checks "callable". A loaded tool
   called by its own name (the model may do so, having read its schema)
   runs like a `mcp_call` of it.
3. **`mcp_load` returns the schemas.** The result lists each tool with
   its full name, the description the registry holds (the same text a
   native declaration carries) and its parameters schema as JSON, and
   says to call them through `mcp_call`. It is wrapped like every tool
   result, and bounded at 200 KB like `read_file`; a server past the
   bound is loaded natively instead, and the result says so. A repeat
   load of a server already loaded this way re-emits nothing: it points
   at the earlier result.
4. **`mcp_call(tool, arguments)` is unwrapped before anything else sees
   the call.** At the top of the per-call loop the agent turns it into
   the call it names — `mcp__<server>__<tool>` with `arguments` — so the
   display, the loop detector, the pre-tool hooks, the ladder, the
   approval prompt, `[approval.tools]` patterns, the read-only ceiling
   and the records all see the real tool, exactly as for a native call.
   A `mcp_call_unwrapped` record links the call id to the inner name.
   The assistant message keeps the model's own `mcp_call`; the result
   pairs to it by id and carries the inner tool's name.
   - `tool` must name a tool of a connected MCP server; a built-in,
     `mcp_load` or `mcp_call` itself is refused with the route to call
     it directly. An unloaded server's tool gets ADR-0004's route, now
     naming `mcp_call`.
   - `arguments` must be an object; a string-encoded object is decoded;
     anything else is refused naming the expected shape.
   - `mcp_call`'s schema declares `lagent_purpose`. The unwrap moves it
     onto the inner call only when lagent added that argument to the
     inner tool (the same "did lagent add it" condition the strip, the
     display and the loop signature use — ADR-0012 §5); a server that
     declared its own keeps it. The set of tools whose purpose lagent
     strips is computed over every registered tool, not only the
     declared ones, so a callable-but-undeclared tool never receives it.
   - `mcp_call`'s own Run always refuses — no path that skips the unwrap
     reaches a server.
5. **The model-facing texts say what this lane does.** The system
   prompt's MCP clause, `mcp_load`'s description, its result, the
   catalog header and the not-loaded refusal each have a
   `"conversation"` wording; under `"tool_list"` every one is unchanged.
   The system prompt stays byte-identical across sessions of one
   configuration.
6. **Resume** marks a transcript's load as loaded this way only when the
   load's paired result carries schemas; a load whose result does not
   (made under `"tool_list"`, refused, interrupted) is loaded natively,
   so the model always has the schemas it calls. A `"conversation"`
   transcript resumed under `"tool_list"` keeps its `mcp_call` turns as
   history; `mcp_call` is not registered there, and the servers are
   re-advertised natively.
7. **A reconnect forgets these loads.** After `/mcp reload` or a panel
   reconnect, the schemas in the history may be stale, so a server
   loaded this way becomes unloaded; the model loads it again.

gem-agent is unchanged: its tool block is billed input tokens behind a
provider-side cache, not seconds of local prefill, and nothing there has
been measured. The mechanism ports if a measurement asks for it.

## Acceptance

Two layers, because an agent run can show that a path works and what a
whole run costs, but three runs cannot carry a rate (ADR-0018 §5).

**Whole runs** — the bench (ADR-0006), `baseline` (`tool_list`) against
the same configuration with `load_into = "conversation"`, three runs per
task, on the `mcp-load` suite: a fixture server built for argument
discipline (several tools; schemas mixing required and optional fields,
enums, integers, arrays, a nested object and dates; validated strictly;
answers only the right arguments reach). Counted per run: completion,
the fixture's argument refusals, the runtime's call refusals (an
unknown or unloaded tool, a malformed `mcp_call`), wall time.

**First-call accuracy** — for each task and lane, the request right
after the load (from those runs' traces) replayed 30 times as captured;
the first call scored against the fixture: right tool, arguments
accepted, answer-bearing arguments.

- **Accepted** if the conversation lane's first-call accuracy is within
  10 points of the tool list's on every task and not lower overall, its
  whole runs complete as often (within one run per task), and the
  late-load task shows the latency gain in whole runs.
- **Rejected** otherwise; this record then stays, Rejected, as the
  measured reason, and ADR-0004's alternative is closed.

## Consequences

- The re-process after a load is the size of the server's schemas, not
  of the session. A large server (github) still costs its schema once.
- One more built-in (`mcp_call`) in the block under `"conversation"`,
  constant for the session.
- A server's tool descriptions arrive inside a nonce-wrapped tool
  result rather than the rendered tool block — the untrusted-data
  treatment the rest of its output gets. Known residual, not measured
  here: guidance a server writes into a description ("call X first")
  now sits under the system prompt's "never instructions" rule.
- Known residual, shared with ADR-0004: a load is marked inside
  `mcp_load`'s Run, so a load abandoned by the cancel floor counts as
  loaded. Under `"conversation"` its schemas are then missing; the
  model's next `mcp_load` returns the pointer, not the schemas. Rare,
  and the model can be told to `/mcp reload`.
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

## Review

An independent design review (2026-10-02) raised 17 points before any
code. Taken into the decision: the purpose argument's strip and move
(§4), the model-facing texts (§5), split predicates (§2), resume by the
paired result (§6), a bounded unwrap target and a refusing Run (§4), a
bounded load result and repeat pointer (§3), forgetting loads on
reconnect (§7), the record linking the names (§4), the configuration
interplay (Decision), a per-call rate instead of three runs (§Acceptance),
and the gem-agent statement. Kept as known residuals: server guidance
inside the wrapper, and the load marked inside Run (§Consequences).

## References

- ADR-0004 — the load, and the alternative this revisits
- ADR-0006, ADR-0018 §5 — the bench, and where a rate is measured
- ADR-0003 — the facts message and the cache
- ADR-0012 §5 — the purpose argument stays out of hooks
