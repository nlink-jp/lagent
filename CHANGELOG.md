# Changelog

## [Unreleased]

### Security

- The read lane keeps the runtime's own exports by name, not by
  prefix. `sandbox.ScrubEnv` exempted every `LAGENT_*` variable from
  the secret-name rule, so `LAGENT_API_KEY` — the config's bearer token
  — reached every read-lane command's environment, from where a bare
  `env` puts it in front of the model and into the transcript (system
  risk review R01). The exemption is now an explicit list of the three
  variables the runtime exports for children, `LAGENT_SESSION_ID`,
  `LAGENT_WORK_DIR` and `LAGENT_PROJECT_DIR`; every other name goes
  through the secret-name rule. The write and operator lanes are
  unchanged. A deliberate local change (ADR-0001: the recorded
  gem-agent source commit is unchanged).
- Credential paths are operator-only for the read tools too
  (ADR-0015). The credential list — `.env` and its variants, private
  keys, `credentials.json`, the token stores under home — was denied
  to the read and write lanes at the kernel, refused by `write_file`
  and `edit_file`, and put in front of the operator when a shell
  command named it, but `read_file .env` inside the project was Safe:
  the value went to the model and into the transcript as an ordinary
  tool result (system risk review R02). Now `read_file`, `file_info`
  (each path of its batch) and `view_image` on such a path, resolved
  to the real path the tool will open, are a Review only the operator
  answers, every time: the session allowlist, a `"never"` row and
  `--allow` do not answer it, `--auto` escalates it, `-p` denies it.
  `.env.example` and its siblings stay ordinary files. `search_files`,
  `list_files` and `list_tree` do not prompt: they skip a
  credential-named entry and report the count and names with the
  route. `@` attachments are unchanged. One list, in `internal/sandbox`;
  a deliberate local change (ADR-0001), made in the same shape in
  gem-agent independently.

### Fixed

- A pre-tool hook that exits 0 with output that is not a verdict is
  reported. ADR-0012 §3 promises a warning for unparseable output, and
  the runner emitted one for a crash, a non-zero exit and a timeout,
  but plain text (or JSON that does not parse) on exit 0 let the call
  through in silence — the shape of a guard that meant to deny and
  printed the wrong form (system risk review R06). The call still
  proceeds, as hooks only tighten; the notice names the hook and quotes
  the first line of what it printed. Empty stdout stays silent. JSON
  that parses but is no verdict of the contract — a deny spelled with
  the wrong field (`decision: "deny"`, a bare `permissionDecision`) or
  the wrong value — is reported the same way rather than passing as a
  verdict that denies nothing (independent review); a context hook
  printing such JSON injects nothing and says so.

## [0.3.3] - 2026-09-13

### Security

- `~/.config/mcp-bridge` is a credential location. Its `config.json`
  holds pre-registered OAuth client secrets and static API-key headers
  in plain JSON, and `state/<server>/tokens.json` the access tokens;
  remote HTTP MCP servers are reached through mcp-bridge, and ADR-0002
  makes MCP the only route to the web, so the directory is part of the
  runtime's expected deployment. It joins the credential list beside
  `~/.config/gcloud` and `~/.config/gh`: the read and write lanes deny
  the read at the kernel, the file tools refuse the path, and a shell
  command that names it is Block for the operator to see. A sibling
  entry such as `~/.config/lagent` is unaffected. A deliberate local
  change (ADR-0001: the recorded gem-agent source commit is unchanged).

## [0.3.2] - 2026-09-12

### Fixed

- `LAGENT_LLM_TRACE` files are private: the trace directory is created
  `0700` and each request/response pair `0600`, as the transcript is.
  The trace is the whole request body and the raw stream — the
  instruction files, every tool result, and the `reasoning_content`
  deltas nothing else stores (ADR-0009) — and the operator points it
  anywhere, so the earlier `0755` directory and umask-default files
  left all of it readable by every local user. A directory or file
  that already exists keeps its mode. The configuration reference now
  says what the trace holds and that deleting it is the operator's job.

## [0.3.1] - 2026-09-12

### Fixed

- The footer's `ctx` gauge (and its cache share) is reset by `/clear`.
  It is a mirror fed by each round's usage, and the shared slash
  handler that empties the conversation cannot see the TUI, so the
  discarded conversation's size stayed on screen until the next round
  reported (operator report). The `total` figure counts the process
  and is unchanged.

## [0.3.0] - 2026-09-12

### Added

