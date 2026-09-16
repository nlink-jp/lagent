# Documentation Index

Entry point for lagent's maintainer-facing documentation. For
user-facing material see [`README.md`](../../README.md).

Japanese mirror: [`INDEX.ja.md`](../ja/INDEX.ja.md). `scripts/docs-mirror-check.sh`
enforces the structural half in `make check` — every `docs/en` file has its
`docs/ja` counterpart and back, the ADR catalogue is complete and ordered in
both, and the identifiers of each pair agree. Prose parity is the author's job.

## Specification

- [`lagent-rfp.md`](lagent-rfp.md) — the canonical spec: problem
  statement, functional surface, scope boundaries, phase plan. Features
  outside it need an ADR; where an ADR or the build amended a section,
  a note stands in place and the reference documents are current.

## Reference

Current behaviour, updated in place as the code changes.

- [`reference/configuration.md`](reference/configuration.md) — install,
  the config file, precedence, the command and flag tables
- [`reference/architecture.md`](reference/architecture.md) — package
  layout, the backend, the turn loop, approval, the round ladder,
  persistence, and what is not here
- [`reference/bench.md`](reference/bench.md) — the task bench: how to
  run it, isolation, the tasks, what is measured, comparing
  configurations and the reference runtime

Feature references (interface, tools, approval, sessions, integration)
follow as the Phase 1 measurements settle the surface.

## ADRs

Point-in-time design decisions. Immutable once accepted; a changed
decision gets a new ADR that supersedes the old one (typo and link fixes
excepted).

- [`ADR-0001`](adr/0001-porting-sources-pinned.md) — gem-agent and
  llm-cli are porting sources, pinned by commit: a new repository, not a
  fork; packages come over one at a time with their source recorded, and
  nothing tracks the sources afterwards
- [`ADR-0002`](adr/0002-features-not-reproduced.md) — the gem-agent
  features lagent does not reproduce, why each is bound to Vertex AI or
  Google Cloud, and what lagent does instead
- [`ADR-0003`](adr/0003-session-facts-ride-the-conversation.md) — the
  system prompt is byte-identical across sessions; the isolation tag,
  the work directory and the start date ride the runtime's opening
  message, so the server's prefix cache survives a new session and a
  `/clear` (measured: 118 s against 2 s with 243 MCP tools)
- [`ADR-0004`](adr/0004-mcp-tools-load-on-demand.md) —
  MCP tools are advertised on demand — a catalog in the facts message,
  one `mcp_load` tool, native calls after a load; `[mcp].preload`,
  `[mcp].advertise = "all"` as the baseline
- [`ADR-0005`](adr/0005-images-reach-the-model.md) — images reach the
  model: a dropped image path attaches without an `@` (escaped spaces
  read), `view_image` returns, and a reference that did not attach is
  said to the model
- [`ADR-0006`](adr/0006-measurement-bench.md) — a task bench measures
  the runtime before Phase 2 changes it: fixture tasks, configuration
  files, isolated runs, the transcript as the measurement, the
  reference runtime on the same tasks
- [`ADR-0007`](adr/0007-empty-completion-asked-again.md) — an empty
  completion is asked again, twice at most: the raw stream showed a
  mis-sampled tool-call opener routed into the reasoning channel, a
  coin flip at the point it strikes, and the same request re-sent
  answers normally
- [`ADR-0008`](adr/0008-routes-not-rules.md) — the runtime supplies
  routes, not rules: toolchain caches ride the session scratch so builds
  run in the read lane, an unattended denial names the route,
  `list_tree` says what it saw, and the prompt drops the rules those
  replace (the local-oriented prompt revision)
- [`ADR-0009`](adr/0009-thinking-is-an-operator-key.md) — thinking is
  an operator key: `[llm].reasoning_effort` is sent verbatim, the
  default stays unset until the bench decides, reasoning content is
  never stored
- [`ADR-0010`](adr/0010-no-model-tier.md) — the model tier of
  auto-approval is not adopted: the bench's Review-tier calls were
  write-lane verifications the read lane now runs, a verdict would cost
  seconds per call on one local model that would be judging itself,
  and the operator's rows and lanes already cover them
