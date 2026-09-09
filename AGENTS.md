# AGENTS.md — lagent

Sandboxed CLI agent runtime on a local LLM served over an
OpenAI-compatible API (LM Studio, Ollama). A separate product line from
gem-agent built on the same design; gem-agent is the porting source
(ADR-0001), and the features it has that lagent does not reproduce are
listed in ADR-0002. Experimental, lab-series, unreleased. The scaffold
answers `--version` and `version`; the agent loop is RFP Phase 1.

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
| Release binary | `make build-all` (darwin/arm64 only; signed by `make package`) |
| Release archive | `make package` → `dist/lagent-vX.Y.Z-darwin-arm64.zip`, notarized |
| Release gate | `make verify-release` — refuses a zip with no notarisation marker, one rebuilt after its marker, one that does not unpack, or one whose binary does not run or reports another tag's version |

Version is injected via `-X main.version` from `git describe` — never edit the
`version` var default.

## Structure

```
config.example.toml  shipped config template ([llm] provider/base_url/model/api_key, [model] context_window;
                     pinned by a loader test)
lagent.example.project.toml  shipped <project>/.lagent.toml template
mcp.example.json     shipped MCP server template (pinned by a loader test)
main.go            entry point (package main, calls cmd.Execute(version))
cmd/               cobra root command, REPL loop, wiring, system prompt; `version`,
                   `sessions`, `trust`, `workdirs` subcommands; ask_user, MCP intake,
                   the MCP advertiser and mcp_load (cmd/mcpload.go, ADR-0004)
internal/config/   strict-decode TOML + env/flag precedence ([llm], [model], [sandbox],
                   [agent], [mcp], [tui], [approval]); the policy file
internal/llm/      Backend interface + the OpenAI-compatible client (stdlib net/http,
                   hand-written SSE, tool-call assembly, retry, per-provider context probe)
internal/agent/    tool-calling loop, approval dispatch, nonce wrapping, history,
                   the rule-tier auto-approve ladder, the round ladder
internal/tools/    built-in tools, path confinement, lane-aware exec injection, Register
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
internal/statedir/ shared per-project state convention: root+env override, escape, .project marker
internal/workdir/  per-session work directory: layout under the state root, sweep
                   report, empty-dir removal; LAGENT_WORK_DIR is exported at startup
internal/mention/  @-reference parsing (files, directories, images), project-confined
                   resolution, completion
internal/instructions/ AGENTS.md / AGENT.md / CLAUDE.md / GEMINI.md discovery
                   (ancestor walk, stops at $HOME)
internal/trustpin/ content pins for the agent-facing files and the persistent-file
                   snapshot; cmd/pins.go applies them to the grant
internal/sandbox/  SBPL profile generation per lane (read/write/operator), the shared
                   scratch / persistent-file / credential lists, sandbox-exec wrapping
internal/approve/  MITL gate (y/n/N/a + session allowlist; N = deny with a typed reason)
internal/session/  JSONL transcript: logger + resume loader; usage records in the
                   gem-usage-lens shape; LAGENT_SESSION_ID is exported at startup
internal/repl/     paste-safe input reader (plain REPL, non-TTY fallback)
internal/tui/      Bubble Tea inline TUI: model, approval gate, settings panel
scripts/           codesign-darwin.sh / notarize-darwin.sh (org templates, verbatim),
                   docs-mirror-check.sh, verify-release-selftest.sh (make check)
docs/en/, docs/ja/ INDEX + reference/ + adr/ + the RFP (en: no suffix; ja: .ja.md)
```

Every package under internal/ that came from gem-agent says so in its
package doc comment, with the source commit (ADR-0001). `internal/llm`
is new; the others are ports minus the ADR-0002 features.

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
- **MCP tools are registered always, advertised on load** (ADR-0004).
  `mcpAdvertiser` owns which servers the model can see; `agent.Options.
  Advertise` filters the declarations and refuses a call to a hidden
  registered tool before any gate. The catalog rides the facts message,
  so `AnnounceSession` runs after the MCP connect (startup, `/clear`,
  `/mcp reload`), never before. Anything that changes the connected set
  calls `adv.setInventory` then `ag.RefreshTools()`.
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
- **Skills are pinned, not loaded.** The trust probe and `internal/trustpin`
  still digest `.claude/skills` for change detection (the sibling runtime
  reads them, and a changed one is a fact worth a line), but nothing here
  loads a skill: skills are RFP Phase 2. The trust prompt says so.
- **There is no model tier.** `agent.AutoDecision.ModelConsulted` is
  always false and stays for record-shape parity; the round checkpoint
  asks the operator or stops. Adding a model review is a Phase 2 ADR,
  not a flag.
- **The OpenAI client reads are bounded by hand** — `io.LimitReader` on
  error bodies and the probe, a 16 MiB scanner buffer on the stream —
  and `internal/archtest` allowlists them by name with the reason.
  A new read there needs the same.
- **A new config key means updating `config.example.toml`** — strict
  decode makes a stale template a startup error, so the loader tests parse
  the shipped template against the built-in defaults.
- **Docs are checked mechanically** — `make docs-check` fails on a missing
  en/ja mirror, an ADR absent from either INDEX, an INDEX link that does
  not resolve, or a backticked identifier (tool name, flag, config key)
  present in one language only.
