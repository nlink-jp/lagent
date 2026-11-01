# ADR-0035: a project is its directory, not its spelling

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-01) — implemented |
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

2. **A recorded project path matches the current one by identity.**
   Wherever a recorded path is compared with the current project — the
   transcript's `project` (resume, the session listing), the state
   directory's marker, the `policy.toml` project tables — the two match
   when the strings are equal or, failing that, when both exist and are
   the same directory (`os.SameFile`). One function does this, used by
   all of them. Records written before this change, in either spelling,
   are found; nothing on disk is rewritten.

3. **Writes go to the entry that already exists.** A `policy.toml`
   change for the current project lands in the table that already names
   this directory, whatever its spelling, so one directory never grows
   two tables.

## Consequences

- Resume, the state directory, trust and allowlists agree for a
  directory however its path was typed or completed.
- Identity also matches spellings that differ only in case on a
  case-insensitive volume, which is the same directory there too.
- A recorded path whose directory no longer exists matches only by
  string, as before.
- The path-escape collision check still separates different
  directories: two projects whose paths escape to the same state
  directory name are different files, so they do not match.
- A `stat` of each recorded `policy.toml` project is paid only when the
  exact string is absent.

## Alternatives considered

- **Normalise every path to NFC (or NFD) as a string.** Rejected: it
  needs a Unicode normalisation table (`golang.org/x/text/unicode/norm`
  as a direct dependency), still disagrees with a volume that is
  case-insensitive, and either form disagrees with the spelling on disk
  for half the directories, so a path shown to the operator or written
  into a sandbox profile would not be the one the system reports.
- **Rewrite existing records to the canonical spelling on load.**
  Rejected: it rewrites the operator's files to fix a reading problem,
  and a transcript is a record of what happened.
- **Only the kernel spelling, no identity match.** Rejected: records
  already written under a `$PWD` spelling would stop matching after the
  upgrade — trust asked again, allowlists gone, sessions unresumable.

## References

- `cmd/project.go`, `internal/pathid`, `internal/statedir`,
  `internal/session`, `internal/config/policyfile.go`
