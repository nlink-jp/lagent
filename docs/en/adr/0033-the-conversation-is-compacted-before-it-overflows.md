# ADR-0033: the conversation is compacted before it overflows

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-01) — ported from gem-agent ADR-0006 with a lagent-specific trigger; implemented |
| Date | 2026-11-01 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator report: a lagent session on another machine failed with a context-overflow error. RFP Phase 2 lists compaction, to be designed after measuring the cost of discarding the server's cache, and the plan took it up once a real session needed it; this is that session |
| Amended | 2026-11-01 (ADR-0034): the retained tail is bounded by size as well as count, and §6 is judged on the cut the summariser actually reads up to. The Consequences line that relied on ADR-0029 to keep one round small did not hold on a local window: one round of four whole-file reads reached 113 % of it |
| Relates to | gem-agent ADR-0006 (compaction), gem-agent ADR-0021 §1/§3 (clear records, dead transcript), gem-agent ADR-0051 (the summary's re-read warning), gem-agent ADR-0057 (usage records), ADR-0003 and ADR-0013 (the session-facts message), ADR-0004 (`mcp_load` adds a server's schemas to the tool list), ADR-0029 (bounded tool output) |

## Context

A lagent session grows until the request no longer fits the model's
window, and then every turn fails: the server refuses the request and
the only recovery is `/clear`. The session that failed is not on this
machine, so neither its error text nor its server is known; the design
below does not depend on either.

gem-agent solved the same problem in ADR-0006: summarise the older part
of the conversation, keep the recent part verbatim. Its trigger is the
prompt-token count the server reported for the **previous** request,
checked between rounds. That trigger has two blind spots that matter
more here than on a 1M-token cloud window:

- **A round's growth is invisible until it is sent.** The results of one
  round of tool calls are appended after the last measurement. On a
  window of tens of thousands of tokens, one round of parallel reads can
  carry a session from below the threshold to past the window in one
  step — and the request that would have reported the size is the one
  that fails.
- **Not all growth is messages.** Measured over the operator's recorded
  sessions (410 rounds, Gemma 4 and Qwen 3.6/3.8): rounds whose prompt
  grew by 16–20k tokens on 3–7 KB of new messages. Those are `mcp_load`
  rounds — the loaded schemas ride every later request (ADR-0004) as
  tool definitions, not as history.

The same records measure the size estimate gem-agent uses after a
resume (bytes / 4): bytes per token is 3.5 at the median, 2.25 at the
10th percentile, Japanese and tool output both included. Dividing by 4
undercounts most rounds.

lagent also keeps per-session facts in the conversation rather than the
system prompt (ADR-0003, ADR-0013): the opening message carries the
untrusted-data tag name, the work directory, the MCP and skills
catalogue and recalled memory, and the system prompt points the model
at it. gem-agent has no such message; a ported cut would summarise it
away.

The RFP asked that compaction be designed after measuring what it costs
on a local server: rewriting history invalidates the server's prefix
cache. See Measurement.

## Decision

1. **Ported from gem-agent ADR-0006, as it is there** (source commit
   pinned in `internal/agent/compact.go`):
   - The most recent 6 messages stay verbatim; the cut moves forward
     past any tool result, so no retained tool result loses its call
     (the OpenAI-compatible API refuses that pairing exactly as Gemini
     does). The cut may fall inside the current turn's own tool loop —
     the case compaction exists for — so the turn's request can itself
     be summarised.
   - One summariser call, **with no tools offered**, over the rendered
     transcript of the replaced messages, nonce-wrapped as untrusted
     data under the defensive framing. Tool results and arguments are
     clipped when rendered (runes); image bytes become a line naming the
     image; a previous compaction's summary is rendered whole (gem-agent
     review round 4).
   - The summary replaces the head as one user message carrying it as an
     **attachment**, so the send-time wrap quotes it as data. The
     message says that file contents before this point are no longer
     verbatim and must be re-read before editing (gem-agent ADR-0051).
   - **Fail safe:** an error, an empty summary, or a cancelled call
     leaves the history exactly as it was. Two consecutive automatic
     failures turn automatic compaction off for the session — any
     successful compaction, `/compact` included, ends the run; "nothing
     to compact" is said once, not every round; a compaction does not
     fire again until the history has grown by 4 messages.
   - A `compaction` record (replaced count + summary message) goes to
     the transcript; the loader already replays it, so a resumed session
     comes back compacted. The record also carries the history length it
     saw (`before`), and the loader checks that the history it rebuilt
     has exactly that many messages — instead of refusing whenever any
     earlier line was unreadable, which a torn usage line would have
     made permanent once the session compacted. A `usage` record with source `compact`
     accounts the call; `/usage` shows it as its own line, and the
     footer's gauge never carries it.
   - `/compact` compacts by hand. `[agent].auto_compact` (default on)
     and `[agent].compact_at_pct` (default 80, 10–99) set the automatic
     one. With no known window, automatic compaction is off and
     `/compact` still works.
   - `/clear` (`Reset`), a new session (`Restart`) and a restore
     (`SetHistory`) forget the size measurements and the
     "already compacted at this size" mark; `/clear` and a new session
     also forget the failure count and the one-time warnings, and start
     from the configured `auto_compact` again — two failures turned it
     off for that conversation only. A compaction forgets the
     measurement too: it described the history just replaced.

