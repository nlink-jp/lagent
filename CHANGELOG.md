# Changelog

## [Unreleased]

### Added

- **The task bench** (ADR-0006). `go run ./bench run` measures the
  runtime on six fixture tasks (search then answer, read then edit, a
  change across files, a shell count, an MCP lookup against the bench's
  own fixture server, an image to look at) in isolated homes and state
  roots, reads each run's transcript, and `bench report` prints
  completion, no-tool answers and the medians of rounds, tool calls,
  prompt tokens and wall time per task and configuration. Phase 2
  changes are measured against it.
- **An empty completion is asked again once** (ADR-0007). The bench's
  read-edit task failed on a completion with no text and no tool call;
  the raw stream showed one mis-sampled tool-call opener routed into
  the reasoning channel, and the same request re-sent answered
  normally. The runtime now re-sends once, notes it, and records both
  attempts (`assistant_empty` carries `retried`); a second empty
  completion ends the turn as before.
- `LAGENT_LLM_TRACE=<dir>` writes every model request and its raw SSE
  reply to files, so a bench run's odd completion can be read as the
  server sent it. Off unless set.
- Homebrew tap distribution: `brew install nlink-jp/tap/lagent` installs
  the notarized release archive as-is; `make brew` generates the formula
  from the built zip (the org's vendored `gen-brew.sh`).

## [0.1.0] - 2026-09-10

### Measured

- The RFP's side-by-side usage comparison with gem-agent did not become
  one: Gemma 4 answers the same instructions in a single round where
  Gemini runs a tool loop, so the transcripts record different work.
  The compatibility check is `gem-usage-lens verify --sessions-root`
  against lagent's sessions (no checksum failure); the single-round
  behaviour is the Phase 1 effectiveness finding.

### Added

- **Images reach the model** (ADR-0005). An image path dropped on the
  terminal attaches without an `@` (backslash-escaped spaces read, in
  `@` references too); the `view_image` built-in returns so the model
  can look at an image a tool saved; and a reference that did not
  attach is told to the model as `[not attached: <ref> — <reason>]`
  beside the operator's warning. Measured cause: a pasted screenshot
  path attached nothing, and the model described an image it never
  had.
- **MCP tools are advertised on demand** (ADR-0004). Every server is
  connected and every tool registered as before, but the model is shown
  a catalog in the runtime facts — one line per server with its tool
  names and what it said about itself — and a built-in `mcp_load` that
  advertises one server's tools for the rest of the session. A call to
  a tool the model was not shown is refused with the route.
  `[mcp].advertise = "deferred"` (default) or `"all"` (the baseline);
  `[mcp].preload` and a `--allow mcp__<server>__*` grant advertise from
  the start; `/mcp` shows the loaded state and `/mcp load <server>`
  loads by hand; a resumed session replays its loads. With the
  operator's 243 tools the cold prefix drops from about 63k tokens to
  about 4k.
- **The system prompt is byte-identical across sessions** (ADR-0003).
  The isolation tag name, the session work directory and the start
  date now open the conversation as the runtime's own message
  (`Agent.AnnounceSession`) instead of living in the system prompt, so
  the local server's prefix cache survives a new session and a
  `/clear`. Measured with the operator's 243 MCP tools (60k tokens of
  schemas): a new session started in 2 s instead of 118 s. The session
  listing does not preview that message or count it as a conversation.
- **Phase 1 core (RFP §4).** The agent loop, the built-in file and
  shell tools, the sandbox lanes, the approval gate with its rule-tier
  auto-approve ladder, the MCP client, the JSONL transcript with
  `--continue` / `--resume`, one-shot `-p`, the inline TUI and the plain
  REPL, usage records in the gem-usage-lens shape — ported from gem-agent
  at the commit ADR-0001 pins, minus the features ADR-0002 lists.
- **The OpenAI-compatible backend** (`internal/llm`), written new: stdlib
  `net/http`, a hand-written SSE reader, tool-call assembly from indexed
  deltas, transient retry with backoff while nothing has been consumed,
  `finish_reason=length` returned as a partial result, and a per-provider
  context-length probe (`[llm].provider`: LM Studio's `/api/v0/models`,
  Ollama's `/api/show`, or the configured `[model].context_window`).
- **Config schema:** `[llm]` (provider / base_url / model / api_key) and
  `[model].context_window`; `LAGENT_PROVIDER` / `LAGENT_BASE_URL` /
  `LAGENT_MODEL` / `LAGENT_API_KEY` in the environment layer.
- The sibling runtime's `.gem-agent.toml` is an operator-only file here
  too: the two runtimes share projects, and a file only one protects is
  a file the other's model may rewrite.
- Scaffold: Go module, cobra root command answering `--version` and
  `version` identically (pinned by a test), Makefile with the org
  build / sign / notarize / release-gate targets, docs mirror check,
  RFP, ADR-0001 (porting sources pinned), ADR-0002 (features not
  reproduced from gem-agent).

### Fixed

- `/readonly` and `/help` described commands this runtime does not
  have: the usage line offered `auto on|off` (gem-agent's watcher, not
  ported), and `/help` listed `/compact`, `/riskbook`, `/memory`,
  `/skills` and `/skill`. The UI catalog drops those strings and the
  ones no code reads; `/help` describes `/mcp load <server>`. The same
  sweep over every string in the runtime: the round-limit notice no
  longer credits a progress review, the approval reasons no longer name
  a model tier, the memory-write rule and ceiling kind are gone with
  the memory tools, `/usage` no longer has empty review and compaction
  lines, and the TUI's skill-expansion hook is gone.
- A network client failing in the read lane is told which lane to ask
  for even when it printed nothing: `curl -s` exits 6 without a word,
  so the text-keyed hint never fired and the model retried the same
  lane until it concluded the sandbox blocks the network. The hint now
  also keys on a finite list of network programs (curl, wget, ssh, scp,
  sftp, nc, ncat, telnet, dig, nslookup, host, ping, traceroute) and
  git's remote subcommands.
- The documentation was audited against the code before release: the
  configuration reference no longer shows a default for `[llm].model`
  (there is none; startup fails without it), lists `LAGENT_STATE_DIR`,
  the exported `LAGENT_SESSION_ID` / `LAGENT_WORK_DIR` /
  `LAGENT_PROJECT_DIR` and the `LAGENT_MCP_STDERR` debug switch, and
  covers every subcommand flag and slash command; the RFP carries
  in-place notes where ADR-0003/0004/0005 and the build amended it; the
  shipped `.lagent.toml` template and the ported code cite gem-agent's
  ADRs as gem-agent's, not as records this repository has.
