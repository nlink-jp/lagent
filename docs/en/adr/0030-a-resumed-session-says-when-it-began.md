# ADR-0030: the session's date is captured once, and a resumed session says when its conversation began

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-10-06) — ported from gem-agent ADR-0097; implemented |
| Date | 2026-10-06 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | gem-agent ADR-0097: an operator resumed a session days after it began, and in 2 of 4 runs the model used the conversation's older date for new work. This runtime states the date the same way, in a different place |
| Relates to | [ADR-0003](0003-session-facts-ride-the-conversation.md) (the facts message), gem-agent ADR-0097 (the decision and its evidence) |

## Context

The date reaches the model in the session-facts message (ADR-0003), as
`- session started: <day>`. Read against the code it has the two faults
gem-agent found in its system-prompt line:

- **On resume it contradicts the record.** The facts message is sent
  again after the restored history, saying "session started" on the
  resume day — beside the restored conversation's own facts message,
  which says "session started" on the day it really began.
- **It is not captured once.** An MCP reload re-sends the facts message,
  and each send reads the clock again; a reload after midnight states a
  new "session started" day.

This runtime already states the date at the resume point, near the work —
the position gem-agent measures separately. That is unchanged here.

## Decision

1. **The date is captured once per session**, when it begins (process
   start, or `/clear` starting a new one); every facts message of the
   session states it.
2. **A resumed session states both facts**: `- resumed: <day>; the
   conversation above began on <day>`. The resume day is not called
   "today", which stops being true at midnight. A fresh session keeps
   `- session started: <day>`.

## Consequences

- On resume the new facts message agrees with the restored one.
- An MCP reload no longer moves the date.

## References

- `cmd/prompt.go` — `sessionFacts`, `dateFact`
- `cmd/root.go` — resume, the facts message, `/clear`
