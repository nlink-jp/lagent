# ADR-0009: Thinking is an operator key, passed verbatim, and measured before it is a default

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-12 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The replay experiment for ADR-0007: at the point where the identical request came back empty 10/10, thinking on came back empty 0/10 — and the runtime had no way to turn thinking on |

## Context

The RFP's Phase 2 lists "Gemma 4's thinking toggle" as an item to
adopt or reject on measurement. Until now the runtime sent no
reasoning setting at all, so every measurement — the baseline bench,
the empty-completion traces — was of thinking off: LM Studio's default
for this model (measured: zero reasoning tokens on a request with no
setting).

The replay experiment behind ADR-0007's second amendment measured the
toggle's first effect. At the request where the model had just
received a compile error from its verification run, the identical
request returned an empty completion ten times in ten; with
`reasoning_effort` set, ten tool calls in ten, at a median of 3.0 s
per call against 0.8 s. The transient nudge line reached the same
0/10 at 1.1 s, which is why ADR-0007 took the line and not the toggle
as its remedy. What the toggle does to a whole task — rounds, tokens,
wall time, completion — is unmeasured.

The vocabulary is the server's. LM Studio's endpoint validates the
OpenAI values (`none`, `minimal`, `low`, `medium`, `high`, `xhigh`) and
then maps them to what the model supports; for Gemma 4 that is on or
off, so `none` is off and everything else is on, with a line in the
server log about the mapping. Sending `on` or `off` directly is a 400.
Another server would take other values. The runtime has no reason to
own that table.

## Decision

1. **`[llm].reasoning_effort` (and `LAGENT_REASONING_EFFORT`) is sent
   verbatim** as the request's `reasoning_effort` when set; unset
   sends nothing, which is the server's default. The runtime neither
   validates nor translates the value: the vocabulary belongs to the
   server, the mapping to the model is the server's, and its log says
   what it did. `/settings` shows the key.
2. **The default stays unset until the bench says otherwise.** The
   bench (ADR-0006) runs the six tasks with the baseline configuration
   and one that sets `reasoning_effort = "low"` (LM Studio: on), three
   repetitions each, cases outside and configurations inside. The
   numbers go into `reference/bench.md` under this record's date; a
   default of on is a change to the shipped configuration, decided on
   those numbers and recorded here as an amendment, not assumed from
   the one point already measured.
3. **Reasoning content is never stored.** The backend already streams
   `reasoning_content` to the display only (`[tui].show_thoughts`) and
   drops it from the history; a thinking turn therefore leaves the
   prefix cache what it was, and the transcript carries the reasoning
   token count in its usage record, nothing more.

## Consequences

- Thinking on costs about three times the latency per model call at
  the points measured and adds reasoning tokens to `output`; whether
  it saves rounds is what the bench run decides.
- The empty-completion fault does not occur with thinking on at the
  point measured; ADR-0007's line covers the same point at a fraction
  of the cost, so the key is not a remedy for that fault.
- Bench measurement, 2026-09-12: see `reference/bench.md`.

## Alternatives considered

- **A boolean `thinking = true|false` the runtime translates per
  provider.** Rejected: the runtime would own a table (LM Studio's
  mapping, an OpenAI-style server's levels, whatever comes next) that
  the servers already own and log; a verbatim string is the one shape
  that never lies about what was sent.
- **Default on now, on the strength of the replay.** Rejected: one
  point at three times the latency is not a task-level measurement,
  and ADR-0006 exists so that a default is decided on the bench.
- **Turn thinking on only for the re-send after an empty completion.**
  Rejected: the one-line nudge reaches the same 0/10 at a third of the
  latency and needs no server vocabulary.
