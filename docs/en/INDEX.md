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
  images declare their box: the counter is told, never measures
  (**Accepted**, implemented; four verification passes, the fourth of which
  was a real terminal and found what the other three could not; gem-agent
  ADR-0089 is the same decision on the other side): ADR-0005 settled how an
  image reaches the model, and [ADR-0021](adr/0021-an-images-bytes-never-become-a-path.md)
  — deferred out of §5 — settled how one reaches the operator. `emit` counts the physical rows of
  every line and the bottom pin rests on that count, but `ansi.StringWidth`
  is 0 for all three payload families and `physicalRows` floors at 1, so the
  shortfall is N-1. Measured on gem-agent's probes — they measure a
  terminal, not a runtime, so they are not ported — with the regime arranged
  from the terminal's own height and a plain control at the same fill in
  every run: iTerm2 3.7.2 strands three frames for three drawn images and
  tmux 3.7c three for three rendered sixels, while the payloads tmux
  swallows stay clean in the same run and a screen not yet full takes no
  damage. So the emitter declares the box, `physicalRows` is told it in
  place of its floor of 1, and the declaration covers COLUMNS too, for the
  row count's sake rather than the renderer's. What differs from the other
  side is the lane: `newGlamourRenderer` renders the reply as one piece, so
  this creates a segment lane with an image as its only member, and porting
  `internal/diagram` is explicitly not part of it (ADR-0025 later partitioned
  the reply, for diagrams drawn as pictures). **What may be drawn is
  NOT settled here**: that decision was written and refuted three times — a
  model-named path bypasses `PathJudged`, the intake's path can be
  pre-empted by a symlink because `write` short-circuits on `os.Stat` and
  every call hands the server the work directory, and the third draft's
  bytes have no carrier since `render` returns a string and `Tool.Run` is
  string-only — so it is deferred to its own ADR with the one constraint
  that held against all three: **the view layer opens no file.** Decision 4
  rests on sixel's inability to declare a row count alone; the supply-chain
  rule is not adjudicated here, and this line said it was — the record
  removed that reason rather than settling a question it is not the place
  to settle. `internal/tui`'s
  scrollback accounting is now on both shared-mechanism lists, and the
  `WithAutoStyle` hazard note is recorded as inherited, which it is
- [`ADR-0021`](adr/0021-an-images-bytes-never-become-a-path.md) — an
  image's bytes reach the screen without ever becoming a path
  (**Accepted**, implemented; gem-agent ADR-0090 is the same decision
  on the other side): ADR-0020 §5 deferred the source after three drafts
  and three refutations, keeping one constraint — the view layer opens no
  file, because a read it performs is not a tool call and `pathJudgedTools`
  is keyed on tool name. That rules out the saved path: `write`
  short-circuits on `os.Stat` while every call hands the server the work
  directory in `_meta`, so a local server child can plant a symlink at the
  content-addressed name and the runtime writes nothing; reaching that file
  through `view_image` is contained because the agent resolves symlinks
  first, and a view-layer open would not be. The third draft's alternative
  had no carrier — `render` returns a string and `Tool.Run` is string-only
  — but the channel it needed already exists here too, and is the same one:
  the agent loop sends to the UI mid-call. So `mcpIntake` gains one
  optional sink beside `workDir`, called with the decoded bytes as the
  block is taken in and wired to `prog.Send`; the string contract does not
  move. The sink is not nil outside an interactive TUI — an earlier draft
  promised that and the ordering does not allow it, because MCP connects
  before the runtime knows whether it has a UI; it is inert there instead,
  dropping what it is given. An image was drawn if and only if the intake saved AND described it (withdrawn by ADR-0022),
  since a block the response budget refuses is already neither and drawing
  one would put a picture on screen that the session's record does not
  contain. `image.DecodeConfig` supplies the aspect ratio and doubles as
  the validator; the payload is base64, whose alphabet holds no ESC or BEL.
  The ceiling is 2 MiB decoded, from a measurement on the counter both
  runtimes share — about 3.6 ms per MiB, with the string held in three
  places at once. Named as a new surface: a server can now put a picture on
  the operator's screen. Implemented here after gem-agent, which had the
  lane already; the decision was taken on both sides at once so that
  neither holds it alone, and the port carried the erase gem-agent's first
  real-terminal run made necessary. **§2's source is withdrawn by
  [ADR-0022](adr/0022-showing-is-an-act-of-output.md)**
