# ADR-0042: the approved lanes' toolchain cache lies outside everything that runs unasked

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-09) — implemented |
| Date | 2026-11-09 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Ports | gem-agent ADR-0109 (same defect, same placement) |
| Triggered by | An audit of gem-agent found the same directory layout here (2026-11-09): the approved lanes' toolchain cache sits inside the directory the read lane may write |
| Amends | ADR-0008 §1 (where `<name>-approved` lives) |
| Relates to | gem-agent ADR-0073 (the lanes), gem-agent ADR-0058 (the work directory is a file-tool root, and a write inside it is Safe in the rule tier), gem-agent ADR-0059 (retention is the operator's, through `workdirs`) |

## Context

ADR-0008 §1 separates the read lane's cache from the approved lanes' so that "a read-lane command steered by what it read cannot seed an object that an approved `make build` links". The two directories were `<scratch>/<name>` and `<scratch>/<name>-approved`. The difference exists only in an environment variable: the read lane's profile allows `file-write*` on the whole scratch subpath, so a command that names the `-approved` path writes there. Go does not verify a cache entry's content on read, so the rejected route of ADR-0008's Alternatives ("one cache directory for every lane") remained open.

The audit proposed moving the cache to a sibling of the scratch, inside the work directory. That closes the shell route and leaves another: the work directory is a root of the file tools, and `risk.Classify` rates a `write_file` inside it Safe, so under auto-approval a model steered by what it read writes into the sibling without asking. The class is "a directory an approved build trusts must be out of reach of everything that runs unasked". There are two unasked writers by default, the read lane and the file tools, and the directory must also not lie where another session's lanes write. With `[approval].model_tier = "shell"` under `--auto`, a write-lane command a judge model approved also runs without asking the operator; the write lane writes this directory by design, so that opt-in is outside this guarantee.

## Decision

1. **The approved lanes' caches live in the session's own directory beside its work directory**: `<state>/<project>/approved-cache/<session id>`, a sibling of `<state>/<project>/work/<session id>` (`workdir.ApprovedCachePath`), mode 0700. Each row of `[sandbox].scratch_caches` is a child of it, under the table's own name. The `-approved` suffix is gone: the location is the separation. The directory is no file-tool root, and only this session's profiles name it.
2. **The profile says so.** `Spec.ApprovedCacheRoot` is added to the write and operator lanes' writable set. The read lane's profile denies `file-write*` on it after its last write allow. At the default state root no allow but this session's own write and operator lanes reaches the directory; the denial by name is a second line. A session directory that has become a link is refused when it is next created, not followed: the write lane can replace it, and a resume would otherwise widen what the lane may write.
3. **The claim is measured at startup.** `VerifyReadLane` gains a probe that must fail: a write into the approved root. A machine where it succeeds runs with the read lane disabled, as for any other probe.
4. **Retention follows the work directory.** The cache is keyed by the session id, so `--continue` finds it warm. `lagent workdirs` counts it and `workdirs clean` removes it with the work directory (gem-agent ADR-0059); nothing removes it on its own.
5. **Where there is no such directory, the approved lanes get no redirection.** A session with no work directory (its state could not be created), or one whose directory cannot be made, does not set those lanes' cache variables (one the operator exported passes through). The read lane is unaffected and can build. A write-lane or operator-lane Go build then fails on the default cache, which neither lane may write, and says so.

## Consequences

- An entry planted in an old `<scratch>/<name>-approved` is not consumed: nothing reads that path any more.
- Residual, accepted: the guarantees above are those of the default state root. A state root the operator placed inside the project, in a temporary root (`/private/tmp`, `$TMPDIR`) or under `/private/var/tmp` puts the directory or an ancestor in the reach of a lane or the file tools: the read lane can rename a writable ancestor out from under a denial by name, and other sessions' lanes may write it. A project that contains the state root (the home directory, after the broad-root confirmation) does the same for the file tools. This is the existing class for the work directory and transcripts under such a root.
- Residual, accepted: only `work/` is enumerated, so a cache whose work directory was deleted by hand is left for the operator to delete, and `lagent workdirs` does not show it.

## Alternatives considered

- **A sibling of the scratch in the work directory** (the audit's proposal) — rejected: the file tools write there as Safe (see Context).
- **A per-process directory in the temporary root** — gem-agent's first implementation, rejected on independent review, measured under real profiles: with `TMPDIR` under `/private/var/tmp` the read lane could rename an ancestor and plant an entry, and could write a concurrent process's root (a denial names only its own); at the default `TMPDIR` every session's write lane could write every other session's live cache, a new cross-session edge; and a process that did not exit cleanly orphaned hundreds of megabytes that nothing claimed.
- **Mark the path protected in the rule tier** — rejected: a second list of a directory's meaning, and it would leave the shell route to the kernel profile anyway. Placing the directory outside every root needs no rule at all.
- **`GODEBUG=gocacheverify=1`** — rejected: Go-only, while the table is any toolchain's.

## References

- ADR-0008 §1 and Alternatives (the guarantee this restores)
- gem-agent ADR-0073 (lane profiles, last-match-wins)