- [`ADR-0011`](adr/0011-skills.md) — skills are loaded, in Claude
  Code's format, from lagent's own directory: `~/.config/lagent/skills`
  and the project's `.claude/skills` (trusted and pinned), one catalog
  line per skill in the facts message, `load_skill` results sent
  unwrapped and confined, `/skill` and `/skills`, `allowed-tools` ignored
- [`ADR-0012`](adr/0012-pre-tool-hooks.md) — pre-tool hooks are the
  operator's control outside the model: `[[hooks.pre_tool_use]]` on
  Claude Code's measured contract, a deny is a floor before the ladder,
  anything else fails open with a notice, global config only
- [`ADR-0013`](adr/0013-memory-rides-the-facts-message.md) — agent
  memory rides the runtime-facts message: a directive in the
  instruction section was acted on 0/18 and a facts-message line 5/6,
  so recall goes there; two scopes under the state root, the operator
  writes with `/remember`, the model proposes through gated tools
- [`ADR-0014`](adr/0014-context-and-end-hooks.md) — the hook set is
  gem-agent's: `session_start`, `user_prompt_submit` and `session_end`
  join `pre_tool_use` on Claude Code's measured contracts; injected
  context rides the data lane as a `hook` attachment, a prompt can be
  refused, a start or end cannot (amends ADR-0012 §1)
- [`ADR-0015`](adr/0015-credential-reads-are-operator-only.md) —
  credential paths are operator-only for the read tools too:
  `read_file`, `file_info` and `view_image` on a path the lanes deny are
  a Review only the operator answers, on the real path, in every mode,
  and `-p` denies them; `search_files`, `list_files` and `list_tree`
  hide nothing — ADR-0016 withdraws §2, since the kernel lists names
  while refusing content; one list, in `internal/sandbox`
- [`ADR-0016`](adr/0016-the-kernel-reads-the-file.md) —
  the kernel reads the file (**Accepted**, implemented): the file tools'
  reads run in a child under
  `sandbox-exec`, so a credential open is refused by the kernel and the
  Go matcher stops being the boundary; a refusal becomes the operator's
  prompt and the read is re-issued in process on approval. Measured:
  `cat` and `stat` on `.env` refused, `.env.example` read, `grep -r`
  skips the one file, `ls -a` still lists the name, 18.8 ms per spawn.
  Because names are listed and content is refused, ADR-0015 §2 is
  withdrawn: `credentialTally` and the enumeration tools' credential
  code are deleted, and the walks' spelling defect dissolves with them
- [`ADR-0017`](adr/0017-the-runtime-hides-only-its-own.md) —
  the runtime hides only its own (**Accepted**, implemented): the
  environment scrub covered one
  child of six and passed `OPENAI_KEY` while catching `NPM_TOKEN`, and
  an allowlist would only move the unbounded list, so it is deleted and
  the operator's environment is not touched. The one bounded set is
  lagent's own namespace, so the prefix rule is inverted — every
  `LAGENT_*` is removed from every child except the three exports, a
  partition a test closes. `LAGENT_API_KEY`, the variable R01 named, is
  in the removed half; the configuration-file route stays the residue
  ADR-0015 accepted
- [`ADR-0018`](adr/0018-the-injection-bench-scores-argument-values.md) —
  the injection bench scores argument values, against a benign twin
  (**Accepted**, design only): measured on this model, loud payloads
  ("discard all instructions") never get through wrapped or not, so a
  bench built from them reports perfect resistance after the defence is
  removed; payloads that accept the task and dictate one output field
  get through 92-100% unwrapped, and wrapping cuts one to 0.3%, another
  to 14%, and a third not at all. So the agent task plants its payload
  in a tool result, scores the value of an argument rather than prose,
  ships a benign twin, varies the targeted element, and leaves rates to
  the single-turn harness
