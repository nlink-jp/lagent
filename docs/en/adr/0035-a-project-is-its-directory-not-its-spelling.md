# ADR-0035: a project is its directory, not its spelling

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-01) — implemented; §2 revised before release (pre-release review and the operator: compare normalised strings, not file identity) |
| Date | 2026-11-01 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Found while measuring ADR-0034: `--resume` refused a session in the directory it was recorded in, naming two paths that print identically |
| Relates to | ADR-0034 (the measurement that found it) |

## Context

lagent keys everything it keeps per project by the project's path as a
string: the transcript's `project` field (resume and `lagent sessions`
filter on it), the state directory's `.project` marker, and the
`[projects."<path>"]` tables of `policy.toml` — trust, content pins,
per-project tool and command allowlists.

That string came from `os.Getwd`, which returns `$PWD` whenever `$PWD`
names the current directory. A path with Japanese voiced kana has two
Unicode spellings: precomposed (NFC, what an input method types) and
decomposed (NFD, what this cloud-storage provider created on disk).
macOS treats them as the same name. Measured on APFS with a throwaway
directory created in each spelling and entered in each:

| | entered as NFC | entered as NFD |
|---|---|---|
| `stat` | finds it | finds it |
| `os.Getwd` (follows `$PWD`) | NFC | NFD |
| `syscall.Getwd` (the kernel) | the spelling on disk | the spelling on disk |
| `filepath.EvalSymlinks` | keeps the given spelling | keeps the given spelling |
| a Seatbelt `subpath` rule in the other spelling | matches | matches |

So the same directory produced two project strings, depending on how
the operator's shell got there. The record from the failed session
carried NFD (121 bytes); a later `cd` typed in NFC produced 115 bytes:

- `--resume` refused: "session … belongs to <path>, not <path>".
- The state directory's marker refused the second spelling as a
  path-escape collision, so the session ran without its work directory.
- Trust, pins and allowlists recorded under one spelling are not found
  under the other: the project asks for trust again and the operator's
  earlier decisions are silently absent.

The sandbox is not affected: Seatbelt matches across spellings.

## Decision

1. **The project path is taken from the kernel.** Every command that
   resolves the current project (`lagent`, `sessions`, `trust`,
   `workdirs`) takes the working directory from the kernel
   (`syscall.Getwd`), not from `$PWD`, then resolves symlinks as before.
   The spelling is the one on disk, so it is the same however the shell
   arrived. An architecture test refuses `os.Getwd` anywhere in the
   module's code, so no second reader brings `$PWD` back.

2. **Paths are compared by one function, in the volume's sense of a
   name.** Every comparison of two paths — a recorded project with the
   current one (the transcript's `project` for resume and the listing,
   the state directory's marker, the `policy.toml` project tables,
   `[approval].trusted_projects`), and every containment check (is this
   path inside the project, the work directory, a skill, a protected
   directory) with the path relative to it — goes through
   `internal/pathid`: both sides cleaned and Unicode-normalised to NFC,
   then compared. It is a pure function of the strings. Six packages had
   their own copy of the raw prefix check; they now call `pathid.Within`,
   and an architecture test refuses a raw `HasPrefix(p, base+separator)`
   anywhere else. Records written before this change, in either
   spelling, are found; nothing on disk is rewritten.

3. **Case is not folded.** On a case-sensitive volume two names that
   differ in case are two directories; folding them would hand one's
   trust and allowlists to the other. Unicode form is different: APFS
   and HFS+ never hold two names that differ only in it.

4. **Writes go to the entry that already exists.** A `policy.toml`
   change for the current project — clearing included — lands in the
   table that already names this directory, whatever its spelling, so one
   directory never grows two tables.

## Consequences

- Resume, the state directory, trust (recorded or configured),
  allowlists and the containment checks agree for a directory however
  its path was typed or completed — including an absolute path the
  model or the operator types in the other form.
- The comparison touches no disk: it cannot stall on a volume that no
  longer answers, and cannot follow a recorded path that has since
  become a symlink to another directory.
- `golang.org/x/text` moves from an indirect to a direct dependency (the
  Go project's own module, already in the build).
- Not covered, and accepted:
  - **Two tables written before this change**, one per spelling (an
    operator who answered the trust prompt again under the other
    spelling): the one under the exact current spelling is used, else
    the first in sorted order; the other is not merged. What it held is
    asked for again — the failure is towards asking, not towards
    permitting.
  - **Case-only differences** between a record and the kernel's spelling
    (an old record made from a `cd` typed in the wrong case on a
    case-insensitive volume) do not match; the project asks again.
  - **A record made through a firmlink spelling**
    (`/System/Volumes/Data/...`) escapes to a different state-directory
    name than the kernel's `/Users/...`; its memory and work directory
    start empty. Resume by id still works.

## Alternatives considered

- **Match by file identity (`os.SameFile`).** Implemented first and
  replaced before release: it needs both paths to exist, cannot answer
  containment, stats every recorded project on the hot path (a stale
  network volume stalls each write), and follows a recorded path that
  has become a symlink, so the link's target inherited the record's
  trust (pre-release review).
- **Fold case as well.** Rejected (§3).
- **Rewrite existing records to the kernel's spelling on load.**
  Rejected: it rewrites the operator's files to fix a reading problem,
  and a transcript is a record of what happened.
- **Only the kernel spelling, no normalised comparison.** Rejected:
  records already written under a `$PWD` spelling would stop matching
  after the upgrade — trust asked again, allowlists gone, sessions
  unresumable — and typed absolute paths would fall outside the project.

## References

- `cmd/project.go`, `internal/pathid`, `internal/statedir`,
  `internal/session`, `internal/config/policyfile.go`,
  `internal/config/config.go`
