# ADR-0001: gem-agent and llm-cli are porting sources, pinned by commit

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-10 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The first package cannot be brought over until the relationship to gem-agent is fixed: fork, shared library, or new repository with recorded sources |

## Context

lagent is gem-agent's design on a local LLM (RFP §1). Three shapes were
open for the relationship between the two code bases:

1. **A fork** of a gem-agent snapshot with the Vertex backend replaced.
   This was tried (`_wip/local-agent`, discarded 2026-09-09). The
   separation review found two failure classes that a subtractive fork
   produces by construction: seams left where upstream features were
   plugged with nil injection points (media upload, web tools, error
   text pointing at `[gcp].bucket`, dead types), and reference
   documents describing gem-agent's behaviour as current.
2. **A shared library** extracted from gem-agent. gem-agent is in
   cli-series under its stability contract; carving its packages into a
   library is a breaking restructure of a released tool for the benefit
   of an experiment.
3. **A new repository** that brings gem-agent's pure packages over one
   at a time, each with its source commit recorded.

llm-cli's OpenAI-compatible client (stdlib `net/http`, hand-written
SSE, no SDK) is the second source, for the backend.

## Decision

lagent is a new repository. gem-agent and llm-cli are **porting sources,
not upstreams**.

- The sources are pinned to the commits reviewed when this record was
  written:

  | Source | Tag | Commit |
  |---|---|---|
  | gem-agent | v0.74.0 | `be7609980022e38314268c58ca94a6517e6f5d28` |
  | llm-cli | v0.2.0 + 7 | `6237a64ce4595a6e3cc5690f9fdddf6a6b407b84` |
  | nlk | v0.5.2 | (Go module, by version) |

- A package brought over states its source in the package doc comment:
  the source repository, the package path, and the commit above. A
  package written new says nothing.
- A port is a copy of what lagent needs, not of what the source has.
  The port drops every reference to a feature in ADR-0002's list before
  it compiles here — code, config keys, error text, and doc comments.
- Nothing tracks the sources afterwards. A later gem-agent fix reaches
  lagent only if someone decides to port it, as a change with its own
  commit naming the source commit.
- Candidates for porting, in the order Phase 1 needs them: sandbox,
  bounded, tools, risk (rule tier only), approve, session, mcp, repl,
  tui, uitext, config (loader shape), instructions, mention, ignore,
  archtest. The backend (`internal/llm`) is written new around llm-cli's
  client.

## Consequences

- The two runtimes can diverge freely. That is the point: lagent's
  measurements must be attributable to the model, not to a drifting
  code base.
- A ported package carries gem-agent's tests, so the port is verified
  against the same behaviour on arrival.
- The fork's two failure classes cannot recur through the mechanism
  that produced them: there is no upstream tree to subtract from, and
  no upstream documents to inherit.
- The cost is duplication. A defect fixed in gem-agent may persist in
  lagent until someone ports the fix.

## Alternatives considered

- **Fork (1)** — rejected on the evidence of the discarded attempt.
- **Shared library (2)** — rejected: touches a released tool under the
  cli-series stability contract for an experiment's benefit; revisit
  only if lagent is promoted and the duplication has a measured cost.
- **Adding an OpenAI backend to gem-agent** — rejected for the same
  reason, and because gem-agent's charter is one backend.

## References

- RFP §3 "Shape of the port" — `docs/en/lagent-rfp.md`
- ADR-0002 — the features the port must not carry