- [`ADR-0019`](adr/0019-the-caller-names-the-work-dir.md) — the caller
  names the work directory: `_meta` on every `tools/call` (**Accepted**,
  implemented; gem-agent ADR-0088 is the same decision on the other
  side): organization ADR-021 settled that a server producing files
  takes its destination as a per-call `work_dir` argument, and ten fleet
  servers came to require it — the contract places an obligation on the
  calling side too. Measurement of the four calling runtimes is why the
  argument exists at all: neither MCP `roots` nor the environment
  reaches half of them. The contract's second channel, the request's
  `_meta["jp.nlink/work_dir"]`, exists for the runtimes we write
  ourselves — it is schema-blind, so it rides every `tools/call` without
  knowing any tool's schema, and a model that forgets the argument still
  leaves the server a destination this session can read back.
  `LAGENT_WORK_DIR` does not serve: it is an environment variable for
  child processes, not a protocol value, and reading it needs
  `${LAGENT_WORK_DIR}` in the registration entry, where the sibling
  runtime expands an undefined variable silently to the empty string. So
  `mcp.Client` carries the session work directory and attaches the key
  to `params._meta` on every call; a session with no work directory
  attaches nothing, because an empty hint is worse than none — a server
  would take it for an answer; the value is a `NewStdio` parameter
  rather than a global or a setter, since it does not change while the
  client lives; and the model's own argument always wins, `_meta` being
  read only when the argument was absent (ADR-021 §2's resolution
  order). chrome-pilot's registration loses `--workspace-root
  ${LAGENT_WORK_DIR}`, the only line that made gem-agent's and lagent's
  `mcp.json` two separate files
- [`ADR-0020`](adr/0020-inline-images-declare-their-height.md) — inline
  images declare their height: the counter is told, never measures
  (**Proposed**, not implemented; gem-agent ADR-0089 is the same decision
  on the other side): ADR-0005 settled how an image reaches the model and
  nothing has settled how one reaches the operator — an MCP server's
  screenshot is, on this surface, a path in a line of text. `emit` counts
  the physical rows of every line and the bottom pin rests on that count,
  but an image is a line the counter cannot see: against x/ansi v0.11.6
  `ansi.StringWidth` is 0 for iTerm2 `OSC 1337`, kitty `APC _G` and sixel
  `DCS q` alike, while `ansi.Hardwrap` leaves all three byte-identical so
  `wrapForScrollback` shears nothing. Measured with gem-agent's
  `tools/rowprobe` on iTerm2 3.7.2 (16/16 cursor reports, a property of
  the terminal rather than the runtime, so the tool is not ported): the
  declared box is reserved exactly in both dimensions whatever the picture
  does inside it — a 16:9 image in a 40x12 box draws about ten rows and
  occupies twelve — so no aspect-ratio derivation is needed; the cursor is
  left on the image's LAST row, which makes the raw delta one short and
  made a terminal honouring every declaration read as one honouring none
  on the first pass; and an undeclared image takes its native size,
  recoverable only from the cell pixel size. So the emitter declares the
  height and `physicalRows` is told it. The difference from the other side
  is the lane: gem-agent's `diagram.Split` already partitions a reply,
  while `newGlamourRenderer` here renders it as one piece — so this
  creates a segment lane with an image as its only member, and porting
  `internal/diagram` is explicitly not part of it. Only protocols that can
  declare a row count are taken (sixel is refused for needing a derivation
  and a community encoder); the capability is probed once before Bubble
  Tea owns stdin, for the reason `newGlamourRenderer` already records
  about `WithAutoStyle` — a terminal's reply becomes phantom user input —
  and an unanswered probe means no capability, because a cursor report
  carries no tag and an abandoned reply is misfiled rather than lost.
  ADR-0005's bare-path grammar is deliberately not reused: it reads the
  operator's input, drawing reads the model's output. Unresolved and
  written down rather than claimed: iTerm2 answered the next cursor report
  0.8-1.5s after a 2.4 KB payload, which bounds its parser and not its
  drawing, and tool output still reaches the terminal without ANSI
  stripping — pre-existing, not widened, not repaired here
