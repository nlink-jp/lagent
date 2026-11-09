# ADR-0045: a pipe without `-p` is the operator's own input, and two audit limits are accepted

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-09) |
| Date | 2026-11-09 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Ports | gem-agent ADR-0110 and the parts of gem-agent ADR-0113 that apply here |
| Relates to | gem-agent ADR-0023 (project trust), ADR-0041 (a path is judged where it leads) |

## Context

An audit of gem-agent raised three points that this runtime shares. The runtime guards what runs without being asked and records as a residual what needs an approved or operator's act.

## Decision

1. **A pipe without `-p` stays the operator's own input.** The plain REPL reads it through the path for typed input, so `!` and slash commands in it run and its text is the turn's instruction. Nothing runs unless the operator composes the pipe, the shape of `curl … | bash`, so it is not gated. The README says so, and that third-party text belongs to `-p`, which attaches it as data.
2. **Ancestor instruction files stay outside the trust gate.** Reading them is the drop-in behaviour. The reason recorded in the code, that a clone cannot plant files above itself, does not hold for a session started in a subdirectory of the clone: its root instruction file is then an ancestor. Starting there is the operator's act, and only instruction text crosses.
3. **A file tool's verdict and its write can see different links.** A directory inside a root can be swapped for a link between the verdict and the write, which can land a write under `.git` or `.claude` that a direct call would be refused. It needs an earlier approved write-lane command, a background child left running, and a change timed between the two. It is accepted for those preconditions.

## Consequences

- No behaviour changes.

## Alternatives considered

- **Refuse a pipe without `-p`** — gem-agent implemented and withdrew this before release: it breaks the script use to guard an act the operator performs themselves.
