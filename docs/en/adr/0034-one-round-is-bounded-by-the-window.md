# ADR-0034: one round is bounded by the window

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-01) — revised once against an independent design review; implemented and accepted on the Measurement below |
| Date | 2026-11-01 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator report (2026-11-01, v0.20.x): a session read several files larger than the window allows, compaction did not help, and every later turn failed until `/clear` |
| Amends | ADR-0033 (the retained tail is bounded by size, not only by count; its Consequences relied on ADR-0029 to keep one round small, which it does not), ADR-0029 (a tool's own cap is an upper bound; the runtime may allot less, and every cut names its route or says the rest was not kept) |
| Relates to | ADR-0033 §4–§6 (the projection, the estimate, the declined compaction), ADR-0029 (what was not shown is reachable), ADR-0004 (`mcp_load` grows the tool definitions), ADR-0003 (the system prompt and tool descriptions are byte-identical across sessions) |

## Context

The failed session is on this machine, so its record could be read
(structure only — sizes, tool names, the runtime's own notices):

- mlx-serve, Qwen 3.6 35B A3B, a window of 167,936 tokens.
- The model was asked to analyse a directory of data files. In its third
  round it called `read_file` four times in parallel, each on a whole
  file, no window. The results were 68,053 + 204,876 + 204,876 + 28,716
  bytes: 506 KB in one round. Two of them had already been cut at
  `read_file`'s cap (200 KiB).
- The request before that round measured 22,110 tokens. ADR-0033's
  projection put the next one at 113 % of the window, and compaction
  declined (§6): the retained tail alone was past the threshold. The
  request went out and failed, as did every turn after it.
- `/compact` by hand then summarised 8 of the 14 messages and kept the 6
  most recent verbatim — the assistant turn with the four calls and the
  four results. The history was no smaller where it mattered, and the
  next turn failed too.

ADR-0033 said this case was "still not rescued" and out of reach because
"ADR-0029 bounds what a single tool returns". That premise does not hold
on a local window:

1. **Every cap that feeds the prompt is absolute.** `read_file` returns
   up to 200 KiB — about 68k tokens by ADR-0033's estimate — a value
   carried over from a runtime with a 1M-token window; here one result
   is 40 % of the window. @-attachments share the same origin: 256 KiB
   per message.
2. **Nothing bounds a round.** Each call is capped on its own; four
   parallel calls are four caps.
3. **The retained tail is counted, not sized.** Compaction keeps the six
   most recent messages whatever they weigh, so the round that caused
   the overflow is exactly the part it never touches.

## Decision

The **threshold** (`C = compact_at_pct × window`) is the line a request
is kept under; the part of the window above it is the reserve for the
model's output and for estimate error. Everything below applies only
when the window is known; with no window, each tool's own cap applies
as today. No configuration key is added.

1. **A round's tool results share a budget, re-judged before every
   call.** Let `F` be the estimate of the system prompt and the tool
   definitions *as they stand* (an `mcp_load` earlier in the round
   included), `S` the summary allowance (ADR-0033 §6), and

   `T = (C − F − S) / 2`

   — half of what a compaction can make room for. Before each call the
   runtime computes the room left for this round's results:
   - **A compaction can run before the next request** (`auto_compact`
     on and not switched off by failures): `room = T − U`, where `U` is
     what this round's results have taken so far. The next request may
     pass the threshold; the compaction before it keeps this round
     verbatim (§4) and gets under it.
   - **No compaction can run:** `room = min(T − U, C − P)`, where `P` is
     the live projection of the next request (ADR-0033 §4, this round's
     results so far included). The request stays under the threshold on
     its own.

   Call *i* of the *n* the model made is allotted `room / (n − i + 1)`:
   what a small result does not use passes to the calls after it. The
   allotment converts tokens to bytes at **2 bytes per token** — below
   the 10th percentile ADR-0033 measured (2.25) — so it bounds the
   tokens a result costs rather than estimating them. Below zero, the
   allotment is zero.

2. **The allotment rides the call.** It reaches the tool as an upper
   bound on the bytes returned inline, below the tool's own cap: on the
   call's context in-process, and as an explicit field of the sandboxed
   file child's request, which a context value never reaches. Every
   allotment has a floor of 1 KiB, outside the budget, so a short
   result — an edit's confirmation, an error — arrives whole and a byte
   window always advances. A result longer than its allotment plus
   2 KiB for the tool's own marker and route (a tool that did not apply
   it) is cut by the runtime with a marker that says the rest was not
   kept; a test runs the built-in tools under a small allotment and
   requires every built-in tool to be classified, so none relies on
   that backstop.

3. **Every cut names its route, or says the rest was not kept.**
   ADR-0029 gave some tools a route; a smaller allotment makes cuts
   common, so each one is spelled out:
   - `read_file` whole file: `offset=N reads on` (exists). Line window:
     `start_line=K reads on` (new — today it gives none). Byte window:
     `offset=N reads on` (new for this path).
   - `shell_exec` in a sandboxed lane: head and tail, the full output
     spooled to the file the marker names (exists). Operator lane and
     unsandboxed runs save nothing by design (credentials are readable
     there); their marker already says the middle was not saved, and
     the route is to run the command narrower.
   - MCP results: the intake's spill and path (exists); the preview
     shrinks to fit the allotment, head and tail in their existing
     proportion.
   - `search_files`, `list_tree`, `list_files`, `file_info`: their
     existing counts and cut; the route is a narrower path or pattern,
     and the marker says so.
   - Counted against the budget but not cut: an image `view_image`
     attaches (ADR-0033's flat 2,000 tokens), and a tool marked
     `KeepWhole` — one whose result is the operator's or the runtime's
     own words, bounded at its source: what `load_skill` returns (an
     instruction file the operator installed, a body 64 KiB, a supporting
     file 96 KiB, where half the instructions is worse than none),
     `ask_user`'s answer, `mcp_load`'s tool list, the memory tools'
     confirmations. Every tool `cmd/` registers is one (pre-release
     review: the backstop cut an operator's long answer).

4. **Compaction's retained tail is bounded by size.** The cut keeps the
   most recent messages up to `keepRecent` (6), but moves forward to a
   later safe boundary while their estimate exceeds `T`, as far as the
   last message that is not a tool result: inside a tool loop that is
   the last round's assistant turn, at the start of a turn it is the
   operator's message. When every message the count would keep is a
   tool result — one round of more parallel calls than `keepRecent` —
   the cut steps back to that round's own assistant turn (pre-release
   review: six parallel reads left no cut at all). ADR-0033 §6's check — decline once, with the
   reason — is made on the cut the summariser actually reads up to
   (after `fitHead`), and stays for what no bound here reaches. The
   compaction record is unchanged: it carries the count replaced,
   wherever the cut fell.

5. **@-attachments share the bound.** One message's attachments
   together get at most `T` (at 2 bytes per token) below their own
   limits; what is cut is reported as the existing limit problems are.

6. **A window with no room ends the turn.** When no compaction can run
   and the room is zero, every result would be its route alone and the
   model would call again into the same wall until the loop guard or
   the round limit stopped it. Instead the turn ends after that round
   with a notice that the conversation fills the window, naming
   `/compact` and `/clear`. The history stays valid and under the
   window.

7. **The window is awaited for the budget too.** The first round waits
   for the window lookup (ADR-0033 §4, at most once per session) whether
   or not automatic compaction is on.

8. **What the model and the operator are told stays true.** `read_file`'s
   description gains one fixed sentence — a read may come back shorter
   than its cap when the conversation is filling the window, with the
   route to the rest — so it stays byte-identical across sessions
   (ADR-0003). The README's "a read cut at 200 KB" changes with it.

## Consequences

- At 2 bytes per token `T` ≈ 61k tokens is 122 KB on the failed
  session's window (the system prompt and tools taking about 6 %). The
  failed round would have come back as about 120 KB: the first three
  reads cut near 30.6 KB each with their routes, the last (28.7 KB)
  whole. The next request would have been about 37 % of the window.
- **Sessions already stuck can be rescued.** A session recorded before
  this change — or one whose compaction failed — that holds an oversized
  round recovers once the operator has written again: the retained tail
  is just that message, and the oversized round is summarised with its
  tool output clipped (ADR-0033 §7). On a resumed session that happens
  automatically before the first request; in the session that just
  failed, `/compact` does it (a compaction that just ran waits for four
  more messages before firing again, ADR-0033 §1). No record is
  rewritten.
- On a small window a round of large reads costs more rounds: the model
  reads on with the routes it is given — a premise, measured before this
  ADR is accepted. On a large window `T` exceeds the tools' own caps and
  nothing changes.
- Not covered, and accepted:
  - **A compaction that fails** before a request that needed it: that
    one request goes out as today and may fail; the session is rescued
    as above, and two failures switch to the no-compaction rule of §1.
  - **A round whose own call arguments are huge** (a `write_file` of a
    large file): arguments are the model's output, not a tool result,
    and are not cut. They fall in the margin `T` leaves under the
    threshold, and past it ADR-0033 §6 declines with the reason.
  - **What is counted but not cut** (skills, images) can take a round
    past `T` on a small window; §4 and ADR-0033 §6 then decide.
  - **A window the system prompt and tool definitions nearly fill**:
    `T` goes to zero; ADR-0033 §6's notice already names that share.

## Alternatives considered

- **A smaller fixed cap for `read_file`** (e.g. `OutputCap`'s 20 KB).
  Rejected: still absolute — too small for a large window, and parallel
  calls still add up on a small one.
- **A per-result cap relative to the window.** Rejected: it does not
  bound a round; *n* parallel calls are *n* times the cap.
- **One budget fixed at the start of the round, split `R / n`.**
  Rejected: an `mcp_load` in the round grows the tool definitions after
  it is fixed, and a small result's unused share is wasted while a large
  one is cut.
- **Up to the window rather than the threshold** when no compaction can
  run. Rejected (review): it leaves no room for the model's output — no
  `max_tokens` is sent — and none for a result that tokenises worse than
  the estimate.
- **Clipping old tool results in the history at compaction time.**
  Rejected: it rewrites what the model already saw, needs a new record
  kind for resume (a stability-contract change), and the route in a
  clipped old result may no longer be accurate. Summarising them once
  the operator writes again (Consequences) reaches the same sessions
  without that.
- **Spilling an oversized `read_file` result to the work directory.**
  Rejected: the file itself is the store, and a copy would duplicate
  what may be an approved credential read in a second place.
- **A system-prompt rule to read large files in windows.** Rejected: the
  tool description already says to prefer windows, and the model in the
  failed session did not. The runtime's facts are made true first
  (CLAUDE.md).
- **Recognising the server's overflow error and retrying smaller.**
  Rejected for ADR-0033's reason: a text rule over an open set of
  servers.

## Measurement

mlx-serve (Qwen 3.6 35B A3B 4-bit), the same window (167,936 tokens),
the same project and files, 2026-11-01; one-shot, read-only, `--mcp off`,
an isolated state directory.

| Run | What happened | Largest request |
|---|---|---|
| The reported request, replayed | The model read in line windows this time; one whole read (68 KB) fit its allotment and came back whole; no cut, no overflow | 87,116 tokens (51 %) |
| The reported round, forced (four whole-file reads in one round, then quote each file's last line) | 68 KB + 2.0 MB + 267 KB + 29 KB asked; 31.4 + 31.3 + 31.3 + 28.7 KB returned, the first three with `offset=N reads on`. The model then read each tail (`offset=-500`) and quoted the last lines: three verbatim, the fourth reached whole but copied with its empty fields dropped | 75,136 tokens (44 %); the round itself took the request from 6,609 to 72,083 |
| The stuck session, resumed with a new message (the record from before this change) | Projected at 109 %; compaction summarised the 6 messages holding the oversized round and kept 4; the turn answered | 7,026 tokens (4 %); 44 s in all |

- The model reaches what was cut through the routes it is given: the
  premise holds.
- The forced round's CSV cost about 1.87 bytes per token (122.7 KB for
  65.5k tokens) — under the 2 used for the allotment, so the round came
  in about 7 % over its estimate. `T` is half of the room a compaction
  makes, and the request stayed at 42 %; the margin is what absorbs this,
  and it is recorded here rather than tightened.
- Before this change the same round reached 113 % and every later turn
  failed.
- Two findings outside this ADR: `shell_exec` in the read lane could not
  run under this project's cloud-storage path (`getcwd: Operation not
  permitted`), so the model fell back to `read_file`; and resuming
  requires the working directory's spelling to match the record's
  byte for byte, which a path entered in a different Unicode
  normalisation does not.

## References

- ADR-0003, ADR-0029, ADR-0033
- `internal/agent/agent.go` (the round loop), `internal/agent/compact.go`,
  `internal/tools/tools.go`, `internal/tools/shellout.go`,
  `internal/tools/nav.go`, `cmd/mcpresult.go`, `cmd/filechild.go`,
  `internal/mention/mention.go`
