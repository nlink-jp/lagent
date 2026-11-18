# ADR-0054: the session is assembled from parts — `runREPL` is split without changing what it does

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-18) — being implemented |
| Date | 2026-11-18 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The operator, after gem-agent's `runREPL` was restructured (gem-agent ADR-0124, released in v0.108.1): "do the same for lagent" |
| Relates to | gem-agent ADR-0124 (the parts this design takes, and what implementing them taught), ADR-0001 and ADR-0002 (gem-agent is a source; what is not carried), ADR-0003 (facts ride a message, not the system prompt), ADR-0012 and ADR-0014 (hooks), ADR-0023 (stability contract), ADR-0032 (the model tier), CONVENTIONS ADR-029 (design before implementation) |

## Context

### What was measured (v0.32.0, 2026-11-18)

| Measure | Value |
|---|---|
| `runREPL` in `cmd/root.go` | **1,587 lines, 226 branches** |
| Next largest | `tui.Model.Update` 339 lines / 64; `Agent.execCallInner` 320 / 48; `Agent.Run` 303 / 50 |
| Commits touching each file, last 300 | `cmd/root.go` **70**; `internal/agent/agent.go` 38; `internal/tools/tools.go` 33 |
| Locals rewritten after startup by closures | 15 (`grant.excluded`, `persistentSnap`, `curLog`, `sessionLog`, `sessionPath`, `sessionID`, `hookSession`, `workDir`, `dates`, `projectContext`, `skillsList`, `mcpClients`, `mcpSummary`, `mcpInv`, `mcpFilter`), plus the field `settings.inv`; two more bound late (`prog`, `ag`) |
| `if prog != nil` output decisions | 9: the stream observer, `askOperator`, `notice`, `OnToolCall`, `OnToolDone`, `OnAttach`, `OnUsage`, `OnAutoDecision`, `resolveWindow` |
| `defer`s | 10 in `runREPL` itself (one more inside the `resolveWindow` closure) |
| Transcript nil checks | 8 (6 on `sessionLog`, 2 on `curLog`) |
| Tests that read `root.go`'s source or copy `runREPL`'s flow | 12 files: in `cmd/`, `regrant_test`, `wiring_test`, `review3_test`, `grant_test`, `ladder_wiring_test`, `pins_recorded_test`, `review4_test`, `usage_test`, `risktier_test`, `sessions_test` (copies the resume flow); in `internal/archtest/`, `inert_test`, `unwrap_test` |
| ADR line citations of `root.go` | 4: `root.go:1602` in ADR-0020 (en, ja), `root.go:918` in ADR-0021 (en, ja) |

The shape is gem-agent's before its ADR-0124, for the reasons recorded there: lagent's `cmd` was ported from it (ADR-0001). The startup order is held by comments; what `/clear` and the reloads change is held in closure-captured locals; where output goes is decided in nine places; teardown order is the reverse of ten `defer`s across 1,500 lines; the tests copy or parse the function because they cannot call it; three front-end branches each start a turn with `announceGrant()` then `ag.Run`, and `-p` calls `ag.Run` directly.

### Found while reading (not part of this decision)

