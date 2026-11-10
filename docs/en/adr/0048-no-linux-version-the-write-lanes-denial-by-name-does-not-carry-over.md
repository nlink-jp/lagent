# ADR-0048: No Linux version — the write lane's denial by name does not carry over

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-11) |
| Date | 2026-11-11 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | A user asked gem-agent for a Linux version, with WSL2 as the Windows route. It was measured and declined there (gem-agent ADR-0115). The mechanism measured is the one this runtime shares, so the operator asked for the decision to be recorded here as well |
| Ports | gem-agent ADR-0115 |
| Relates to | ADR-0001 (the porting source: the lanes and `internal/sandbox`), ADR-0032 (the model tier judges write-lane shell commands) |

## Context

lagent is macOS-only because its isolation is built on `sandbox-exec`, as gem-agent's is. Whether another kernel could carry the same guarantees had not been measured until gem-agent ADR-0115.

The property in question is the same here. A write to a persistent file — `AGENTS.md`, `CLAUDE.md`, `.mcp.json`, `.lagent.toml`, `.claude/` and the rest of `sandbox.PersistentFiles` — through `write_file` or `edit_file` always asks the operator. A `shell_exec` call in the write lane is not judged by its text for this; the kernel refuses those files **by name, at any depth, whether or not the file exists yet**. A model that is refused by `write_file` tries `shell_exec` next, and on macOS that attempt fails in the kernel whatever the command's spelling.

Under auto-approve the model tier may approve a write-lane shell command (ADR-0032), so without the kernel's refusal a new instruction file could appear with no operator having read anything.

## Measurement

Not repeated for this record. `internal/sandbox` and the lanes are shared with gem-agent, and nothing in the measurement depends on the runtime: it concerns what an unprivileged process can be denied on Linux. gem-agent ADR-0115 holds the machines, the method (every refusal has a control that must succeed without the mechanism) and the full table. What it found, on two Linux machines, 2026-11-11:

| What the macOS write lane guarantees | Landlock alone | Namespaces |
|---|---|---|
| An existing `AGENTS.md` cannot be written, unlinked or renamed | **allowed** | refused |
| A new `AGENTS.md` cannot be created | **allowed** | **allowed** |
| A directory holding an `AGENTS.md` cannot be renamed | not measured | **allowed** |

What exists can be protected, and what does not exist yet cannot. The read lane's guarantees (no write, no network, no signal to the parent, credential directories unreadable) did carry over.

## Decision

1. **There is no Linux version of lagent.** It stays macOS-only.
2. **WSL2 is not offered as the Windows route.** It is the Linux version.
3. **The sandbox control is not put behind an abstraction** for a second mechanism.
4. **This is reopened by a mechanism, not by demand**, on the condition gem-agent ADR-0115 §4 states: an unprivileged kernel facility that refuses creation and rename by name pattern inside a writable tree, shown by probes with controls. If gem-agent reopens it, this record is reopened with it — the mechanism is shared and is fixed in both (AGENTS.md, "The sibling runtime").

## Consequences

- The Makefile keeps its single target, and `AGENTS.md` and `CLAUDE.md` keep "do not add linux/windows build targets", now pointing here.
- A local model on a Linux machine is a natural pairing, and it is not served. That is the cost.

## Alternatives considered

The four weighed in gem-agent ADR-0115 apply unchanged, and are not taken for the same reasons: unconfined as the Linux default; the read lane reproduced with every write-lane command asking; detection after the write instead of prevention; writing into an overlay and carrying the result through the name matcher. The second would also withdraw ADR-0032 on Linux.

## References

- gem-agent ADR-0115 — <https://github.com/nlink-jp/gem-agent/blob/main/docs/en/adr/0115-no-linux-version-the-write-lanes-denial-by-name-does-not-carry-over.md>
- ADR-0001, ADR-0032