2. **The session facts cross the cut verbatim.** The latest
   session-facts message, when it falls in the summarised head, leads
   the summary message as its unwrapped text, exactly as
   `AnnounceSession` wrote it; no facts message reaches the summariser
   (an older one names a tag this session no longer uses). Facts already
   in the retained tail stay where they are. A summary message that
   carries facts is still a summary: the next compaction reads its
   summary whole, without its framing or facts, and carries only the
   facts on.

3. **A summary that names the session's tag is refused.** The summary is
   wrapped in the session tag on every later request, and nlk/guard
   withholds content that contains its own tag name. A summary that
   copied the name ("keep exact identifiers") would be withheld on every
   request after the history it replaced was gone; it is checked before
   the swap and refused like any other failure.

4. **The trigger is the size of the request about to be sent, not of the
   last one.** Before every request the runtime estimates the whole
   request — system prompt, tool definitions, history, and the wrap
   around each tool result and attachment — and remembers the estimate
   it made when it last got a measurement. The projected size is

   `measured prompt of the last request + (estimate now − estimate then)`

   so the server's own count stays the base and only the change since
   is estimated: new tool results, newly loaded schemas, a compaction's
   shrinkage. With no measurement yet (first request, a resumed
   session), the projection is the estimate alone. A count below half
   its own request's estimate is not taken as a base — the measured
   ratio never goes there, and a server that reports only the tokens it
   did not have cached would turn a full window into a small number.
   Compaction runs when the projection reaches `compact_at_pct` of the
   window. A turn can start before the window lookup has answered (an
   argv first message, one-shot); the first judgment waits for it —
   once per session, at most the lookup's own 15 s timeout — rather
   than letting the first request out unjudged.

5. **The estimate is bytes / 3.** It sits between the measured median
   (3.5) and the 10th percentile (2.25): it overcounts English slightly,
   which only compacts a little early. Applied to a round's delta the
   error is a fraction of one round's growth; right after a compaction
   the delta is the whole summarised head, and the next measurement
   replaces the projection. An inline image counts a flat 2,000 tokens —
   unmeasured, and replaced by the server's count after the first
   request that carries it.

6. **A compaction that cannot get under the threshold is declined.** The
   system prompt, the tool definitions and the retained tail are out of
   its reach. When their estimate plus an allowance for the summary is
   already at the threshold, the runtime says so once — naming the share
   the system prompt and tool definitions take — instead of paying for
   a summariser call every two rounds that changes nothing.

7. **The summariser's request is bounded too.** Its rendered transcript
   must fit half the window by the same estimate. The per-message clips
   tighten first (1500/4000 → 600/1500 → 200/500 runes for tool output
   / text); if even the tightest does not fit, the cut moves back to an
   earlier safe boundary until the head does — a smaller compaction
   now, and another when the history has grown, rather than none. Only
   when no head of at least 4 messages fits is the compaction refused.
   On a 1M cloud window this never bound; on a local one an unbounded
   summariser call is the overflow this ADR exists to prevent, moved
   one call earlier.

8. **A summary cut by the output limit is a failure.** `finish_reason`
   `length` is a partial result for a conversation turn (RFP); for a
   summary it is a record whose end — what is open, what comes next —
   is missing, and replacing history with it would delete exactly that.
   The history stays as it was.

9. **The operator is told before the wait, not only after.** The
   summariser call and the cold request after it are a silent wait on a
   local server; an in-turn notice says compaction started, and the
   closing notice says how many messages were summarised and that their
   detail is now second-hand.

