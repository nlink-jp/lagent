# ADR-0006: A task bench measures the runtime before Phase 2 changes it

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-12 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The Phase 1 exit measurement did not become a comparison: on the same instructions the local model answered in one round where Gemini worked through a tool loop, and nothing in the repository could say for which tasks that happens |

## Context

The RFP's Phase 2 list (compaction, the model tier of auto-approval,
skills / memory / hooks, a local-oriented prompt revision, the thinking
toggle) is adopted or rejected "on the strength of Phase 1
measurements". Phase 1 produced one measurement and one observation:
the usage records are compatible with gem-usage-lens, and Gemma 4
answered a coding task in a single round. The observation is not
uniform — a lookup session on the same runtime loaded six MCP servers
and called their tools eleven times — so it is a property of some task
classes, and the boundary is unknown.

Every Phase 2 item depends on that boundary. Compaction and the model
tier assume a multi-round loop; a prompt revision can only be judged
against a before number; the thinking toggle trades tokens for
behaviour that has to be counted. Without a repeatable measurement each
item would be decided on an impression, which is how the earlier fork
was judged and discarded.

What exists for measurement today: `-p` runs a task unattended; the
transcript records every message, tool call and usage line; the
isolated-`HOME` E2E harness from Phase 1 shows a run can be made
independent of the operator's configuration. gem-usage-lens is not a
fit: `ingest` writes the operator's live store, the store's
deduplication keys on the sessions root, and the comparison it draws
(cost) is the one Phase 1 showed to be meaningless between runtimes.

## Decision

A **task bench** in the repository, run by hand, that measures the
runtime's behaviour on a fixed task set and prints what a Phase 2 ADR
needs as evidence.

1. **Tasks are fixtures.** `bench/tasks/<name>/` holds a `task.toml`
   (the prompt, the expectations, the servers it needs) and a
   `fixture/` directory that becomes a fresh project for every run. Six
   tasks cover the classes Phase 1 raised: search then answer, read
   then edit, a change across three files, a shell command for a
   number, an MCP lookup, and an image to look at. Expectations are
   mechanical — a file contains or lacks a string, the answer matches a
   pattern, at least N tool calls were made — so "completed" is a fact
   the bench states, not a judgment.
2. **Configurations are config files.** `bench/configs/<name>.toml` is
   a complete `config.toml`; a run of the bench names the
   configurations to compare. A prompt revision, a thinking toggle or a
   tool-description change is measured as one more configuration, or as
   one more binary (`--bin`), against the same tasks.
3. **Every run is isolated.** The runner gives each run its own `HOME`
   (config, global `mcp.json`), its own state root (`LAGENT_STATE_DIR`),
   and a fresh copy of the fixture; the operator's configuration,
   sessions and projects are never read or written. The bench's own
   stdio MCP server (`bench/mcpfixture`) answers the lookup task with
   canned data, so the MCP class needs no network and no operator
   server.
4. **Cases outside, configurations inside, one timeout per call.** For
   every task and repetition the configurations run back to back, so a
   swing in the local server's state (cache, load) hits every
   configuration alike; each run has its own deadline; progress prints
   as each run lands.
5. **The transcript is the measurement.** After a run the bench reads
   the session transcript with the runtime's own loader and counts:
   rounds (assistant messages carrying tool calls), tool calls by name,
   prompt / output / cached tokens from the usage records, wall time,
   the exit code, and whether the expectations held. `bench report`
   folds a results directory into one table per task and
   configuration: completion rate, median rounds, median tool calls,
   median prompt tokens, median wall time.
6. **The reference runtime runs the same tasks.** The runner carries a
   finite table of runtime profiles (binary flags, state-root variable,
   transcript layout); gem-agent's profile makes the ceiling behaviour
   on each task a number beside lagent's, without pretending the two
   cost the same. The profile is added when it is verified against
   gem-agent's binary, not before.

Nothing of the bench ships in the binary; `bench/` is a separate
`package main` in the module, run with `go run ./bench`, and its results
directory is ignored by git. A Phase 2 ADR quotes the table it was
decided on.

## Consequences

- The first bench run is the Phase 2-0 deliverable: the table of which
  task classes end in one round under the baseline configuration. The
  three hypotheses it tests are written down before the run: the
  system prompt gives no trigger to look at the repository first; the
  model answers coding tasks from prior knowledge when the task lets
  it; the thinking mode moves the plan out of the visible text.
- Every later Phase 2 item states its expected effect on this table
  before implementation and its measured effect after.
- A bench run costs real minutes on the local server (six tasks, three
  repetitions, two configurations is thirty-six runs; a cold prefix is
  about two minutes). The bench is run by hand, never in `make check`.
- The task set is small and synthetic. It measures whether the runtime
  enters multi-step work, not how well it codes; a task that the model
  can answer from the prompt alone is a task the bench must not
  contain, and the fixtures are written so that the answer is only in
  the files.

## Alternatives considered

- **Compare in gem-usage-lens.** Rejected: writes the operator's live
  store, keys on the sessions root, and measures cost, which Phase 1
  showed does not compare across runtimes. `verify --sessions-root`
  stays the compatibility check it already is.
- **Manual sessions, read afterwards.** Rejected: not repeatable, no
  isolation from the operator's configuration, and the one observation
  it produced is exactly what could not be turned into a boundary.
- **A `go test` benchmark.** Rejected: minutes-long runs against a live
  server do not belong in the test suite, and buffered test output
  hides a stalled run until the end (the knowledge base's bench entry).
- **Real MCP servers from the operator's configuration.** Rejected for
  the fixture: their answers change, some need the network, and a
  bench that depends on the operator's `mcp.json` is not isolated. An
  operator can still point a configuration at them for a one-off.
