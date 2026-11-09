# ADR-0046: added directories are roots the operator grants

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-09) |
| Date | 2026-11-09 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator: gem-agent's `/add-dir` is missing here |
| Ports | gem-agent ADR-0099 as amended at its v0.94.0 review, and gem-agent ADR-0107 §1 and §2 |
| Relates to | ADR-0003 (per-session facts leave the system prompt), ADR-0008 (supply a route, not a rule), ADR-0035 (`pathid`), ADR-0036 (the session's roots are read under a reach denial), ADR-0041 (a path is judged where it leads; probe leftovers), ADR-0042 (the approved cache) |

## Context

A session has two roots: the project and the session work directory. With the sandbox on, a tree outside both cannot be written at all, and the file tools refuse it even for a read. The only route is `--no-sandbox`, which gives up every protection for the whole session. An operator who keeps notes, a vault or a sibling checkout next to the project has nothing in between.

gem-agent closed this with a design that took three reviews and left a list of measured failures. This runtime shares the lanes, the file-read child, the verdict and the watch on persistent files, so it takes that design and states here what it must do differently.

## Decision

The behaviour is gem-agent ADR-0099's, section for section. In short:

1. **An added directory is a root with a level.** `read`: the file tools and the read lane read it, and no lane writes it, the operator lane included. `write`: as `read`, and the file tools and the write and operator lanes write it like the project. Credential filters, persistent-file names and `.git` writes hold in every root.
2. **Nothing in one is loaded.** No instructions, servers, policy or skills.
3. **The operator adds them.** `--add-dir` / `--read-dir` (repeatable), `[directories].write` / `.read` in the global config file, `/add-dir` / `/read-dir` at the prompt. `/dirs` lists the roots; a bare `/add-dir` or `/read-dir` does too. The project's `.lagent.toml` cannot add one: `LoadProject` already refuses the table as an unknown key.
4. **One rule decides.** `sandbox.RefuseRoot` refuses `/`, the home, anything holding the project, the work directory, the configuration home or the state directory, anything inside those two, credential locations, shared temporary directories, `/dev`, and, for write, the configuration file in use and `~/Library` except a synced folder (strictly inside `~/Library/CloudStorage`, or iCloud Drive). It judges the kernel's name for the path. Refused at startup, it stops the start.
5. **Every consumer of the root set learns the added roots**: the registry (each root has its own `os.Root`, a write to a read root is refused there whatever the verdict said), the file-read child, `risk.Classify`, the risk reviewer's payload and rules, `@` mentions, `Agent.intoRoots`, the profile `Spec` and its rebuild at `/clear`, and every sentence that names the roots.
6. **Profile order is part of the rule.** The read re-allow of an added root comes before `writeCredentialBlock`, which stays the last read rule. Each added root gets its own probes under the real profiles, at startup and on every grant.
7. **A write root's persistent files are watched and reported**, with `trustpin.Snapshot`, at startup, `/clear` and exit. Nothing blocks.
8. **One grant path.** The operator's commands and the model's request both call one function: judge, open and read the real path back, take the grant lock (shared with `/clear`), build and probe, snapshot, swap, record. A failed verification refuses the grant and leaves the session as it was: a grant never changes what the runtime established about the sandbox. A typed path that resolves through a link adds nothing and names its real path. A grant lasts for the process and is written to no file.

### What differs here

1. **The model learns the roots from the facts message, not the system prompt.** ADR-0003 keeps the system prompt byte-identical across sessions, because a change anywhere in it re-processes the whole prefix on a local server. `sessionFacts` carries one line per added root with its level, and says instruction files there are data. The system prompt does not change.
2. **A grant is announced by a fresh facts message at the next turn boundary.** `AnnounceSession` appends, as `/mcp reload` already does, so the earlier prefix stays cached and the compactor carries the message as it does any facts message. `AnnounceSession` is a between-turns operation, so a grant only marks the facts stale; a request's tool result tells the model at once. A grant therefore costs one more facts message, and only then.
3. **`request_directory` is kept, for the reason ADR-0008 gives.** A local model refused a path outside the roots today ends in prose. The tool gives it a route the operator always answers: `OperatorOnly` in every mode, unanswerable by the session allowlist, a policy or the risk reviewer, not registered under `-p`, not carried by a `Subset`. Its schema is a fixed addition to the tool list in interactive sessions.
4. **It ships as one change.** gem-agent shipped startup and mid-session grants in two releases and fixed the second at review. Both land here together, with those fixes in place.
5. **The names are this runtime's.** Configuration home `~/.config/lagent`, state through `statedir`, probe directories `.lagent-root-probe-*`, watch state `added-persistent.json` beside `persistent.json`.
6. **The approved cache needs no rule of its own.** It lies under the state directory (ADR-0042), which `RefuseRoot` keeps out of every root, and its read-lane denial by name still follows every allow.
7. **Paths compare through `pathid` where they name the same place** (ADR-0035). The grant's own comparisons of two names the kernel gave stay byte-exact, as gem-agent's review requires: a case-only swap on a case-sensitive volume is a change.

### Build order

1. The sandbox: `AddedRoot`, the profile, `RefuseRoot`, `CanonicalDir`, the probes.
2. The consumers: registry, child, verdict, reviewer, mentions, `intoRoots`.
3. Startup: configuration, flags, facts, the watch.
4. The grant path, the slash commands and `request_directory`.

Each step is one commit with its tests. The tests gem-agent's ADR-0099 §12 lists are ported with the code.

## Consequences

- A vault, notes or a sibling tree can be read or written under the lanes without `--no-sandbox`.
- An approval to `write` a broad directory means later writes there are not asked about under auto-approval. That is the grant, and the prompt says so.
- **Residual: a write root can be given persistent files, and they are reported, not refused.** A write-lane command can build `d/AGENTS.md` in scratch and move `d` into the root; a later session started under it loads that as an ancestor file. The report reaches only the sessions of the project that added the root. Blocking it would mean denying every rename into the root. Do not add as a write root a directory above projects you open with an agent.
- **Residual: walks in an added root apply the project's ignore rules**, as they do in the work directory.
- **Residual: a write root may hold files other programs run** (`~/bin`, a shell's configuration). The operator saw the path; a list of such places would drift.
- **Residual, from gem-agent's review:** `os.OpenRoot` opens without `O_NONBLOCK`, so a root swapped for a FIFO between the judgement and the open would hang the grant; only a process outside every lane can make that swap.
- A grant spawns a few `sandbox-exec` probes.
- A resumed session starts from the flags and the configuration again; `/dirs` shows what is in force.

## Alternatives considered

- **State the roots in the system prompt, as gem-agent does.** Rejected: it breaks ADR-0003 for every session that uses the feature, and a mid-session grant would re-process the whole prefix.
- **Leave out `request_directory`, as the local counterpart of a cloud feature.** Rejected: the cost is one schema, and without it the model has no route but prose.
- **A short delta message instead of a full facts message after a grant.** Rejected: the compactor carries facts messages and would drop the delta, and the model would lose the root after a compaction.
- **Add only the slash commands.** Rejected: they are a thin entrance to the roots, the verdict and the probes, which are the feature.
