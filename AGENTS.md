# AGENTS.md — lagent

Sandboxed CLI agent runtime on a local LLM served over an
OpenAI-compatible API (LM Studio, Ollama). A separate product line from
gem-agent built on the same design; gem-agent is the porting source
(ADR-0001), and the features it has that lagent does not reproduce are
listed in ADR-0002. Experimental, lab-series; RFP Phase 1
(the core loop, tools, sandbox, MCP, sessions, TUI) is in.

- **Module:** `github.com/nlink-jp/lagent`
- **Series:** lab-series (target; developed in `_wip/lagent` until integration)
- **Spec:** `docs/en/lagent-rfp.md` / `docs/ja/lagent-rfp.ja.md` (canonical)
- **Docs entry point:** `docs/en/INDEX.md` / `docs/ja/INDEX.ja.md`

## Build / test

| Task | Command |
|------|---------|
| Build | `make build` → `dist/lagent` (never `go build` directly) |
| Test | `make test` (or `go test ./...`) |
| Lint | `make lint` (golangci-lint, org config in `.golangci.yml`) |
| Vet + lint + test + docs mirror + release gate + build | `make check` |
| Docs mirror only | `make docs-check` |
| Release gate's own test | `make gate-check` |
| Release binary | `make build-all` (darwin/arm64 only; signs the binary) |
| Release archive | `make package` → `dist/lagent-vX.Y.Z-darwin-arm64.zip`, notarized |
| Release gate | `make verify-release` — refuses a zip with no notarisation marker, one rebuilt after its marker, one that does not unpack, or one whose binary does not run or reports another tag's version |
| Homebrew formula | `make brew` after `make package` — generates `Formula/lagent.rb` from the built zip into the local `nlink-jp/homebrew-tap` checkout and pushes it (`make brew-print` renders only) |
| Task bench (ADR-0006) | `make bench-build` (→ `dist/bench-mcp`), then `go run ./bench run --bin dist/lagent --configs baseline` and `go run ./bench report <dir>` — minutes on the local server, never part of `make check` |

Version is injected via `-X main.version` from `git describe` — never edit the
`version` var default.

## Structure

```
config.example.toml  shipped config template (every section: [llm], [model], [sandbox], [agent],
                     [mcp], [tui], [approval]; pinned by a loader test)
lagent.example.project.toml  shipped <project>/.lagent.toml template
mcp.example.json     shipped MCP server template (pinned by a loader test)
main.go            entry point (package main, calls cmd.Execute(version))
cmd/               cobra root command, REPL loop, wiring, system prompt; `version`,
                   `sessions`, `trust`, `workdirs` subcommands; ask_user, MCP intake,
                   the MCP advertiser and mcp_load (cmd/mcpload.go, ADR-0004); skill
                   discovery, load_skill, /skill and /skills (cmd/skills.go, ADR-0011);
                   hook config → runner (cmd/hooks.go, ADR-0012); save_memory / delete_memory,
                   /memory, /remember, /forget (cmd/memory.go, ADR-0013)
internal/config/   strict-decode TOML + env/flag precedence ([llm], [model], [sandbox],
                   [agent], [mcp], [tui], [approval]); the policy file
internal/llm/      Backend interface + the OpenAI-compatible client (stdlib net/http,
                   hand-written SSE, tool-call assembly, retry, per-provider context probe)
internal/agent/    tool-calling loop, approval dispatch, nonce wrapping, history,
                   the rule-tier auto-approve ladder, the round ladder
internal/tools/    the nine built-in tools (list_files, list_tree, search_files, read_file,
                   file_info, view_image, write_file, edit_file, shell_exec), path
                   confinement, lane-aware exec injection, Register
internal/bounded/  the one place a read, listing or process output is capped —
                   every primitive returns the `more` fact
internal/archtest/ AST tests pinning confined opens, bounded reads and the
                   single decision point
internal/mcp/      .mcp.json parsing + stdio JSON-RPC client (kill-and-respawn)
internal/mcpfilter/ the one predicate behind `[mcp] exclude`
internal/ignore/   ignore-aware enumeration: builtin dir list + full gitignore matcher
internal/risk/     rule tier of the auto-approve ladder (pure, no model); exact for
                   file-tool paths (persistent files OperatorOnly), a Block floor only
                   for shell text — the lane decides the rest; the shared lists come
                   from internal/sandbox
internal/policy/   per-tool approval policy, pure resolver
internal/uitext/   ja/en UI string catalogs: completeness enforced by test —
                   new operator-facing strings go in BOTH catalogs or make check fails
internal/banner/   the lines printed before the operator has typed: a line earns a
                   place only if nothing else will say it; Sample() is the read-through
internal/statedir/ shared per-project state convention: root (LAGENT_STATE_DIR overrides it),
                   escape, .project marker
internal/workdir/  per-session work directory: layout under the state root, sweep
                   report, empty-dir removal; LAGENT_WORK_DIR is exported at startup
internal/mention/  @-reference parsing (files, directories, images), project-confined
                   resolution, completion
internal/instructions/ AGENTS.md / AGENT.md / CLAUDE.md / GEMINI.md discovery
                   (ancestor walk, stops at $HOME)
internal/memory/   facts recalled across sessions (ADR-0013): global + project scope under
                   the state root, budgeted, FactsLines for the runtime-facts message
internal/hooks/    operator pre-tool hooks on Claude Code's measured contract (ADR-0012):
                   PreToolUse payload on stdin, deny by JSON or exit 2, fail-open otherwise
internal/skills/   Claude Code SKILL.md discovery (global ~/.config/lagent/skills +
                   project .claude/skills), confined Body/File reads, the catalog lines
internal/trustpin/ content pins for the agent-facing files and the persistent-file
                   snapshot; cmd/pins.go applies them to the grant
internal/sandbox/  SBPL profile generation per lane (read/write/operator), the shared
                   scratch / persistent-file / credential lists, sandbox-exec wrapping
internal/approve/  MITL gate (y/n/N/a + session allowlist; N = deny with a typed reason)
internal/session/  JSONL transcript: logger + resume loader; usage records in the
                   gem-usage-lens shape; LAGENT_SESSION_ID is exported at startup
internal/repl/     paste-safe input reader (plain REPL, non-TTY fallback)
internal/tui/      Bubble Tea inline TUI: model, approval gate, settings panel
bench/             the task bench (ADR-0006): runner + report (package main), configs/<runtime>/*.toml,
                   tasks/<name>/{task.toml,testdata/}, mcpfixture/ (the stdio MCP fixture server);
                   _results/ is ignored by git (and by go's ./..., which is why the underscore)
scripts/           codesign-darwin.sh / notarize-darwin.sh (org templates, verbatim),
                   docs-mirror-check.sh, verify-release-selftest.sh (make check)
docs/en/, docs/ja/ INDEX + reference/ + adr/ + the RFP (en: no suffix; ja: .ja.md)
```

