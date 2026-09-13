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

## The sibling runtime

gem-agent (`nlink-jp/gem-agent`, cli-series) is the porting source
(ADR-0001), not an upstream this repository tracks, and ADR-0002 lists
the features there that this runtime deliberately does not reproduce.

**A defect or a design change in a mechanism both runtimes have is
fixed in both, in the same piece of work.** That is the default, not a
follow-up: finish the sibling before calling the work done. The file
almost always has the same name there.

The mechanisms that are shared today: `internal/sandbox` (the lanes,
the scratch / persistent-file / credential lists, the file-read
profile and its child), `internal/risk`, `internal/tools` (path
confinement through `os.Root`, the walks, the caged reads),
`internal/bounded`, `internal/hooks`, `internal/mcp`,
`internal/trustpin`, `internal/archtest`, and the approval ladder in
`internal/agent` and `internal/approve`.

**This is not a rule to port features.** A feature gem-agent gains does
not arrive here by default — ADR-0002 decides that, and this repository
does not track gem-agent's later changes. What the rule covers is the
mechanisms both already have.

**Why it is written down.** Measured, 2026-09-13: the credential-read
redesign landed in both runtimes, and two defects in it — a search walk
with no credential judgment where the sandbox child could not be
installed, and an environment rule that missed four spawn sites — were
identical in both, so every fix had to be made twice anyway. Worse, the
tool descriptions of `list_files` and `list_tree` here still promised
the model that credential files were being withheld from listings, a
release after that behaviour was withdrawn: the change went into one runtime
and not the other, and a model was told something false for a whole
release. A shared mechanism that is fixed in one place is not fixed.

If you deliberately change only one runtime, say so in the commit
message and in the ADR, with the reason. A divergence nobody wrote down
reads as an oversight to the next person, and gets "fixed" wrongly.

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
                   file-tool paths (persistent-file writes and credential reads
                   OperatorOnly, ADR-0015), a Block floor only for shell text — the
                   lane decides the rest; the shared lists come from internal/sandbox
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
internal/hooks/    operator hooks on Claude Code's measured contracts (ADR-0012/0014):
                   PreToolUse / SessionStart / UserPromptSubmit / SessionEnd payloads on stdin,
                   deny by JSON or exit 2, context via stdout, fail-open otherwise
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

