# ADR-0010: The model tier of auto-approval is not adopted

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-12 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The RFP's Phase 2 lists the model tier "to be measured before adoption"; the bench has now measured what it would have judged, and the cost a local model would charge for it |

## Context

gem-agent's auto-approve ladder has two tiers. The rule tier
classifies a call by what it is — the lane a shell declared, the file
a tool names, whether the tool is an MCP server's — into Safe, Review
and Block. The model tier then takes every Review-tier call under
`--auto` and asks a second model, with the operator's rulebook and the
call's arguments in the payload, whether to approve it, escalate it,
or block it. gem-agent measured that tier costing 2–11 s per decision
on Vertex AI and moved it to a model slot of its own (gem-agent
ADR-0082) to keep it under four.

lagent shipped the rule tier only (ADR-0002, RFP §3: "measured in
Phase 2 before adoption"), with `AutoDecision.ModelConsulted` always
false for record parity. What Phase 2 has measured since:

- **What the tier would have judged.** Over the bench's 36 baseline
  runs on the six tasks (ADR-0006, the runs of 2026-09-12), the rule
  tier made 22 auto decisions per 18 runs; 18 were Safe and ran, four
  to five were Review — every one a `shell_exec` in the write lane,
  every one a verification `go run` the model could have run in the
  read lane and, in the other runs, did. No MCP call reached Review
  (the bench's server carries a `"never"` policy, as an operator's
  lookup server would), no file write did (edits inside the project
  are Safe), and no Block occurred. In one-shot the Review calls were
  denied and every run completed anyway; interactively they would
  have been one prompt each.
- **What it would cost here.** A model-tier decision is one more
  request to the same local server, with a different prefix (the risk
  prompt, not the conversation), so it does not ride the prefix cache
  the main loop lives on: about 1.5–2k tokens of prompt processing at
  the measured 580 tok/s plus a JSON verdict — three to five seconds
  per decision on the reference machine, and the main turn waits for
  it. Thinking on for the verdict (ADR-0009's cost measurement) would
  make it ten. gem-agent's 3.65 s floor came from a second, smaller
  model; lagent has one model and no second slot.
- **Who would be judging.** With one local model, the tier would be the
  proposer evaluating its own proposal — the objection gem-agent ADR-0020
  §4 raised for the memory file and ADR-0082 answered with a separate
  model. The rule tier reads no command text for intent on purpose
  (gem-agent ADR-0073: the lane decides); a model tier reads nothing
  else.

The operator's levers for a Review-tier call already exist and are
cheaper than a verdict: a `"never"` row in `[approval.tools]` for a
tool the operator has decided about, `--allow` for one run, the
read-lane facts ADR-0008 made true so that inspection and verification
never reach Review in the first place. A model tier's remaining value —
per-call judgment of a write-lane command the operator has not decided
about — is the one thing the bench has not needed once.

## Decision

The model tier is **not adopted**. The RFP's Phase 2 item closes as
rejected on measurement, not deferred. The ladder stays as it is:
Safe runs under `--auto`, Review asks the operator (or is denied
unattended, with the route ADR-0008 gives it), Block always asks. The
`ModelConsulted` field stays false for gem-usage-lens record parity.

The line that would reopen this record is a measurement, not an
opinion: a task class on the bench where Review-tier calls the
operator would have approved stop unattended runs from completing,
and where no `[approval.tools]` row or lane fact removes them. The
bench's `gate_decision` and `auto_decision` records are where that
count comes from.

## Consequences

- Phase 2's approval work is done: no second model call, no risk
  prompt, no rulebook payload, no verdict schema to keep in step with
  gem-agent. The decision ladder remains one function
  (`internal/agent/decision.go`) that the architecture test pins.
- An unattended run that needs a write-lane shell the operator has not
  pre-approved stops there and says so. That is the ceiling of one-shot
  use on this runtime, stated rather than papered over by a verdict
  the proposer wrote.
- ADR-0002's row for the model tier reads "Phase 2" today; this record
  is its outcome, and the RFP's Phase 2 list carries a note pointing
  here.

## Alternatives considered

- **Adopt gem-agent's tier as is, on the main model.** Rejected: three
  to five seconds per Review-tier decision on a runtime whose whole
  turn is often ten, judged by the proposer, for calls the bench shows
  the operator's rows and the lanes already cover.
- **A smaller second model for the tier** (gem-agent ADR-0082's
  remedy). Rejected for now: a second model loaded beside a 26B one
  on the reference machine is memory the main model needs, and the
  bench has produced no decision for it to make. If the reopening
  measurement arrives, this is the shape to measure first.
- **Approve write-lane verification commands by pattern** (`go test`,
  `go vet`). Rejected: "verification" is a word in a command line, not
  a lane (gem-agent ADR-0073), and ADR-0008 removed the need — those
  commands run in the read lane, unasked, when the model declares it.
