# ADR-0007: An empty completion is asked again once

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
reason — is **asked again once** with the identical request. The
history is unchanged (nothing was stored), so the request is byte for
byte the one that just failed and the server's prefix cache answers it
in seconds. The retry is noted to the operator (`OnNotice`) and the
transcript records both attempts as `assistant_empty`, the first with
`retried: true`. A second empty completion ends the turn as before,
with the same error and the same message.

Bounded on purpose: one retry, never a loop. The fault is a single
mis-sampled token; if two requests in a row produce it, something else
is wrong and the operator should see it. The retry consumes a round of
the turn budget like any model call, and a cancelled context is not
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
  server that has stopped answering, and the fault measured is one
  token, not a state.
- **Ask the server to stop routing stray tokens into reasoning.**
  Out of reach: the behaviour is the inference server's and the
  model's; the runtime sees only the result.
- **Treat `<tool_call|>` in the reasoning channel as a tool call.**
  Rejected: there is no call to recover — no name, no arguments —
  and reading the reasoning channel for control would make the
  runtime's behaviour depend on a server's private token handling.