- **A tool with no path argument is invisible to the rule layer.**
  `risk.credentialRead` judges a `path` (or `file_info`'s `paths`), so
  `search_files` — which takes a pattern and walks — is `Safe` and never
  gates. ADR-0016 §5 nevertheless promised the matcher as the boundary
  when the file-read cage cannot be installed, and for the walk it was
  not: the degraded walk read `.env` and printed the matching lines. The
  walk now consults `sandbox.CredentialPath` itself in `readForSearch`.
  Before writing "layer X covers this when layer Y is absent", check
  that X can see the call at all.

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
  row, no runtime toggle. The other three events (ADR-0014) share the
  runner: session-start output and prompt-hook context ride the next
  turn as `hook` attachments through `AttachData` / `pendingAtts`,
  never the system prompt and never the typed input; `/clear` fires
  the old session's end hook before `ag.Restart` and the new one's
  start hook after the facts message.
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
- **`LAGENT_LLM_TRACE` is the one file where reasoning content lands.**
  The trace is the request body and the raw stream as sent — the
  instruction files, the nonce-wrapped tool results, the
  `reasoning_content` deltas ADR-0009 keeps out of the transcript — so
  `traceFiles` creates the directory `0700` and the files `0600` like
  the transcript (`TestTraceFilesArePrivate` pins it). Nothing deletes
  a trace; a bench or debugging note that sets it says so.
- **The bench is the evidence for Phase 2** (ADR-0006). A prompt,
  tool-description or config change that is meant to change the model's
  behaviour is measured as one more configuration on the same tasks
  before and after. Fixtures live in `testdata/` so their `.go` files
  are never built or linted; each run's project sits under the run's
  own `HOME`, so this repository's `AGENTS.md` never reaches a run. A
  task the model could answer without the files measures nothing.
- **The operator's environment is not filtered; the runtime's own
  namespace reaches no child** (ADR-0017). `sandbox.ChildEnv` removes
  `runtimeOwnEnv` and keeps `childExportEnv` (`LAGENT_SESSION_ID`,
  `LAGENT_WORK_DIR`, `LAGENT_PROJECT_DIR`, pinned to the export sites'
  constants), and it is applied at every spawn site: `laneEnv` for all
  three lanes, `internal/mcp`'s server spawn, `internal/hooks`. The two
  halves partition `LAGENT_`, and `internal/archtest`
  `TestRuntimeEnvNamespaceIsPartitioned` fails on a `LAGENT_` literal
  in neither half or a listed name the tree no longer uses — which is
  what closes R01: `LAGENT_API_KEY` is in the removed half because the
  runtime reads it for itself, not because anything recognises the word
  `key`. The withdrawn scrub guessed at the operator's names, covered
  one child of six, and passed `OPENAI_KEY` while catching
  `NPM_TOKEN`. Apply `ChildEnv` at any new spawn site; never re-add a
  rule over names the runtime does not own.
- **The kernel reads the file** (ADR-0016) — the covered reads
  (`read_file`, `view_image`, `file_info`, `search_files`) run in a
  child of this binary under `sandbox.FileReadProfile`, which denies
  `sandbox.CredentialFilters` at the kernel. `Registry.SetFileChild`
  injects it the way `SetLaneExec` injects the lanes, after
  `sandbox.VerifyFileReadLane` proves on this machine that the cage
  refuses `.env` and reads an ordinary file; unproven, the reads stay in
  process and the note says so. A refused open comes back as
  `tools.ErrCredentialRead`, which `Agent.credentialRetry` turns into
  the operator's question and, on a yes, re-runs under
  `tools.WithDirectRead` — the reason an approved credential read must
  NOT go to the child. So the Go matcher raises the prompt and the
  kernel is the boundary: a miss costs a spawn and a blunter prompt,
  never a leak. **Never add a matching rule to close a reported
  "hole"** — an entry in `internal/sandbox` is the mechanism working; a
  rule about how paths compare is the signal the boundary is in the
  wrong place. Hard links, copies and secrets in unlisted files are the
  written ceiling.
- **The walks judge no names** (ADR-0016 §3, withdrawing ADR-0015 §2) —
  `list_files` and `list_tree` list a credential-named entry like any
  other, and `search_files` names what the kernel would not let it read.
  `credentialTally` is gone; do not reintroduce name judging in a walk.
- **A credential path is the operator's question in every file tool**
  (ADR-0015). `read_file`, `file_info` and `view_image` on a path
  `sandbox.CredentialPath` names are Review with `OperatorOnly` —
  must-prompt: no session allowlist, `"never"` row or `--allow`
  answers it, `--auto` escalates it, `-p` denies it — judged on the
  real path `Agent.decide` resolves (`risk.PathJudged` names the tools
  it resolves for; a new path-judged tool goes on that list, never on a
  second one). `search_files`, `list_files` and `list_tree` never
  prompt: they skip a credential-named entry and report the count and
  names with the route. One list in `internal/sandbox`, read by the
  profile, the write tools' Block, the shell floor, the read tools'
  Review and the enumeration skip; a new credential location goes
  there and nowhere else. `@` attachments are the operator's and pass
  no gate.
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
- **The footer's `ctx`/`cache` gauge is a mirror.** It follows each
  round's `Usage` message and is reset on the `/clear` command word in
  `internal/tui/model.go` — the shared slash handler empties the
  conversation but cannot see the model (the same shape as the `/auto`
  marker). A new slash command that empties the conversation must reset
  it the same way; a third such case is the signal to make the handler
  declare its effect instead of matching command words.
