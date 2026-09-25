# ADR-0023: an agent runtime in daily use — repositioning and promotion to cli-series

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-26 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator decision: "promote lagent from lab to a product", on the evidence that it is in daily use, and placed in cli-series rather than the lite-series the RFP anticipated |
| Relates to | gem-agent ADR-0061 (the same move on the sibling runtime), RFP §1, §3, §4 and §6 (amended here) |

## Context

The RFP wrote lagent's charter as an experiment. That premise was
load-bearing in five places:

1. **Purpose** — "measure how far a local LLM can carry an agent
   runtime … on the same scale as gem-agent" (RFP §1). The README's
   opening, the org profile row, the website card and the GitHub About
   text all say the runtime exists to measure.
2. **Target user** — "the developer, for now. Experimental;
   distribution is not assumed" (RFP §1), with "distribution and team
   use (experimental stage)" in the explicit out-of-scope list (§3).
3. **Status** — the README's "experimental (lab-series)" note, which
   still describes Phase 2 as the next step.
4. **Series** — lab-series, with promotion to lite-series "reconsidered
   once practicality is shown" (RFP §6).
5. **The health check** — RFP §4 Phase 3 lists "a health-check
   procedure (gem-agent's drill equivalent)". It was not built.

Reality moved out from under the premise. Phase 2 settled every item
it listed except compaction, which waits on a real session crossing
half the window. Twenty-two ADRs, releases through v0.10.2 with signed
and notarized archives, a Homebrew formula, and two external risk
reviews answered. And the operator uses lagent daily in real work.
The measurements the charter asked for have been taken; what remains
is a tool in use.

## Decision

**1. lagent is a coding-agent runtime for work that should not go to a
cloud API.** Its identity is the RFP's problem statement, not its
purpose line: offline environments, confidential projects, and work
where per-token spend is unwanted, carried by the same auditable
minimal loop as gem-agent (read / edit / shell / MCP / approval) on a
local LLM served over an OpenAI-compatible API. The bench (ADR-0006)
stays, as the instrument that decides changes, not as the reason the
runtime exists.

**2. Promotion, effective now, on the evidence of daily use.** The
operator's judgment is that real use answers what a promotion bar
would ask — does it work in practice. No bar is written after the
fact to be passed.

**3. cli-series, not lite-series.** RFP §6 named lite-series as the
place to reconsider. lite-series' conventions describe small, focused
tools that compose in pipelines, need no interactive UI, cross-compile
to Linux, Windows and darwin/amd64, and take `LITE_<PROJECT>_`
environment variables. lagent is a TUI agent, macOS arm64-only by
design (the lanes are `sandbox-exec`), with `LAGENT_*` variables.
Placed there it would be a standing exception to each of those rules.
cli-series already holds gem-agent — the same design, the same
platform restriction, already listed there as a per-tool build quirk
— and llm-cli, the local-LLM client lagent's backend descends from
(ADR-0001).

**4. The cli-series stability contract applies from now.** The
operator-facing surface — flags and subcommands, config keys,
`LAGENT_*` variables, the transcript and usage-record formats, the
hook contracts, the skill and memory locations — is a promise.
Breaking changes go through the organization's breaking-change
process (user confirmation → compatibility plan → implementation),
not a CHANGELOG line. A 0.x version number and a user count of one do
not soften this; the contract is what makes the promise checkable.

**5. The health-check procedure is not built, and is not a
condition.** Regular use provides the rot detection a drill exists
for — the conclusion gem-agent ADR-0061 reached when it retired its
monthly drill. The RFP line stays with a note pointing here.

## Consequences

- Positioning surfaces are rewritten from the new premise rather than
  word-patched: README (both languages), AGENTS.md, the org profile
  row, the website card (moved from Experimental to the AI Agents
  section), the GitHub About text. RFP §1, §3, §4 Phase 3 and §6 carry
  notes pointing here; the Discussion Log stays as history.
- The repository moves from the lab-series umbrella to the cli-series
  umbrella; the local path becomes `cli-series/lagent`. The repository
  URL and the module path do not change. The org's `check-org.sh`,
  which names the runtime's `internal/sandbox/lane.go` by path, and
  gem-agent's AGENTS.md sibling paragraph follow the move.
- The shared-mechanism rule with gem-agent (AGENTS.md: a defect in a
  mechanism both runtimes have is fixed in both) is unchanged; both
  runtimes now sit under the same contract.
- Accepted risk: the surface changed fast while the runtime was a lab
  project, and from now a change to a flag, a config key or a record
  format costs the process in decision 4. That is the intended price.
- Historical documents — the ADRs, the RFP Discussion Log, the risk
  review submissions, other repositories' RFPs that call lagent a
  lab-series runtime — keep their era's language. A record speaks for
  its own time.
