# Architecture

Current behaviour of lagent, written to be readable cold. Why a given
decision was made lives in the [ADRs](../INDEX.md#adrs); this document
describes what the code does today. gem-agent's architecture is the
porting source (ADR-0001); where this runtime differs, the difference is
what ADR-0002 leaves out and what the RFP defers to Phase 2.

## Shape

One binary, one process, one conversation. `main.go` hands off to
`cmd.Execute`, which builds five things and wires them together:

```
cmd/            flags, config load, project resolution, wiring, REPL/TUI
  |-- internal/config      strict-decode TOML + env/flag precedence
  |-- internal/llm         Backend interface + the OpenAI-compatible client (stream observer)
  |-- internal/tools       the nine file/shell/image built-ins + Register
  |-- internal/agent       the turn loop, approval dispatch, the round ladder
  `-- internal/tui         Bubble Tea inline UI (or internal/repl, non-TTY)
```

The tools package holds the nine built-ins that need only the project
directory: `list_files`, `list_tree`, `search_files`, `read_file`,
`file_info`, `view_image`, `write_file`, `edit_file`, `shell_exec`.
`cmd/` registers `ask_user` and `mcp_load` through the same `Register`,
plus every MCP tool.

Supporting packages: `internal/sandbox` (Seatbelt profile generation
per lane, the persistent-file and credential lists), `internal/approve`
(plain-REPL gate), `internal/risk` (auto-approve rule tier),
`internal/policy` (per-tool approval policy), `internal/mcp` (stdio
JSON-RPC client), `internal/mcpfilter` (the one predicate behind
`[mcp] exclude`), `internal/banner` (the lines printed before the
operator has typed anything, and the rule that decides which ones),
`internal/mention` (`@`-references: files, directories, images — and a
dropped image path without the `@`, ADR-0005),
`internal/instructions` (`AGENTS.md` discovery), `internal/hooks`
(the operator's hooks on Claude Code's measured contracts: pre-tool,
session start, prompt submit, session end — ADR-0012/0014), `internal/memory` (facts recalled across sessions, two
scopes under the state root, ADR-0013), `internal/skills`
(skill discovery and confined loading in Claude Code's format,
ADR-0011), `internal/ignore`
(ignore-aware enumeration: builtin dir list + gitignore matcher),
`internal/session` (transcript: logger + resume loader, usage records),
`internal/statedir` (per-project state layout), `internal/workdir` (the
per-session work directory under the state root), `internal/trustpin`
(content pins of the agent-facing files and the persistent-file
snapshot), `internal/uitext` (ja/en UI string catalogs),
`internal/bounded` (the capped read/list/capture primitives every other
package uses), `internal/archtest` (AST tests that pin the structural
rules: path packages open through `os.Root`, reads are bounded, the
rule tier is consulted in one function, every loader of project content
takes the grant).

## The backend

`internal/llm` sends every conversation to one endpoint, the
OpenAI-compatible `chat/completions` at `[llm].base_url`, with stdlib
`net/http` and a hand-written SSE reader. The history is converted to
the wire shape once per call: the system prompt as the system message,
user text (with `@`-referenced and bare-dropped images as image parts,
ADR-0005; a `view_image` result carries its image the same way),
assistant turns
with their tool calls, tool results paired to their calls by id — ids
the server sent, or synthetic ones when a transcript carries none.
Streaming folds text deltas to the caller as they arrive and assembles
tool calls from their indexed deltas; the final usage chunk fills the
four accounting buckets. A transient failure (429, 5xx, a dropped
connection) retries with backoff only while nothing has been consumed.
A `finish_reason` of `length` is returned as a partial result with the
text that arrived, never discarded.

`[llm].provider` selects one thing: where `ContextWindow` asks. LM
Studio answers on its native `/api/v0/models/<id>`, Ollama on
`/api/show`, and a plain OpenAI-compatible server has no such endpoint,
so `[model].context_window` is required there.

## MCP servers

`.mcp.json` is read in Claude Code's format; every server is connected
and every tool registered as `mcp__<server>__<tool>`, filtered by
`[mcp].exclude`. What the model is shown is decided separately
(ADR-0004, `cmd/mcpload.go`): under `[mcp].advertise = "deferred"` the
runtime-facts message carries a catalog — one line per server with its
tool names and the first sentence of the `instructions` it published at
initialize — and the built-in `mcp_load` advertises a server's tools
for the rest of the session. A call to a registered tool the model was
not shown is refused before any gate, with the route. `[mcp].preload`
and a `--allow mcp__<server>__*` grant advertise from the start;
`"all"` is the baseline that advertises everything. A resumed session
replays its `mcp_load` calls; `/clear` starts unloaded with a fresh
catalog; `/mcp` shows each server's loaded state.

## One turn

`Agent.Run` takes the operator's text, expands `@`-references into
attachments beside it, and loops: send the history (tool results
nonce-wrapped at send time), stream the answer, and for each tool call
decide, gate, execute, and append the result. The loop ends on a text
answer, a round limit, or a loop-guard stop. A completion that carries
neither text nor a tool call is asked again, twice at most, with the
same history and one transient line appended, before it is reported
(ADR-0007). Every request replays the
whole history behind a session-scoped isolation tag, so the request
prefix stays byte-identical across rounds and the server's prefix
cache can hit — on a local model that cache is the difference between
a two-second turn and a two-minute one.

The system prompt is byte-identical across sessions too (ADR-0003):
the isolation tag's name, the session work directory and the start
date open the conversation as the runtime's own user-role message
(`Agent.AnnounceSession`, `session.FactsPrefix`), because the server
renders every tool schema after the system text and re-processes all
of it when one byte there changes. The listing neither previews that
message nor counts it as a conversation.

## The agent core knows nothing about the UI

`agent.Options` is the whole contract between the loop and whatever
runs it; every surface (TUI, plain REPL, one-shot) wires the same
callbacks and the agent never imports a UI package:

- `OnToolCall` / `OnToolDone` — a call is about to be gated and
  executed; a call has produced its result (the TUI's activity line and
  its stall detector re-arm on the second, never on stream chunks).
- `OnUsage` — one round's token spend, for the footer gauge.
- `OnAutoDecision` — each auto-mode verdict, so the UI can show what ran
  without asking, and why.
- `BeforeOperatorWrite` / `OnOperatorWrite` — an operator-approved write
  into the files later sessions trust is about to run; it ran (the pins
  compare the file before and after).
- `OnAttach` — what an `@`-reference pulled in, and what it could not.
- `OnNotice` — an in-turn notice (a truncated answer, a late-returning
  abandoned call) for the operator to see.
- `OnRoundLimit` — the checkpoint dialog; nil means unattended, and the
  checkpoint stops.
- `ClipboardImage` — the `@clipboard` capture; nil reports it unavailable.
- `Advertise` — which registered tools are declared to the model
  (ADR-0004); a call to a hidden one is refused before any gate.
- `PreToolHook` — the operator's pre-tool hooks (ADR-0012), consulted
  after the advertise check and before the ladder; a deny is a floor
  and its reason is the tool result.
- `PromptHook` — the operator's prompt-submit hooks (ADR-0014), seen
  before a turn is recorded; a block erases the prompt, context rides
  the turn as a `hook` attachment quoted as data.

## Approval

Before the ladder, the operator's pre-tool hooks (`internal/hooks`,
ADR-0012) may refuse a call outright: a deny is a floor no mode, row or
allowlist lifts, and the reason goes to the model as the tool result.
The rule tier (`internal/risk`) then classifies every call: Safe runs under
`--auto` without asking, Block always asks, Review asks the operator.
There is no model tier here: gem-agent's second model call judging the
proposed call was measured and not adopted (ADR-0010). The session ceiling
(`--read-only`) caps the lane a call may reach; lifting it is the
operator's act. Operator-only files — the instruction files, `.mcp.json`,
`.lagent.toml`, and the sibling runtime's `.gem-agent.toml` — are never
answered by a standing approval. Credential material is the same
question for the read tools (ADR-0015): `read_file`, `file_info` and
`view_image` on a path the lanes deny — `.env` and its variants, a
private key, `credentials.json`, the token stores under home, by the
one list `internal/sandbox` keeps; `.env.example` and its siblings are
ordinary files — are a Review only the operator answers, judged on the
real path, in every mode; `-p` denies them. `search_files`,
`list_files` and `list_tree` do not ask: they skip such an entry and
report the count and names.

## The round ladder

Three consecutive identical calls escalate immediately; the round limit
is a checkpoint. Interactively both ask the operator, with the turn's
recent calls as evidence; unattended (`-p`), both stop the turn,
fail-closed, because there is no model review to vouch for progress in
the operator's place. An absolute cap of three times `[agent].max_turns`
bounds the spend nothing can lift.

## Persistence

The transcript is a JSONL file per session under the state root;
`--continue` and `--resume` replay it. Usage records are written per
model call in the shape gem-usage-lens reads for both runtimes
(`prompt` / `output` / `thoughts` / `cached` / `tool_prompt` / `total`,
with `tool_prompt` always zero here). The per-session work directory
lives under the same state root in a per-project directory of its own
(`LAGENT_STATE_DIR` overrides the root) and is exported to children as
`LAGENT_WORK_DIR`, beside `LAGENT_SESSION_ID` and `LAGENT_PROJECT_DIR`.
Those three are the only `LAGENT_*` names a read-lane `shell_exec`
keeps by name; every other variable whose name looks like a secret,
`LAGENT_API_KEY` included, is dropped from that lane's environment.

## Configuration and drop-in behaviour

`~/.config/lagent/config.toml` is strict-decoded; precedence is flags >
`LAGENT_*` > file > defaults. The project's `AGENTS.md` / `CLAUDE.md` /
`AGENT.md` / `GEMINI.md` are read as they are, up the ancestor chain
and from `~/.config/lagent`, the project's own after it is trusted and
its pins agree; `.mcp.json` is read in
Claude Code's format. `.lagent.toml` carries the project's approval
policy and MCP exclusions, nothing else.

## Not here

`web_search`, `web_fetch`, media uploads, Cloud Logging, thought
signatures, safety settings, the summary model and the delegated file
search are gem-agent features bound to Vertex AI (ADR-0002). History
compaction is the one Phase 2 item left (RFP §4); the model tier was
measured and not adopted (ADR-0010); skills (ADR-0011), pre-tool hooks
(ADR-0012) and memory (ADR-0013) are in.

## Skills

A skill is Claude Code's `SKILL.md` directory, read as-is (ADR-0011).
Global skills are copied into `~/.config/lagent/skills/<name>/` — never
linked from `~/.claude`, which the lanes deny; project skills are the
trusted project's `.claude/skills/<name>/`, pinned by content like the
instruction files, so a changed one stays out until re-trusted. One
catalog line per skill rides the runtime-facts message beside the MCP
catalog, so the system prompt stays byte-identical (ADR-0003).
`load_skill` is the one tool whose results enter the prompt unwrapped:
they are the operator's own instructions, and the tool cannot read
outside a discovered skill's directory. `/skill <name>` sends a skill's
body as the turn by hand; `/skills` lists what is loaded.

## Memory

Short facts recalled in every session (ADR-0013): global
(`<state>/memory/global/<name>.md`) and project
(`<state>/memory/projects/<escaped>/<name>.md`), plain markdown outside
the repository. Recall rides the runtime-facts message, not the system
prompt — measured: a standing directive in the instruction section was
acted on in 0/18 runs, one line in the facts message in 5/6 — so a
memory that names a file or a command is a pointer the model acts on.
The operator writes with `/remember` and removes with `/forget`; the
model proposes through `save_memory` / `delete_memory`, which the rule
tier keeps at Review so every save asks. Read once at start and on
`/clear`; `/memory` reads the disk.