Every package under internal/ that came from gem-agent says so in its
package doc comment, with the source commit (ADR-0001). `internal/llm`
is written new (its type shapes follow gem-agent's, as its doc comment
says); the others are ports minus the ADR-0002 features. A ported
comment keeps the source's design references, written `gem-agent
ADR-NNNN`; a bare `ADR-NNNN` is one of this repository's records, and
`internal/archtest` fails on a bare number no file under `docs/en/adr`
answers.

## Gotchas

- **macOS-only by design** — isolation is built on sandbox-exec, as in
  gem-agent. Do not add linux/windows targets to the Makefile.
- **gem-agent is a source, not an upstream.** Bring a package over with
  its commit recorded; do not add a feature because gem-agent has it.
  Anything in ADR-0002's list stays out — code, config keys, error text
  and docs alike. The earlier fork failed exactly by keeping those seams.
- **`--version` must always answer** and `version` must print the same
  line (pinned by `cmd/root_test.go`) — a Homebrew formula's `brew test`
  runs it.
- **A dropped image path attaches without an `@`** (ADR-0005), and a
  reference that could not attach is told to the model as a `missing`
  attachment rendered outside the nonce tag. Only images are taken
  bare; extend `mention.bareImageRefs`, never widen it to text files.
- **No string for a feature this runtime does not have.** The catalog,
  `/help`, tool descriptions, notes and error text name only what is
  here; a leftover from the porting source (`/readonly auto`, `/compact`,
  "the model tier") is a seam ADR-0001 forbids. Before a
  release, grep every string literal in cmd/ and internal/ for the
  ADR-0002 and Phase 2 feature names, and grep the `uitext.Messages`
  fields for ones no code reads — the completeness test only checks
  that both languages agree.
- **MCP tools are registered always, advertised on load** (ADR-0004).
  `mcpAdvertiser` owns which servers the model can see; `agent.Options.
  Advertise` filters the declarations and refuses a call to a hidden
  registered tool before any gate. The catalog rides the facts message,
  so `AnnounceSession` runs after the MCP connect (startup, `/clear`,
  `/mcp reload`), never before. Anything that changes the connected set
  calls `adv.setInventory` then `ag.RefreshTools()`.
- **A read-lane network failure is explained even when silent.** The
  read lane has no network; `curl -s` prints nothing and exits 6, so the
  text-keyed `sandbox.DeniedHint` never fired and the model retried the
  same lane three times before giving up (measured 2026-09-10).
  `internal/tools/netclient.go` keys the hint on a finite list of
  network clients (and git's remote subcommands) as well — extend the
  list, never turn it into a pattern over the command text.
- **The system prompt is byte-identical across sessions** (ADR-0003).
  Anything per-session — the isolation tag name, the work directory,
  the start date — goes through `Agent.AnnounceSession` as the
  runtime's opening message, never into `buildSystemPrompt`; the server
  renders every tool schema after the system text and re-processes all
  of it when one byte there changes (measured: 118 s against 2 s with
  243 MCP tools). `TestSystemPromptIsIdenticalAcrossSessions` pins it.
  A second `system` message does not help: it is folded into the same
  turn.
- **Prompt processing on a local model is the cost to design around**:
  measured about 580 tokens/s on the reference machine, so a cache miss
  on a 20k-token prefix is 30+ seconds of silence before the first token,
  while an identical prefix re-sent hits LM Studio's KV cache in about a
  second. Anything that rewrites history (compaction) discards the cache.
- **Tool-call arguments arrive as one chunk** in streaming; there is no
  liveness signal while a large `write_file` argument is generated. Do
  not add a stall timeout on that path.
- **`finish_reason=length` is a partial result** — surface it, never
  drop the turn (a fork-era defect). The agent notifies and keeps the
  text; `emptyResponseError` names the reason when nothing arrived.
- **Skills come from lagent's own directory, in Claude Code's format**
  (ADR-0011). `~/.claude/skills` is never read — copy a skill into
  `~/.config/lagent/skills/`; a link fails in the lanes because the
  kernel resolves it into the denied `~/.claude`. Project skills load
  only from a trusted project and only while their pin matches. The
  catalog rides the facts message, never the system prompt. `load_skill`
  is the one tool whose results are sent unwrapped;
  `internal/archtest` pins `InstructionTools` to that one name, and a
  second exemption needs its own ADR.
- **Pre-tool hooks are a floor, and their contract is measured, not
  documented** (ADR-0012) — `internal/hooks` denies on Claude Code's real
  stdout-JSON and exit-2 forms and fails open on everything else, with
  a notice. The hook runs in `execCallInner` after the advertise check
  and before `decide`; `hookDenied` is provenance the wrap layer and
  the attach branches read, never inferred from the result text. Never
  add an "allow" bypass: hooks tighten, the ladder decides. No settings
  row, no runtime toggle. One event; more take an ADR each.
- **A standing directive belongs in the facts message, not the
  instruction files** (ADR-0013, measured on the `pointer-*` bench
  tasks: 0/18 from `AGENTS.md` at any size, 5/6 from one facts line).
  Memory recall is `memory.FactsLines` appended to `sessionFacts` at
  every `AnnounceSession` (startup, `/clear`, `/mcp reload`); nothing
  about memory goes into `buildSystemPrompt`. `save_memory` /
  `delete_memory` are Review in `risk.Classify`, never Safe, and the
  operator's `/remember` bypasses no gate because the operator is the
  gate. A memory that is meant to drive behaviour is written as a
  pointer (a file, a command, a tool), the shape that measured.
- **There is no model tier, by decision** (ADR-0010).
  `agent.AutoDecision.ModelConsulted` is always false and stays for
  record-shape parity; the round checkpoint asks the operator or stops.
  Reopening it takes the measurement ADR-0010 names, not a flag.
- **The OpenAI client reads are bounded by hand** — `io.LimitReader` on
  error bodies and the probe, a 16 MiB scanner buffer on the stream —
  and `internal/archtest` allowlists them by name with the reason.
  A new read there needs the same.
- **The bench is the evidence for Phase 2** (ADR-0006). A prompt,
  tool-description or config change that is meant to change the model's
  behaviour is measured as one more configuration on the same tasks
  before and after. Fixtures live in `testdata/` so their `.go` files
  are never built or linted; each run's project sits under the run's
  own `HOME`, so this repository's `AGENTS.md` never reaches a run. A
  task the model could answer without the files measures nothing.
- **Toolchain caches are the operator's table, not a list in code**
  (ADR-0008 §1). `[sandbox].scratch_caches` maps a variable to a
  directory under the session scratch; `laneEnv` renders it with the
  read lane and the approved lanes apart (`go-build` /
  `go-build-approved`) because a build cache is trusted on read. The
  loader refuses loader variables and the ones the lane decides; a
  project file cannot set it. Do not add a toolchain in code — add the
  row to the example config.
- **A new config key means updating `config.example.toml`** — strict
  decode makes a stale template a startup error, so the loader tests parse
  the shipped template against the built-in defaults.
- **Docs are checked mechanically** — `make docs-check` fails on a missing
  en/ja mirror, an ADR absent from either INDEX, an INDEX link that does
  not resolve, a backticked identifier (tool name, flag, config key)
  present in one language only, an en/ja mermaid pair whose shape
  differs, or this file losing one of its sections.