- [`ADR-0022`](adr/0022-showing-is-an-act-of-output.md) — showing an image is
  an act of output, not a side effect of a tool result (**Accepted**,
  implemented; gem-agent ADR-0091 is the same decision on the other side): an MCP
  image block is how a tool result carries an image into the MODEL's context,
  and MCP says who content is for with an `audience` annotation this runtime
  drops at the parser — measured across the 24 registered servers, four can
  emit an image block and none sets an audience, so ADR-0021's condition was
  never "the server asked for this to be shown" but this runtime inferring it.
  Two of those four return a screenshot so the MODEL can look, and every such
  inspection also put a full-size picture into the operator's scrollback,
  competing with the model's own reply. Meanwhile the common case — a
  generated artifact — comes back as a path under the work-directory
  contract, so the intake route never fires for it, and a model asked to show
  one reaches for `shell_exec` with `open`, which the read lane denies at the
  kernel. So the intake draws nothing; showing becomes an act of the model's
  output through a tool whose read happens in the TOOL layer, where a
  model-named path is judged like any other (`view_image`'s own confinement),
  and the operator keeps a direct route in `/show <path>`, the trust line
  ADR-0005 already draws for `@<image>`. The lane, the declared box, the
  erase, the ceiling and the probe are ADR-0020's and unchanged; only the
  source moves
- [`ADR-0023`](adr/0023-promotion-to-cli-series.md) — an agent runtime
  in daily use: repositioning and promotion to cli-series (**Accepted**;
  gem-agent ADR-0061 is the same move on the sibling runtime): the
  charter's measurements have been taken and the operator uses lagent
  daily, so its identity becomes the RFP's problem statement — work that
  should not go to a cloud API — and the bench stays as the instrument
  that decides changes. Promoted on the evidence of daily use, with no
  bar written after the fact. cli-series rather than the lite-series
  RFP §6 anticipated, because lite-series' conventions (pipeline tools,
  no interactive UI, Linux and Windows builds, `LITE_<PROJECT>_`) would
  each need a standing exception, while cli-series already holds
  gem-agent and llm-cli. The cli-series stability contract applies from
  now; the health-check procedure is not built and is not a condition
- [`ADR-0024`](adr/0024-outside-text-is-made-inert-for-the-terminal.md) —
  text from outside the runtime is made inert before the terminal sees it
  (**Accepted**; ported from gem-agent ADR-0093): the model's words, a tool
  call's detail and the approval dialog reached the terminal with their
  escape sequences intact — measured on a real terminal (tmux 3.7c) with a temporary copy of gem-agent's probe, 61 of 95 deliveries acted before and 0 after. Control characters are
  removed, not sequence bodies, once at the TUI's ingress (every message
  string by reflection, every shown `Options` callback and field), the
  Markdown renderer's output is held to the SGR it writes and closed at the
  reply's end because goldmark decodes `&#27;`, and the plain REPL and `-p`
  go inert only when the stream is a terminal (the RFP's stdout contract
  amended). No box-art hold: this runtime draws no box-art diagrams
- [`ADR-0025`](adr/0025-mermaid-fences-render-as-pictures.md) —
  mermaid fences render as pictures where the terminal can draw them
  (**Accepted**, implemented; ported from gem-agent ADR-0092): on iTerm2 and kitty a fence
  in a reply is drawn by mermaid-render from the source as written, one em of
  diagram text per terminal line, the width from the cell aspect read with an
  ioctl; tall pictures scroll, in bands on kitty only. Every failure is the
  source with a note. Elsewhere the fence stays source — no box-art lane is
  added. Where pictures draw the reply is partitioned (ADR-0020 amended, its
  A3 answered), and a payload
  reaches the terminal beside the renderer's ADR-0024 hold, as a tool image
  does. The font is `[tui.diagram]`, read only where images draw; a bad
  setting is a banner warning. Known limitation: narrowing the window loses
  the pictures on screen
