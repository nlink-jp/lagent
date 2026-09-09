# ADR-0004: MCP tools are advertised on demand, from a catalog the model reads first

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-10 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | After ADR-0003 the prefix cache survives a new session, but the first session after a server restart still pays the whole tool block: 243 MCP tools, 60,097 tokens, about two minutes on the reference machine — and a 26B model choosing among 243 schemas is a measurement nobody has made |

## Context

Every request advertises every tool of every connected MCP server as a
function schema. With the operator's global `mcp.json` (23 servers)
that is 243 schemas and 60,097 tokens before a word of conversation;
three servers (slack-extender, github, obsidian) are half of it. The
built-in tools (eight file and shell tools and `ask_user`) are nine
schemas, about 2,500 tokens.

ADR-0003 made that block cacheable across sessions. It did not make it
small: the server's cache is per process and holds what fits, the
first session after LM Studio starts pays the full pass, and the model
reads 243 schemas on every turn whether or not the task touches any of
them. For a local model both costs are structural — prompt processing
at about 580 tokens per second, and attention spread across two
hundred tools it will not call.

`[mcp].exclude` is the lever available today: the operator trims the
set by hand, per project. It asks the operator to predict which servers
a session will need, which is the guess the runtime is better placed to
avoid making.

The RFP keeps drop-in reading of `.mcp.json`: the set of servers stays
the operator's Claude Code configuration, read as it is. What this
record changes is what the model is shown, not what is connected.

## Decision

MCP servers are connected as today; their tools are registered as today
(`mcp__<server>__<tool>`, filtered by `[mcp].exclude`, gated by the same
ladder); but they are **not advertised** to the model until a server is
loaded. The model sees:

1. **A catalog, in the runtime-facts message** (ADR-0003's lane, not
   the system prompt): one line per connected server — its name, its
   tool count and tool names, and the first sentence of the
   `instructions` it published at `initialize`, clipped. The catalog is
   session state (which servers came up), which is exactly what the
   facts message carries; the system prompt and the advertised tool
   block stay byte-identical whatever the MCP configuration.
2. **One built-in tool, `mcp_load`**, taking a server name. It
   advertises that server's registered tools for the rest of the
   session (`Agent.RefreshTools` with the widened set) and returns
   their one-line list. Read-only and Safe: the server is already
   running and nothing runs by loading it. Unknown and excluded names
   are refused with the catalog's spelling.
3. After a load, the server's tools are called **natively**, by their
   registered names, with the schema the chat template renders — the
   argument discipline a 26B model gets from a rendered schema is the
   reason this is a load and not a `mcp_call(server, tool, json)`
   proxy (see Alternatives).

The trigger is written where a capability's trigger has to be: the
catalog line says "call `mcp_load` with the server name first; its tools
then appear", and the stable system prompt says one generic sentence
about MCP servers being listed in the runtime-facts message and loaded
on demand. Nothing says "do not" — the omission of the schemas is the
whole restriction.

Operator controls:

- `[mcp].preload = ["tor-exit-lookup", …]` advertises named servers
  from the start, for the lookups an operator uses every session; the
  cache then holds them like the built-ins.
- `--allow mcp__<server>__*` preloads that server, in every mode: the
  grant is the operator's declaration that the run needs it, and a
  pipeline must not depend on the model remembering to load.
- `[mcp].advertise = "all"` restores today's behaviour — the
  measurement baseline, and gem-agent parity for a comparison run.
- `/mcp` shows each server's loaded state; `/mcp load <server>` loads
  one by hand.

Persistence: a load is recorded in the transcript as the tool call it
is; on `--continue` / `--resume` the runtime replays the `mcp_load`
calls it finds and advertises those servers again, so a resumed
session sees what it saw. `/clear` starts unloaded, with a fresh
catalog in the new facts message.

Accounting: the usage records need no new field — the saving shows as
`prompt` tokens per round, side by side with the baseline in
gem-usage-lens.

## Consequences

- The cold prefix drops from about 63k tokens (the built-ins + 243 MCP
  schemas) to about 4k (the built-ins, `mcp_load`, and a catalog line
  per server) for the operator's configuration — the two-minute first
  session becomes about ten seconds, and the cache is a bonus rather
  than a rescue.
- A load costs one prefix re-process: the loaded schemas plus the
  conversation so far, since the tool block precedes it. Early in a
  session that is a few seconds; `[mcp].preload` moves the servers an
  operator always wants to the cacheable side.
- The model sees a task-sized tool set. Whether a 26B model picks the
  right server from a 23-line catalog is the measurement this record
  exists to enable; the baseline is `advertise = "all"`.
- Tool-call records, `[approval.tools]` patterns, `--allow`,
  `[mcp].exclude`, the rule tier's `mcp__` handling and the approval
  prompt are untouched: the model's call after a load is the same
  `mcp__<server>__<tool>` call it makes today.
- Two new operator-facing things earn their place: the catalog line in
  the facts message, and the loaded marker on `/mcp`. No banner line.
- gem-agent, the porting source, pays the same block on every Vertex
  request as billed input tokens; this design ports without change and
  is worth proposing there. That proposal is outside this record's
  binding.

## Alternatives considered

- **A `mcp_call(server, tool, arguments)` proxy with schemas fetched
  as text** — keeps the tool block constant forever, but the model
  writes arguments from prose it read rather than a schema the
  template renders, and a local model's argument discipline is the
  part that most needs the template. Rejected; revisit if loads prove
  to churn the cache in practice.
- **Trim the set by hand** (`[mcp].exclude`, today) — remains
  available and composes with this; rejected as the only answer
  because it asks the operator to predict the session.
- **The catalog in the system prompt** — stable within a
  configuration, but a server that fails to start, or a project
  `.mcp.json`, changes it and busts the cache; the facts message is
  where session state already lives. Rejected.
- **Load by first call** (the model calls `mcp__server__tool` unseen
  and the runtime loads on the miss) — the model cannot call a tool
  it was not given a schema for without guessing arguments, and a
  guessed call that reaches a server is the failure mode the approval
  gate exists to stop. Rejected.
- **Advertise everything and rely on the cache** (ADR-0003 alone) —
  the state before this record; kept as the `advertise = "all"`
  baseline.

## References

- ADR-0003 — the facts message, and the cache measurements
- ADR-0002 — the port carries no provider-side tools; MCP is the only
  extension route
- RFP §7 — prompt processing as the constraint to design around
- gem-agent ADR-0077 (`[mcp] exclude`) at the pinned commit — the
  filter this composes with