- Agent memory (ADR-0013): short facts at a global and a project scope
  under the state root, recalled in the runtime-facts message — the
  bench measured a standing directive acted on 0/18 from the
  instruction files at any size and 5/6 from one facts-message line.
  `/remember [global] <name> <fact>`, `/forget`, `/memory`; the model
  proposes with `save_memory` / `delete_memory`, which the rule tier
  keeps at Review so every save asks. The bench gains `pointer-*`,
  `memory-follow` and `trust = true`.
- Pre-tool hooks (ADR-0012): `[[hooks.pre_tool_use]]` runs the
  operator's guard before a model tool call on Claude Code's measured
  PreToolUse contract (stdin JSON; deny by `permissionDecision` or
  exit 2). A deny is a floor before the approval ladder and the reason
  is returned to the model; anything else fails open with a warning.
  Global config only; `hook_denied` in the transcript.
- Session-start, prompt-submit and session-end hooks (ADR-0014):
  `[[hooks.session_start]]`, `[[hooks.user_prompt_submit]]` and
  `[[hooks.session_end]]` on Claude Code's measured contracts. Context
  a hook prints rides the next turn as a `hook` attachment quoted as
  data; a prompt hook can refuse the prompt (erased, reason shown); a
  start or end cannot. `/clear` fires end then start.
- Skills (ADR-0011): Claude Code's `SKILL.md` format read as-is from
  `~/.config/lagent/skills/<name>/` and a trusted project's
  `.claude/skills/<name>/` (pinned; a changed one stays out until
  re-trusted). One catalog line per skill rides the runtime-facts
  message; `load_skill` returns a skill's body or supporting file,
  confined to its directory and sent unwrapped as the operator's own
  instructions; `/skill <name> [args]` sends a skill by hand and
  `/skills` lists them. The bench gains `skill-follow`.
- `[llm].reasoning_effort` (and `LAGENT_REASONING_EFFORT`) rides every
  request verbatim when set; the OpenAI vocabulary, which LM Studio
  maps to the model's thinking on/off. Measured on the bench
  (ADR-0009): thinking on completed 18/18 against 17/18 with no empty
  completion, at twice the wall time; the default stays unset.

### Decided

- **The model tier of auto-approval is not adopted** (ADR-0010). Over
  36 bench runs the rule tier's Review calls were four to five
  write-lane verification shells the read lane now runs unasked; a
  verdict would cost seconds per call on the one local model, judging
  its own proposal. The ladder stays Safe / Review / Block, and the
  RFP's Phase 2 item closes as rejected on measurement.

### Fixed

- A re-send after an empty completion now carries one transient line
  ("Reply now: give your final answer as text, or call a tool."),
  never stored: replayed ten times, the identical request came back
  empty 10/10 right after a compile error from the verification run
  and 0/10 with the line (ADR-0007, second amendment).

## [0.2.0] - 2026-09-12

### Added

- **The task bench** (ADR-0006). `go run ./bench run` measures the
  runtime on six fixture tasks (search then answer, read then edit, a
  change across files, a shell count, an MCP lookup against the bench's
  own fixture server, an image to look at) in isolated homes and state
  roots, reads each run's transcript, and `bench report` prints
  completion, no-tool answers and the medians of rounds, tool calls,
  prompt tokens and wall time per task and configuration. Phase 2
  changes are measured against it.
- **The runtime supplies routes, not rules** (ADR-0008, the RFP's
  local-oriented prompt revision). Every `shell_exec` runs with the
  `[sandbox.scratch_caches]` table pointed into the session scratch
  (shipped row: `GOCACHE = "go-build"`; the read lane and the approved
  lanes get separate directories), so `go run`, `go vet` and `go test`
  run in the read lane without approval (they failed in both lanes
  before: the sandbox denies the cache under `~/Library`); a denied
  call in a one-shot run is told that no one can approve it and what
  runs without approval, instead of "ask the user"; `list_tree
  dirs_only` on a flat directory lists its files instead of "(empty
  directory)"; and the system prompt states the lanes as they are and
  drops the "ask how to proceed" rule. Measured cause: a bench run that
  explained its fix in prose after the runtime's own chain of a failed
  cache write, an unattended denial and that rule. The table and the
  per-lane split follow gem-agent ADR-0084's review of the same design.
- **An empty completion is asked again, twice at most** (ADR-0007).
  The bench's read-edit task failed on a completion with no text and
  no tool call; the raw stream showed a mis-sampled tool-call opener
  routed into the reasoning channel, and the same request replayed
  came back empty about half the time. The runtime now re-sends the
  identical request up to twice, notes each re-send, and records every
  attempt (`assistant_empty` carries `retried`); a third empty
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
