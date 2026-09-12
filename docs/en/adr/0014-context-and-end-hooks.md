# ADR-0014: The hook set is gem-agent's — session start, prompt submit and session end join pre-tool

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-12 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator: the hooks should be the full set, not the one event |
| Amends | ADR-0012 §1 (one event → gem-agent's four, on the same mechanism) |

## Context

ADR-0012 §1 implemented `PreToolUse` and declined the other events
until a consumer appeared, on the ground that a capability without a
trigger is dead weight. The operator's direction is the full set. The
argument for it is drop-in parity, which ADR-0012 §4 already values
for the one event: an operator who keeps one hooks block for Claude
Code and gem-agent keeps the same block for this runtime, with nothing
to port when a consumer arrives — and the consumer that gem-agent
added two events for (a shared knowledge space across runtimes,
gem-agent ADR-0069) is one this runtime would join with those events,
not without them. "Full set" here means gem-agent's four: Claude
Code's other events (PostToolUse, Stop) are in neither runtime, and
each would need a delivery rule of its own.

gem-agent measured the three contracts against Claude Code itself
rather than reading them from documentation (gem-agent ADR-0069 §2–3,
ADR-0071 §4): the `SessionStart` payload carries `source`
(`startup` / `resume` / `clear`), `UserPromptSubmit` carries the typed
text under `prompt`, `SessionEnd` carries `reason`; plain stdout on
exit 0 is injected context, a JSON object is a verdict of which only
`hookSpecificOutput.additionalContext` is context; a prompt can be
refused by exit 2 or either block form and the turn never starts; a
session start or end cannot be refused.

Two of lagent's own records bear on delivery. ADR-0003 keeps the
system prompt byte-identical, so injected context cannot go there.
ADR-0013 measured what this model acts on: a line in the runtime-facts
message or beside the typed input, framed as something to act on. A
hook's output is code run over whatever that code read — in the
triggering design, a store other sessions write to — so it is data,
and it says so.

## Decision

1. **Three more events, from lagent's own config, global only.**
   `[[hooks.session_start]]` (optional `matcher` on the source:
   `startup`, `resume`, `clear`, `a|b`, `*`), `[[hooks.user_prompt_submit]]`
   and `[[hooks.session_end]]` (no matcher; every prompt or end runs
   them, and a matcher there is a config error). ADR-0012 §1's project
   ban stands: a context hook runs on every turn.
2. **The payloads are Claude Code's, with the fields lagent has.**
   `hook_event_name`, `session_id`, `transcript_path` (empty when the
   log is disabled), `cwd`, plus `source`, `prompt` or `reason`.
   `session_start` fires at startup with `startup`, or `resume` under
   `--continue`/`--resume`, and on `/clear` with `clear`.
   `user_prompt_submit` fires for every turn that reaches the model —
   a typed message, the argv first message, a `/skill`-expanded turn,
   the `-p` prompt — and not for slash commands or the `!` escape.
   `session_end` fires on exit with `exit` and on `/clear` with
   `clear` for the old session, before the new one starts.
3. **Output is context or a verdict; a prompt can be refused.** Exit 0
   with plain stdout is context; a JSON object is a verdict whose
   `additionalContext` is context. A prompt hook refuses by exit 2
   with stderr as the reason, or by either block form ADR-0012 §3
   accepts; the prompt is erased — nothing enters the history or the
   transcript — and the operator sees the reason (`-p` exits non-zero
   with it). The first block wins and discards the other hooks'
   context. A session-start hook that blocks is a reported failure
   that injects nothing; session end ignores output. Everything else
   fails open with a notice, as for pre-tool.
4. **Injected context rides the data lane.** A hook's output is an
   attachment of kind `hook` on the next user message — stored beside
   the typed text in the transcript, flattened after it on the wire
   inside the turn's nonce tag and announced as quoted data, the lane
   piped stdin uses. The system prompt is untouched; the typed input is
   untouched; the model is told what it is. Capped at 8000 runes per
   hook with a visible cut, and one notice per injection so the channel
   is not quiet.
5. **`/clear` fires end then start.** The old session's `session_end`
   (`clear`) runs before the new transcript takes over; the new
   session's `session_start` (`clear`) output rides the first new
   turn.

## Consequences

- One hooks block serves three runtimes, and the knowledge-space
  design that needed these events on gem-agent can register the same
  script here.
- Every turn pays one process spawn per configured
  `user_prompt_submit` hook; a start or end pays one per hook.
  Unconfigured, nothing runs.
- `agent.Options` gains `PromptHook`; `Run` may return
  `ErrPromptBlocked` before recording anything. `internal/hooks` is
  now the full port of gem-agent's package; ADR-0012's "cut to one
  event" note is superseded.
- Whether this model acts on hook-injected data is unmeasured; the
  same bench that measured memory (ADR-0013) measures it when a
  consumer's output exists to measure.

## Alternatives considered

- **Keep waiting for a consumer** (ADR-0012 §1). Set aside on the
  operator's direction; the parity argument holds without one.
- **Inject session-start output into the facts message** (the channel
  ADR-0013 chose for memory). Rejected: the facts message is the
  runtime's own words; hook output is a script's, run over unreviewed
  input, and belongs in the data lane with its provenance stated.
- **PostToolUse and Stop.** Not adopted: in neither runtime, no
  contract measured, each with its own delivery question.
