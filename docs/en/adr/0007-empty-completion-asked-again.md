# ADR-0007: An empty completion is asked again, twice at most

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-12 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The bench's `read-edit` task failed 2/3 and then 2/8 on a completion that carried nothing — and the raw stream showed why |

## Context

In the baseline bench (ADR-0006) the `read-edit` task ended twice in
three runs with "the model returned no usable response (empty text, no
tool call)". The transcript recorded finish reason `stop`, three or
four output tokens, one reasoning token. Eight traced repetitions
(`LAGENT_LLM_TRACE`) reproduced it twice, both directly after
`read_file` returned the file to fix, and the raw stream was the same
each time: one delta with `reasoning_content` `<tool_call|>`, one empty
delta, `finish_reason: stop`, usage with `completion_tokens: 4`.

The model began a tool call with a broken opener token, the server
routed the stray token into the reasoning channel, and generation
stopped. Nothing reached the text or the tool-call channel. The very
same request, sent again, produced a normal tool call in every other
repetition — the glitch is in the sampling of one token, not in the
conversation.

The runtime's answer today is gem-agent's: report the empty response,
store nothing, and end the turn with an error. Interactively the
operator re-sends; in `-p` the run is over and the pipeline sees an
error for a fault that a second request would not have.

## Decision

An empty completion — no text, no tool call, whatever the finish
reason — is **asked again, up to twice**, with the same history and
**one transient line appended**: "Reply now: give your final answer as
text, or call a tool." The history is unchanged (nothing was stored,
and the line is sent, never kept), so the request is the one that just
failed plus that line, and the server's prefix cache still answers it
in seconds. Each re-send is noted to the operator (`OnNotice`) and the
transcript records every attempt as `assistant_empty`, the re-sent
ones with `retried: true`. A third empty completion ends the turn as
before, with the same error and the same message.

The line is there because the identical request is not always enough
(second amendment, 2026-09-12). Traced with `LAGENT_LLM_TRACE` and
replayed ten times each: at the point after `read_file` the identical
request came back empty 4/10; at the point after a compile error from
the verification run it came back empty **10/10** — deterministic, so
no number of identical re-sends would recover it — and with the one
line appended, 0/10 at both points. Thinking on (`reasoning_effort`)
also gave 0/10 at that point, at three times the latency per call;
that is ADR-0009's measurement, not this record's remedy. The line
names both routes a turn can take and prefers neither, so it steers
the token path, not the answer.

Bounded on purpose: two retries, never a loop. The bound was one when
this record was written, on the reading that the fault is a one-off
mis-sample; the same day a run hit two empties in a row, and the traced
request replayed four times against the server came back empty twice —
at the point it strikes, the fault is a coin flip, not a one-off. One
retry then leaves a quarter of the cases failing; two leave an eighth,
at the cost of a few cached seconds. Three identical requests all empty
is something the operator should see. A retry consumes a round of the
turn budget like any model call, and a cancelled context is not
retried.

## Consequences

- The bench's `read-edit` class loses its dominant failure. The measured
  effect is recorded in `reference/bench.md` beside the baseline.
- A one-shot pipeline no longer fails on a fault the next request
  would not show.
- The `assistant_empty` record now carries `retried`, so a transcript
  says whether the runtime asked again and whether it helped.
- The empty completion's other shape — the model narrating the change
  it "will" make instead of making it — is not this record's subject.
  That one is a plan, not a glitch, and is measured for the prompt
  revision that follows.

## Alternatives considered

- **Keep failing the turn.** Rejected: the operator's only remedy is to
  re-send the same message, which is exactly what the runtime can do,
  and `-p` has no operator.
- **Retry until something arrives.** Rejected: unbounded retries hide a
  server that has stopped answering; the bound is set from the measured
  rate, not removed.
- **Re-send the identical request only.** The first two versions of
  this record; replaced by the second amendment when a point was
  measured at which the identical request is empty every time. The
  transient line keeps the prefix cache (it is appended after the
  history) and is kept out of the history and the transcript.
- **A different seed on retry.** Not measured; the line was, and it
  also covers the deterministic point where a seed change is a guess.
- **Ask the server to stop routing stray tokens into reasoning.**
  Out of reach: the behaviour is the inference server's and the
  model's; the runtime sees only the result.
- **Treat `<tool_call|>` in the reasoning channel as a tool call.**
  Rejected: there is no call to recover — no name, no arguments —
  and reading the reasoning channel for control would make the
  runtime's behaviour depend on a server's private token handling.