1. **`/clear` freezes the TUI when a `session_start` hook prints anything** (a defect; measured). The slash handler runs synchronously inside Bubble Tea's `Update` (`tui/model.go`, `submit`). `onClear` runs `sessionStartHooks("clear")`, which reports the attached context by calling `notice` itself, and `notice` calls `prog.Send` — which, from inside `Update`, never returns: `Update` is the only reader of the channel it waits on. The hook runner's own notices (a hook that failed or timed out) raised by `sessionEndHooks("clear")` or `sessionStartHooks("clear")` take the same path. Measured on the v0.32.0 binary in a private tmux server with `--no-sandbox` and one `session_start` hook: with `command = "true"`, `/clear` and a `/version` after it both answer; with `command = "echo context-from-hook"`, `/clear` never answers and nothing typed afterwards is processed. gem-agent met the same defect and collects the hook notices raised while a slash command runs into that command's output; lagent's hooks (ADR-0014) were ported without it.
2. **`/clear` panics when the startup transcript could not be opened** (a defect; read in the code, the same as gem-agent's, fixed there in v0.108.1). When the state directory resolves but `openSessionLog` fails without a resume, `curLog` stays nil; a later `/clear` that opens its own transcript reports the persistent files through `curLog.Log` — a method on a nil `*session.Logger`, which locks its mutex first — so with any change to report, the session panics.
3. **The exit summary is not last when a `session_end` hook prints.** lagent runs the summary before the `session_end` hooks, so in the plain REPL a hook's notice (a failure, a timeout) prints below the resume command; in the TUI the notice is sent after the program has ended and is lost. Proposed as lagent's own decision, for the reason gem-agent's maintainer gave there (gem-agent ADR-0065 addendum): the resume command and the totals are the last thing in the scrollback.
4. **To be measured: the persistent-file reports of `/clear` write to stderr while the TUI draws.** `reportPersistent` and `watch.report` print straight to stderr; during `/clear` under the TUI that happens inside `Update`, and the code's own comment on `reconnectMCP` says a stderr write under the TUI corrupts the display. It does not freeze. gem-agent does the same. Measured with step 0's TUI driver; if the display is damaged, it is a defect in both runtimes, and its fix is put to the maintainer then — it is not folded into this work.

## Decision

`runREPL` is restructured into the parts below — the ones gem-agent ADR-0124 introduced, with lagent's own contents. **Nothing the operator, the model, a child process or a log reader can observe changes**: flags, output text and order, exit codes, transcript records, environment variables, files, the startup and exit sequences, the system prompt, the facts messages and the order of the tool declarations. ADR-0023's contract (flags, config keys, `LAGENT_*`, record formats, the hooks contract) is untouched by construction. The four findings above are handled in their own `fix:` commits (§3), never inside a `refactor:`. No code, text or documentation of a feature ADR-0002 lists comes over from gem-agent.

### 1. Conceptual design — the parts

| Part | Promise |
|---|---|
| **`runFlags`** | the root command's `flag*` values read once; `afterDirectShell`, a package variable today, becomes an argument of `runDirectShell` |
| **Startup phases** | each takes what it needs from earlier phases as arguments and returns a typed result; they run in today's order, so every warning prints where it does today |
| **State holders** | created up front by `newLiveSession`, filled by the phase that produces their content, shared by pointer: grant, persistent snapshot, transcript (with a nil-safe `log`), work directory, project content (instructions, skills, memory store), MCP set |
| **`liveSession`** | owns the holders after startup; the post-startup mutators are its methods; they run between turns only |
| **`surface`** | the one decision of where operator-facing output goes, one concrete type whose late-bound program decides |
| **Front ends** | `runOneShot`, `runTUI`, `runPlain`; every turn starts in `liveSession.turn` |
| **Cleanup stack** | `push` (last pushed runs first) and `pushFinal` (after every other step, in the order pushed) |

### 2. Detailed design

#### 2.1 Startup phases

| Phase | What moves into it, in today's order | Notes |
|---|---|---|
| `resolveRun` | config load, the read-only flags, the first message, the effective auto state and ceiling, the UI language, the interrupt ladder | |
| `admitProject` | project directory, broad-root refusal, policy file, project trust and pins, project config, MCP filter, approval policy, the obsolete-entries note, the persistent snapshot, the direct-shell note | |
| `openTranscript` | resume resolution, the transcript, the restored history | returns `opened` |
| `prepareWorkDir` | the work directory and its environment | takes `opened` |
| `buildConfinement` | sandbox availability, added directories, the lanes, the registry, the granter, **`request_directory`** (registered here today, before `load_skill`; it stays, because the registry's order is the order of the declarations the model receives), the file-read child | takes the work area |
| `loadProject` | instruction files, skills, memory, the MCP servers, the advertiser and `mcpStatus` (reading through the MCP holder) | |
| `buildModels` | the main client and, when the model tier is on, its client (ADR-0032), with the tier's notes | the notes are printed later through `notice`, as today |
| `chooseFrontEnd` | TUI or not, the shared stdin reader, images, the gate, the line reader, `surface`, the stream observer | |
| `registerTools` | `ask_user`, `mcp_load`, the round-limit dialog | returns `toolsRegistered`, which `assembleSession` takes |
| `assembleSession` | `composeSystem`, the write guard, `notice`, the tier notes, the hooks, the agent, the restored history, the dates, `announceFacts` and `announceGrant`, the start hooks, the exit summary, `showImage`, the settings store, the reloads, `clear` | split further where it would exceed 150 lines |

#### 2.2 `liveSession` and the transcript holder

Methods, each the body of today's closure moved with no edit beyond names: `composeSystem`, `announceFacts`, `turn`, `clear`, `reconnectMCP`, `reloadMCP` (reconnect, then `announceFacts` — what `/mcp reload` does; `clear` reconnects without announcing and announces last, as today), `reconnectServer`, `loadMCP`, `sessionStartHooks`, `sessionEndHooks`, `reportPersistent`, `resolveWindow`, and `slash`, which runs `memorySlash` before `slashOutput` as both call sites do today; `slashOutput` keeps its parameters. The transcript holder's `log` records nothing when there is no transcript; once Found 2's fix is in, every session record goes through it.

#### 2.3 `surface`

Its methods are the nine decisions of the Context: `stream`, `ask`, `notice`, `toolCall`, `toolDone`, `attached`, `usage`, `autoApproved`, `contextWindow` — each printing exactly what its branch prints today — plus, once Found 1's fix is in, `hookNotice` and `collectHookNotices`.

#### 2.4 The model seam

`newModelClient(cfg, model)` builds a client for a model name through `modelClient`, an interface over `*llm.OpenAI` (embedded) with what `runREPL` and its callees use: `ChatStream`, `ContextWindow`, `SetObserver`, `SetReasoningEffort`. It is called where `llm.NewOpenAI` is called today, for the main model and for the tier's. When the tier is off, the tier's backend stays a nil interface — never a typed nil inside one — because `ModelTier: riskBackend != nil` and the agent's fallback to the main client test for it. `internal/llm` does not change.

#### 2.5 The exit sequence

Today, in run order: the added roots' persistent files (`watch.report`), the project's (`reportPersistent`), the exit summary (not under `-p`), the `session_end` hooks, close the MCP clients, close the skills' roots, remove the work directory if empty, remove the fallback scratch, close the transcript, clear `afterDirectShell`. Step 1 reproduces it without the last step, which disappears with the variable. Found 3's fix then makes the summary a final step.

#### 2.6 The judges

- **Characterization** (`cmd/characterize_test.go`): the test binary re-run as a child, the model scripted, the references recorded on v0.32.0 — `-p` with a tool call; the plain REPL fed one line per prompt through every read-only slash command; `/clear` between two `!env` with `session_start` and `session_end` hooks; the exit sequence on a full run and on a start failing in each of several phases, each for the reason it names; machine-dependent lines (the file-read cage's note) left out of the comparison.
- **The TUI driver**: the same child as a TUI under a private tmux server; skipped where tmux is absent.
- **Source-reading tests**: the twelve files of the Context, each retargeted in the step that moves its code to read every file of `cmd` (or the session-wiring list, itself checked for completeness); `archtest/inert_test` keeps passing on `runREPL`.
- **ADR citations**: ADR-0020's and ADR-0021's `root.go` lines, retargeted when their code moves.

### 3. Development plan

| Step | Commit | Done when |
|---|---|---|
| — | `fix:` Found 2, with tests | the guard's tests pass; nothing else changes |
| 0 | `test:` the model seam, the characterization and the TUI driver | the references recorded on this commit's parent pass, stable over repeated runs |
| — | `fix:` Found 1: the hook notices raised while `/clear` runs — the runner's and the attach notice of `sessionStartHooks` — are collected into `/clear`'s output on every front end | a TUI test with a printing `session_start` hook fails before and passes after; in the plain REPL those notices move from before `/clear`'s output into it, so the `/clear` reference changes by exactly those lines, checked before it is replaced |
| — | measure Found 4 with the TUI driver | the result is recorded here and, if the display is damaged, put to the maintainer |
| 1–5 | `refactor:` flags and cleanups; `surface`; holders and `liveSession`; the phases; the front ends and `turn` | `make check` green, references byte-equal, moved code's source-reading tests and ADR citations retargeted in the same commit |
| — | `fix:` Found 3, the exit summary as the final step | a plain-REPL run with a failing `session_end` hook prints the summary last; the test fails before |
| 6 | `docs:` AGENTS.md, the architecture reference (en, ja), this record's addendum with the measures taken again | `make check` green |

The TUI is also compared in a real terminal (tmux, scripted model) between v0.32.0 and the last step.

### 4. Verification, bounded before it starts

- **This design**: one independent pass against the recorded rules and against what `runREPL` does today. Done 2026-11-18: twelve findings, all folded in above (the panic of Found 2, the bodies of §2 written for lagent instead of "as gem-agent", the bounded release passes, Found 3 stated as lagent's own decision, the counts of the Context, `request_directory`'s phase, Found 4, the effect of Found 1's fix on the plain REPL, the model seam's methods and the nil tier, and `memorySlash`, `reloadMCP`, `mcpStatus` and the tier notes placed).
- **The implementation**: conformance to §1–§2, judged by the characterization. Before release, one `/code-review` of the whole diff and one more over the fix diff only. A third pass needs the maintainer's go-ahead; a high finding in the fix-diff pass sends the work back to this design.

### 5. gem-agent

Nothing in gem-agent changes here, except what Found 4's measurement may lead the maintainer to decide. The two runtimes' `cmd` packages will have the same parts under the same names; `cmd` wiring stays outside AGENTS.md's list of shared mechanisms.

## Alternatives considered

| Alternative | Why not |
|---|---|
| Leave lagent as it is | The most-changed file is the least readable, the tests copy it, and it held two defects gem-agent had fixed (Found 1, Found 2). |
| Copy gem-agent's new files over | The bodies differ throughout (no telemetry, facts messages instead of prompt rebuilds, the tier on its own client, other hooks wiring), and a copy would carry gem-agent code and text, which ADR-0002 forbids for the features it lists. The parts are taken; the bodies stay lagent's. |
| Fix Found 1 and Found 2 and stop | They are instances of "output routed in nine places" and "state in closures"; fixing them without the restructure leaves the classes. They are fixed first anyway, on their own, so a release can carry them whatever happens to the rest. |