10. **A resumed session re-loads every server it loaded.** `mcp_load`
    replay on resume reads the transcript's records since the last
    clear (`session.CallArgs`), not the restored history: a compaction
    replaces what the model sees, not what the session did, and a load
    summarised out of the history is still in force.

## Consequences

- A session no longer dies at the window; it pays one summariser call
  and one cold request per compaction, and the summarised half's detail
  is second-hand.
- Growth that arrives in one round — parallel tool results, an
  `mcp_load` — is seen before it is sent.
- The trigger does not read any server's error text, so no server's
  wording is part of the contract.
- The summariser's system prompt and empty tool list differ from the
  main loop's, so on a server that keeps one prefix cache the request
  after a compaction re-reads the system prompt and tool block cold as
  well as the retained tail.
- A history whose retained tail alone exceeds the window (one enormous
  result) is still not rescued; it is declined with the reason.
  ADR-0029 bounds what a single tool returns, which is what keeps that
  case out of reach.
- Compaction cannot shrink the system prompt, the tool definitions or
  the instruction files.

## Alternatives considered

- **Keep gem-agent's trigger (last measurement only).** Rejected: blind
  to the round in which a small window is crossed, and to schema growth.
- **Retry after the server refuses the request.** Rejected: it needs the
  refusal recognised, and LM Studio, mlx-serve and Ollama word it
  differently — a text rule over an open set of servers. It also spends
  a request that was going to fail.
- **Drop the oldest messages without a summary.** Rejected for the
  reason gem-agent gave: decisions disappear silently.
- **Re-append the facts message after the summary** (as `/mcp reload`
  does). Equivalent for the model; carrying the text inside the summary
  message keeps the compaction one record that the existing loader
  replays unchanged.
- **Send the summariser request with the main loop's system prompt and
  tools,** the history as it is and a closing instruction, refusing any
  tool call in the reply. The whole prefix would be the one the server
  just cached, so the summariser would read almost nothing cold. Not
  taken now: the history would reach the summariser in the main loop's
  form rather than as one isolated transcript, the summariser would be
  offered tools it must not use, and on a server whose cache does not
  hold the full window it saves nothing. Revisit if the measured cost
  is the problem.
- **A lower default threshold for local servers' prefill cost.**
  Not taken: compaction's cost is the cold re-read, which a lower
  threshold makes more frequent, not cheaper.

## Measurement

mlx-serve (Qwen 3.6 35B A3B 4-bit with its MTP head, prefix cache 8 GB),
M2 Max 64 GB, 2026-11-01. A copy of a real recorded session — a
fact-checking conversation whose last request measured 86,427 tokens on
a 167,936-token window — resumed in one-shot with `--mcp off` and
`compact_at_pct = 40`, so the first request is the one judged. A
different tool block makes that request's prefix cold either way.

| | prompt tokens | time |
|---|---|---|
| Without compaction: the resumed request, cold | 85,265 | 266 s (prefill 320 tok/s) |
| Summariser call, cold | 22,640 in, 5,447 out (thinking included) | 96 s |
| The request after it, cold | 10,295 | 14 s |
| Compaction path, total | | 111 s |

- The projection said 56 % of the window (about 94k by the estimate)
  for a request the server counted at 85k — about 10 % high, the side
  the estimate is meant to err on.
- The summariser's rendered transcript was 71 KB for 52 messages: about
  a quarter of the history it replaced, inside the half-window bound
  with the loosest clip.
- 52 messages became one 2.8 KB summary; the 6 most recent stayed
  verbatim; the answer after it drew on both.
- Against a warm cache — a session that just ran its last request — the
  same compaction costs about 111 s where an uncompacted request would
  take seconds. That is the price stated in the notice, paid once per
  compaction and only past the threshold.
- The first attempt of this run did not get that far: during the cold
  85k prefill (before this ADR's one-shot window fix, so uncompacted)
  macOS aborted the GPU command buffer
  (`kIOGPUCommandBufferCallbackErrorImpactingInteractivity`) after
  about 5 minutes, and mlx-serve failed the next request too. The
  summariser's failure was handled as designed — history unchanged —
  and the run finished on ADR-0007's re-send. A long cold prefill is
  fragile on this machine; a compaction that keeps the request small
  avoids exactly that.
- The same run found that one-shot started its turn before the window
  lookup returned, so its first request went out unjudged. One-shot now
  settles the window before the turn.

## References

- gem-agent ADR-0006, ADR-0021, ADR-0051, ADR-0057
- `internal/agent/compact.go`, `internal/session/session.go`,
  `cmd/mcpload.go`
