# Changelog

## [Unreleased]

### Added

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
