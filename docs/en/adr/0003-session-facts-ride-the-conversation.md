# ADR-0003: Per-session facts ride the conversation, not the system prompt

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-10 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | With the operator's 23 MCP servers enabled, every new session waited about two minutes before the first token, and the operator reported it |

## Context

A local server processes the prompt at about 580 tokens per second on
the reference machine, and it caches the processed prefix: a request
whose prefix is byte-identical to the previous one starts in about two
seconds. The operator's global `mcp.json` declares 23 servers with 243
tools whose schemas come to 60,097 tokens — a cold prefix of about two
minutes.

Measured with those schemas (2026-09-10):

| Request | Time to first token |
|---|---|
| cold | 136 s |
| identical prefix again | 2 s |
| one line of the system message changed, at its top | 119 s |
| one line of the system message changed, at its end | 118 s |
| a second `system` message added instead | 118 s |
| the same line moved into a user message | 2 s |

The chat template renders the tool schemas after the system text, and a
second system message is folded into the same turn, so any per-session
byte in the system message re-processes every schema. gem-agent's
prompt — the porting source — carries three such bytes: the
untrusted-data tag name (a nonce per session), the session work
directory (a path with the session id in it), and the session start
date. Every new session therefore paid the whole two minutes, and a
`/clear` paid it again.

## Decision

The system prompt is byte-identical across sessions. Everything
per-session rides the conversation as the runtime's opening message:

- `Agent.AnnounceSession(facts)` appends a user-role message that opens
  with `session.FactsPrefix`, names the isolation tag (`<name> … </name>;
  unique to this session`), and carries the facts the caller supplies —
  the work directory with its usage note, and the start date
  (`cmd.sessionFacts`).
- It is authored by lagent, so it rides unwrapped, and it is recorded
  in the transcript like any message, so a resumed session replays it.
  It is called after `agent.New`, after `SetHistory` (the tag is fresh
  and the restored history's own opening message named the old one),
  and after `/clear` — which now tells the model about the rotated work
  directory through a fresh facts message rather than a rebuilt system
  prompt.
- The system prompt says where those facts are ("the runtime-facts
  message at the start of the conversation") and nothing more about
  them. `{{DATA_TAG}}` is gone from it.
- The session listing neither previews the facts message nor counts it
  as a conversation (`session.FactsPrefix`, the ShellContextPrefix
  precedent): a transcript that recorded only it is skipped by
  `--continue` and gets no resume hint.
- `cmd.TestSystemPromptIsIdenticalAcrossSessions` pins the invariant;
  `rotateWorkDir` takes an `announce` callback where it took `setSystem`.

The consecutive user messages this produces (the facts, then the
operator's first message) were measured accepted by the LM Studio
template, with the model reading the work directory from the facts.

## Consequences

- A new session or a `/clear` starts in seconds while the server's
  cache holds the prefix — the same process, the same tool set, the
  same instruction files. The first session after a server restart
  still pays the cold cost once.
- The facts are the second thing in every request instead of the
  first; the model has read them in every measured turn, and the
  transcript shows them at the top of the conversation.
- The date is the session start, as before; a long session's "today"
  drifts exactly as it did.
- The prefix now depends only on the system prompt and the tool set, so
  the remaining lever is the tool set itself: `[mcp].exclude`, or the
  deferred MCP catalog the RFP leaves to Phase 2.

## Alternatives considered

- **A second system message** — measured folded into the system turn:
  118 s. Rejected.
- **A stable alias path for the work directory** (`…/work/current`) —
  removes the path from the volatile set but not the tag or the date,
  and two sessions in one project would fight over one link. Rejected.
- **A tag stable across sessions** — the nonce exists so a tool result
  cannot forge its own closing tag, and `guard.Wrap` refuses content
  that contains the name; a persisted name would widen the window an
  attacker has to learn it for no gain the facts message does not
  already give. Rejected.
- **Facts as a nonce-wrapped data attachment** — the attachment would
  have to announce the tag it is wrapped in, which `guard.Wrap` refuses.
  Rejected.

## References

- ADR-0002 — what the port does not carry
- RFP §7 — the measured prompt-processing constraint
- gem-agent ADR-0018 (session-scoped isolation tag) and ADR-0058
  (work directory) at the pinned commit — the designs this moves