- [`ADR-0026`](adr/0026-a-narrowing-sweeps-the-frame-not-the-screen.md) —
  a narrowing sweeps the frame's rows, not the screen (**Accepted**,
  implemented; ported from gem-agent ADR-0094): the screen clear on a width
  shrink lost the pictures on the screen and piled empty screens into the
  scrollback. A shrink now erases only the rows the drawn frame gained by
  re-wrapping, through a writer that extends the renderer's next flush; frame
  rows end at their text; and while size reports keep coming the frame is
  drawn narrow, because the terminal runs ahead of the width it reports.
  Resolves ADR-0025's known limitation. The writer's measurement surface,
  read only by gem-agent's probe, is not ported
- [`ADR-0027`](adr/0027-box-art-where-no-picture-draws.md) —
  box art where no picture draws (**Accepted**, implemented; the counterpart of gem-agent
  ADR-0095): where the TUI draws no images, a flowchart, sequence or ER fence
  is drawn as text art by mermaid-render's `raster.RenderText`, with the TUI's
  cell measure. That engine is organization code and checks every render on
  its grid, so ADR-0025's two reasons for declining a box-art lane are gone.
  The art is its own segment past glamour and is held to no escapes
  (`inertArt`, amending ADR-0024's "no box-art hold"). Pictures are unchanged
  where images draw; `-p` and the plain REPL stay source
- [`ADR-0028`](adr/0028-loaded-schemas-ride-the-conversation.md) —
  a loaded server's schemas riding the conversation (**Rejected**, measured
  on an unmerged prototype): returning the schemas in `mcp_load`'s result and
  calling through a `mcp_call` proxy keeps the prompt cache, but on github's
  44 tools the model mixed up sibling tools' arguments (104/120 against
  80/120). ADR-0004's proxy alternative is closed; the bench keeps the
  `tickets` persona and the `mcp-load` suite
- [`ADR-0029`](adr/0029-what-was-not-shown-is-reachable.md) —
  what the runtime did not show is reachable and countable (**Accepted**,
  ported from gem-agent ADR-0096 Part A): a spilled MCP result is previewed
  by head and tail with byte spans; `read_file` reads by byte `offset`/`length`,
  so the tail of a saved single-line result is reachable; `shell_exec` keeps
  head and tail and saves the whole output to the work directory (not in the
  operator lane); `search_files` counts every file it did not search. Part B
  is not ported — gem-agent measured it and found no room
- [`ADR-0030`](adr/0030-a-resumed-session-says-when-it-began.md) —
  the session's date is captured once, and a resumed session says when its
  conversation began (**Accepted**, ported from gem-agent ADR-0097): the facts
  message re-sent on resume said "session started" on the resume day beside the
  restored one's real start, and an MCP reload re-read the clock. Now a resumed
  session reads `- resumed: <day>; the conversation above began on <day>`
- [`ADR-0031`](adr/0031-mlx-serve-is-a-supported-backend.md) —
  mlx-serve is a supported backend (**Accepted**, implemented, on the task bench
  and a server measurement with the same weights under both servers):
  `[llm].provider =
  "mlxserve"` reads the window from `/v1/models` and reports a model id the
  list does not carry, since the server answers any name with the model it
  has loaded. The default stays `lmstudio`. Records the wire as measured
  (whole-call tool deltas, `cached_tokens`, no `reasoning_tokens`,
  `reasoning_effort` unvalidated), the measurements (the speed is the MTP
  head of mlx-serve's own build, not the runtime; long context holds) and the
  residual risk of running a community server. Amends ADR-0007: a
  whitespace-only completion is empty
