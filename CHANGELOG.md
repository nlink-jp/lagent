# Changelog

## [Unreleased]

### Fixed

- `/readonly` and `/help` described commands this runtime does not
  have: the usage line offered `auto on|off` (gem-agent's watcher, not
  ported), and `/help` listed `/compact`, `/riskbook`, `/memory`,
  `/skills` and `/skill`. The UI catalog drops those strings and the
  34 others no code reads; `/help` describes `/mcp load <server>`.
- A network client failing in the read lane is told which lane to ask
  for even when it printed nothing: `curl -s` exits 6 without a word,
  so the text-keyed hint never fired and the model retried the same
  lane until it concluded the sandbox blocks the network. The hint now
  also keys on a finite list of network programs (curl, wget, ssh, scp,
  sftp, nc, ncat, telnet, dig, nslookup, host, ping, traceroute) and
  git's remote subcommands.

### Added

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
- **Phase 1 core (RFP §4).** The agent loop, the eight built-in file and
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
