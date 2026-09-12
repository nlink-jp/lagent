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
  skip such a file and say so; one list, in `internal/sandbox`
- [`ADR-0016`](adr/0016-the-kernel-reads-the-file.md) —
  the kernel reads the file: the file tools' reads run in a child under
  `sandbox-exec`, so a credential open is refused by the kernel and the
  Go matcher stops being the boundary; a refusal becomes the operator's
  prompt and the read is re-issued in process on approval. Measured:
  `cat` and `stat` on `.env` refused, `.env.example` read, `grep -r`
  skips the one file, `ls -a` still lists the name, 18.8 ms per spawn.
  Because names are listed and content is refused, ADR-0015 §2 is
  withdrawn: `credentialTally` and the enumeration tools' credential
  code are deleted, and the walks' spelling defect dissolves with them
- [`ADR-0017`](adr/0017-the-runtime-hides-only-its-own.md) —
  the runtime hides only its own: the environment scrub covered one
  child of six and passed `OPENAI_KEY` while catching `NPM_TOKEN`, and
  an allowlist would only move the unbounded list, so it is deleted and
  the operator's environment is not touched. The one bounded set is
  lagent's own namespace, so the prefix rule is inverted — every
  `LAGENT_*` is removed from every child except the three exports, a
  partition a test closes. `LAGENT_API_KEY`, the variable R01 named, is
  in the removed half; the configuration-file route stays the residue
  ADR-0015 accepted
