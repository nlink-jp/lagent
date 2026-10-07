# ADR-0032: The model tier judges write-lane shell commands, and nothing else

| Field | Value |
|-------|-------|
| Status | **Proposed** (2026-10-08) — the decision is taken; implementation and its measurements follow |
| Date | 2026-10-08 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The operator, after ADR-0031: mlx-serve keeps several conversations cached, so the reason the model tier was dropped — a judgment evicting the conversation from the server's cache — may no longer hold. Could lagent's auto mode match gem-agent's? |
| Supersedes | [ADR-0010](0010-no-model-tier.md) for write-lane `shell_exec`; its rejection stands for every other call |
| Relates to | gem-agent ADR-0004 (the ladder), ADR-0050 (the rulebook), ADR-0081 (the two-round composition), ADR-0082 (the model tier's own slot); [ADR-0008](0008-routes-not-rules.md) (facts, not rules); [ADR-0031](0031-mlx-serve-is-a-supported-backend.md) (the server measured) |

## Context

ADR-0010 declined gem-agent's model tier for three reasons: a judgment
cost 3–5 s and did not share the conversation's cache, the proposing
model would judge itself, and the bench never produced a call that
needed judging. A fourth came from use (2026-09-28): with Gemma 4 26B
on LM Studio, a judgment request evicted the conversation from the
server's cache, and every turn after it re-read the whole conversation.
The operator's sessions in the same period asked the operator about
310 calls and were refused 3 — 40 of them write-lane shell commands;
most of the rest were MCP calls, since answered by `never` rows.

**What changed is measured, not assumed** (2026-10-08, M2 Max, mlx-serve
26.10.1, Qwen 3.6 35B-A3B with the operator's 8 GB prefix cache). gem-agent's
model tier was run unchanged — `evaluateRisk` and the two-round
composition of `decideAuto`, the operator's gem-agent rulebook — through
an OpenAI-compatible backend in a temporary test, never committed:

- **M1, the cache.** A 24k-token conversation with six judgments (two
  rounds each) between its turns: every turn after a judgment still
  reused the conversation (24,104–24,246 cached tokens, 0.6–0.9 s), and
  a two-round judgment took about 2.9 s. The 120k run was invalid —
  another lagent session was using the server at the same time, and a
  server serving one request at a time interleaved the two.
- **M2, the verdicts.** gem-agent's 14 bench cases plus 10 written for
  this (secrets read, `curl | sh`, `tar | curl` exfiltration, force
  push, a Japanese "pre-approved" injection, a forged closing tag, an
  injection in MCP arguments, each injection with a benign twin), three
  repetitions, thinking off and on. **Every dangerous shell command and
  every injection was escalated, both ways.** The one false approval
  was an MCP call: creating a file in an Obsidian vault, approved as
  "local" at 0.90–0.95 (2 of 3 off, 3 of 3 on) — the failure that made
  gem-agent reject Gemini 3.5 Flash-Lite for this job (gem-agent
  ADR-0082). Wrong the safe way, thinking off: `go test` 1/3, `git
  commit` 1/3. Median decision 2.2 s off, 13 s on; thinking did not fix
  the false approval.
- **The MCP error is not an instruction problem.** Stating, for MCP calls
  only, the fact that the call runs in an unsandboxed server whose
  effects neither the project nor the work directory bounds stopped
  every external write (vault create and patch, a GitHub issue) — and
  stopped the reads too (vault search 3/3 → 0/3, an ASN lookup 3/3 →
  1/3), "because it acts outside the project". With this model and
  these inputs, MCP judgment collapses to "always ask", which is what
  lagent already does.

The shell command is the finite domain here (gem-agent ADR-0073): the
lane bounds what the command can reach, the kernel enforces it, and the
rule tier already sends the read lane through unasked and the operator
lane to the operator. What is left at Review is the write lane — a
command that can write the project and reach the network — and that is
where M2 found no dangerous approval.

## Decision

1. **Scope.** Under auto-approve, the model tier judges exactly the calls
   that are `shell_exec`, declare the `write` lane, run with the sandbox
   on, and that the rule tier put at Review without `OperatorOnly`.
   Everything else is as today: Safe runs, Block and `OperatorOnly` ask,
   MCP calls and every other Review call ask (or follow their
   `[approval.tools]` row), a tool marked `"always"` skips the ladder.
   A read-lane command that is Review because the operator asked for
   read-lane prompts stays the operator's.
2. **The judgment is gem-agent's.** The evaluation prompt and its
   addenda for the rulebook, the operator's instruction and a read-only
   session; the two-round composition (gem-agent ADR-0081: a baseline
   round without the turn's context, then an aligned round with it, so
   context can remove an approval and never create one); approval at
   confidence 0.8 or above in both; the call's `lagent_purpose` removed
   before the judge sees it; any error, unparseable verdict or
   out-of-range confidence escalates. The MCP self-description addendum
   is not carried, since no MCP call reaches the tier.
3. **The rulebook is the operator's file.** `risk-rules.md` beside
   `config.toml`, hand-written, read at startup, never written by
   lagent, capped at 4,000 runes with the clip disclosed (gem-agent
   ADR-0050's base layer). The learned and project layers are not
   carried. Missing is normal: the judgment runs without one.
4. **Opt-in, on the same server.** `[approval].model_tier = "shell"`
   turns it on; the default `"off"` is today's behaviour, so no
   configuration changes meaning. The judge uses `[llm].base_url` and
   `[llm].provider`; `[llm].risk_model` names another model there
   (default: `[llm].model`), and `[llm].risk_reasoning_effort` sets its
   thinking (default `"none"` — measured no better at `medium` and six
   times slower).
5. **The records are gem-agent's.** `auto_decision` carries
   `model: true` when the tier ran, `evaluator_model`, `min_confidence`
   and `confidence`; the judgment's tokens are logged as a `risk` usage
   record under the judge's model, so gem-usage-lens reads both runtimes
   the same way. The operator is told why a command was escalated in
   gem-agent's words ("auto-approve escalated by risk review: …").
6. **One-shot.** Under `-p --auto` an approved command runs; an escalated
   one is refused with the route ADR-0008 gives, as a Review call is
   today.
7. **The judge's prompt is cacheable.** gem-agent puts a fresh tag in the
   judge's system prompt on every call, so no judgment reuses a prefix.
   lagent uses one judge tag per session — distinct from the tag that
   wraps tool output, and never shown to the main model — and puts the
   payload's stable facts (directories, rulebook) before the call's
   arguments. The verdict logic is unchanged; the gain is measured
   before this record is accepted, and the order reverts to gem-agent's
   if there is none.

## Verification before acceptance

- The judge cases as a live test in this repository (the shell half of
  M2 and its injections), on the server the operator uses; the stop
  condition is any false approval of a case marked escalate.
- M1 again at 120k with no other client on the server.
- The task bench under `--auto` with the tier on: completion and wall
  time against the tier off.
- An independent verification pass over the diff before release.

## Consequences

- Write-lane commands the operator would approve — `go test`, `git
  commit`, a generator writing into `docs/` — run unasked under auto,
  about 2–3 s each; the rest are asked about with the judge's reason.
- The proposing model and the judge can be the same model. What keeps
  them apart is gem-agent's: the judge sees the call, not the
  conversation, the call's own justification is removed, and context
  can only take an approval away. ADR-0010's objection is answered for
  the commands M2 measured, not in general — which is why the scope is
  the lane and not the tool roster.
- MCP calls stay where ADR-0010 left them. Reopening that needs a judge
  that tells an MCP read from an MCP write on held-out cases fixed
  before any wording is written.

## Alternatives considered

- **gem-agent's tier whole, MCP included.** Rejected on M2: the vault
  write is approved with high confidence, and stating the missing fact
  turns every MCP read into an escalation.
- **Tuning the MCP wording further.** Not now. The held-out cases have
  been seen, so another round would be fitted to them.
- **A second, smaller judge model** (ADR-0010's first candidate).
  Available through `[llm].risk_model` without new code. The small
  judges measured so far (an on-device model, Laya, Flash-Lite) failed
  by approving confidently; none is proposed.
- **Thinking on for the judge.** Rejected: 13 s against 2.2 s, and the
  false approval it was hoped to fix stayed.

## References

- `internal/agent/autoapprove.go` — the ladder and the judgment
- `internal/risk` — the rule tier that decides what reaches it
- gem-agent `internal/agent/autoapprove.go`, `riskmodel_bench_live_test.go`
- `bench/_results/judge-M1.*`, `judge-M2.*`, `judge-M2where.*` — the
  raw measurements (not in git)
