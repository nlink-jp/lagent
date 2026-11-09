# ADR-0047: a root that fails at `/clear` costs only itself

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-09) |
| Date | 2026-11-09 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator: the two runtimes should behave the same here |
| Ports | gem-agent ADR-0114 §2 |
| Amends | ADR-0046 ("What differs here", item 8) |

## Context

ADR-0046 made `/clear` drop every added directory when one of them failed its sandbox probes, for simplicity. gem-agent, fixing the same stranded work directory, dropped only the failing one. One ejected volume should not cost the notes directory beside it, and naming the failing root makes the change no larger.

## Decision

1. `VerifyAddedRoots` names the failing root in a typed error (`sandbox.AddedRootError`).
2. `rotateWorkDir` rebuilds without that root, at most once per root, and removes it from the registry (`Registry.DropAddedDirs`), the watch and the transcript's record. The others stay.
3. The note names the root, its level and the one command that adds it again at that level: `/add-dir` for a write root, `/read-dir` for a read root.
4. A failure that names no root drops nothing, and the previous profile stays, as before.

## Consequences

- `/clear` keeps every added directory that still passes its probes.
- A root that fails is added again by hand, as before.

## Alternatives considered

- **Keep dropping every root** — rejected: it costs the operator the working directories for no saving in code.
