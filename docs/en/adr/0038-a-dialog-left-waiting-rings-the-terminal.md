# ADR-0038: a dialog left waiting rings the terminal

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-04) |
| Date | 2026-11-04 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator: gem-agent rings the terminal when a dialog waits (gem-agent ADR-0104, v0.97.0); lagent should do the same |
| Relates to | gem-agent ADR-0104 (the same decision, ported here), ADR-0024 (what reaches the terminal), ADR-0026 (the sweep writer), ADR-0012 (pre-tool hooks) |

## Context

A lagent turn stops for the operator's answer at the same dialogs gem-agent has: the approval prompt, the approval of an MCP call under the read-only ceiling (`ApproveOnce`), the question whether to lift the ceiling (`ApproveLift`), and the question dialog, which carries `ask_user` and the round-limit intervention. Nothing tells anyone outside the screen that the turn is waiting.

gem-agent ADR-0104 decided, from a third party's usage summary and from measurements of Claude Code and of four terminals, to ring the terminal once when such a dialog has gone six seconds without a keystroke. That ADR did not bind lagent, and the same summary gave no grounds for it here: lagent's approval waits in it were short.

The operator decided on 2026-11-04 to port it anyway. An operator who runs both runtimes would otherwise have one that calls them back and one that stays silent at the same moment, and the cost here is one ticker per open dialog. There is no lagent-specific measurement behind this decision, and this ADR does not claim one.

## Decision

Port gem-agent ADR-0104 as it stands at gem-agent `386657f` (v0.97.0):

1. **The same dialogs, the same timing, the TUI only.** Every `ApprovalRequest` and `AskRequest` the TUI opens is checked every six seconds; the notice goes out once, at the first check with no keystroke in the six seconds before it, and an answer cancels it. A keystroke makes the next check pass; reading without one does not. The plain REPL (whole-line reads cannot see a keystroke before Enter) and `-p` (nothing waits) never notify. lagent has no `/riskbook`, so there is no confirmation of that kind to include.
2. **The same setting and bytes.** `[tui].notify = "bell"` (the default) writes one BEL; `"osc9"` writes `ESC ] 9 ; lagent needs your answer ESC \` followed by a separate BEL; `"off"` writes nothing and schedules nothing. Global configuration only, as `[tui]` already is; shown read-only in `/settings`. The OSC 9 body names this runtime and is fixed.
3. **The same write path.** The bytes are the runtime's own, so ADR-0024 neither covers nor needs to neutralise them. They are written through the sweep writer as a write of their own between two renderer flushes, from a `tea.Cmd`. ADR-0026 left `Inject` out of the ported writer because only gem-agent's probes used it; it comes over now, because the notice is its first use in a session.
4. **No new record, no hook.** As in gem-agent: nothing is written to the transcript, and no notification hook is added (ADR-0012's rule for hook events applies here too).

The measurements behind the timing and the terminal behaviour are gem-agent ADR-0104's ("Measured"); they are not repeated here.

## Consequences

- **Behaviour changes for operators who configure nothing:** a TUI dialog left six seconds without a keystroke rings the terminal bell. The release notes say so and name `[tui].notify = "off"`.
- The two runtimes behave the same at a waiting dialog, and a `[tui].notify` line copied between their configurations means the same thing.
- `internal/tui/notify.go` records its gem-agent source commit, per ADR-0001; later gem-agent changes are not tracked.

## Alternatives considered

- **Leave lagent silent until a lagent measurement shows long waits** — not adopted: the operator chose parity between the runtimes over waiting for that evidence.
- **Port only the BEL, without `"osc9"`** — not adopted: the setting would then differ between the runtimes for no saving; the OSC 9 path is a constant string.

## References

- gem-agent ADR-0104 (the decision, the measurements of Claude Code 2.1.311 and of tmux, Terminal.app, iTerm2 and kitty)
- ADR-0001 (porting sources pinned), ADR-0024 (outside text made inert), ADR-0026 (the sweep writer), ADR-0012 (pre-tool hooks)
