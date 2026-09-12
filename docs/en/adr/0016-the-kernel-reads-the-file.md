# ADR-0016: The kernel reads the file — the credential list stops being a matcher the file tools carry

| Field | Value |
|-------|-------|
| Status | **Proposed** (2026-09-13) |
| Date | 2026-09-13 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator, reading the v0.4.0 release and gem-agent's v0.78.0 alongside it: "this credential-file and environment-variable protection looks like it is walking the same road as the command-safety evaluation we withdrew — combinations grow, and every hole found is answered with another pattern." Then: "putting the file tools' reads through the kernel too is the simplest." |
| Amends | ADR-0015 (§2 is withdrawn: names are not withheld, and the enumeration tools carry no credential code) |
| Relates to | gem-agent ADR-0073 (the lanes, and the domain argument this applies), gem-agent ADR-0072 §4 (`os.Root` confinement, unchanged), ADR-0001 (the porting source), ADR-0008 (a skip names the route), ADR-0010 (no model tier: the operator is the only judge), ADR-0017 (the same question where no kernel exists) |

## Context

gem-agent ADR-0073, which lagent's lanes are ported from, withdrew the
Safe derivation from shell command text on a domain argument: a command
string is an unbounded domain, the kernel is a bounded one, so the
judgment moves to the kernel and the text rules survive only as a floor
that can raise a verdict and never lower one. The thirteen
`blockPatterns` that remain are affordable because a miss there costs
one prompt the cage would have caught anyway.

The file tools never made that move, in either runtime. They do not
pass through Seatbelt: they are in-process Go code opening through
`os.Root`, and the only thing between `read_file .env` and the model is
`sandbox.CredentialPath`, a matcher written in Go. A miss there is not
a missing prompt. It is the file's content in the transcript, and from
there in whatever the model is.

ADR-0015 gave that matcher a second job — deciding which entries the
enumeration tools withhold — and the operator's observation is that the
job keeps growing. gem-agent's history, the same code, says it plainly:
since the lanes landed, its credential list gained one entry while the
matching rule changed six times, four of them in a single release, each
from an independent review finding and each a spelling the previous
rule had not considered. lagent is not ahead of that curve; it is
behind it. Its walks still judge the project-relative spelling and do
not resolve their root, which is the defect gem-agent's v0.78.0 review
found and patched.

The reason is the same in both: **the set of ways to spell a path to a
known file is unbounded.** The list of files is finite and is not the
problem.

### What the kernel actually does

Measured 2026-09-13, with a profile carrying only `(allow default)`,
`(deny file-write*)`, `(deny network*)`, the credential deny and the
`.env` template re-allow:

| Operation on `.env` | Result |
|---|---|
| `cat .env` | `Operation not permitted` |
| `stat .env` | `Operation not permitted` |
| `cat .env.example` | allowed — the template re-allow holds |
| `grep -r` over the directory | refuses that one file, continues |
| `ls -a` | **the name `.env` is listed** |

The kernel is a complete boundary for content and for metadata, and it
is not a boundary for names. Both halves decide something below.

Spawn cost on the same machine, 10 runs each:

| What | Cost per run |
|---|---|
| bare `/usr/bin/true` | 1.8 ms |
| `sandbox-exec` + `/usr/bin/true` | 7.8 ms |
| `sandbox-exec` + this binary | 18.8 ms |

A file-tool call already costs a model round, and on a local model that
round is measured in seconds. 19 ms is not a budget worth designing
around.

## Decision

### 1. A read the model asked for happens in a sandboxed child

The registry's read primitives run in a child wrapped by `sandbox-exec`
under a purpose-built profile: one spawn per tool call, this binary
re-executed with an internal subcommand, the request on argv and the
bytes on stdout, bounded as they already are.

The profile is the base body plus `(deny network*)`, plus
`sandbox.CredentialFilters(home)` as `(deny file-read* …)` with the
`.env.example` family re-allowed after it — built from the same
`internal/sandbox` list every other enforcer reads.

Project confinement stays in Go. The kernel does not know where the
project is, and `os.Root` already refuses an escape at the syscall
(gem-agent ADR-0072 §4). The kernel gets exactly one job: **in this
process, credential material cannot be opened.**

Covered: `read_file`, `view_image`, `file_info`, and the whole
`search_files` walk, which runs inside the child so every open it
performs is adjudicated.

### 2. The refusal is the operator's question, and the matcher stops being the boundary

A denied open returns a typed error. The agent turns it into the
operator-only prompt ADR-0015 §1 defined, and on approval the read is
re-issued **in process** — the operator lane's authority applied to a
file tool, since the operator is the only party who may read
credentials and has just said so. Nothing reaches the model before the
gate, because a denied open produces no bytes.

`risk.credentialRead` is no longer required for correctness. It stays
only to raise the prompt without spending a spawn in the common case. A
miss in it now costs a spawn and a less direct prompt, never a leak —
the property gem-agent ADR-0073 gave the shell lanes, now given to the
file tools.

### 3. Names are not withheld — ADR-0015 §2 is withdrawn

The kernel lists the name and refuses the content. A file name is not
the secret; the content is. ADR-0015 §2 reasoned that a listing hiding
`.env` without saying so would be a false "not here", and answered it
by reporting the count and up to five names. gem-agent's ADR-0085 §2
reasoned the opposite way from the same premise and reported a count
with no names. Neither survives: with the content unreadable there is
nothing left to withhold, and both runtimes list the entry like any
other and refuse to read it.

So `list_files` and `list_tree` carry no credential code at all, and
`credentialTally` is deleted. `search_files` names the file it could
not read and keeps going, which is the shape `grep -r` already has and
the shape gem-agent ADR-0052 asks for.

This also dissolves rather than patches the walk defect above: with the
opens adjudicated by the kernel and the names no longer judged, there
is no spelling left for a walk to get wrong.

### 4. The ceiling is written down, not chased

Three things this design does not do, recorded so a future review
reports them as known rather than as holes to close with one more
pattern:

- **A hard link to a credential file under an ordinary name.** There is
  no real path to canonicalise — the link *is* a real name.
- **A copy under an ordinary name.** The same.
- **A secret inside a file the list does not name.** Content inspection
  is the unbounded domain this ADR exists to avoid, and ADR-0015
  rejected it already.

The residue ADR-0015 recorded as accepted rather than scheduled — the
read lane reading the configuration file and the transcripts — is
unchanged by this ADR and stays accepted.

### 5. Degradation is measured, like the lanes

At startup the child is verified the way the read lane is: a probe file
the profile must refuse, and an ordinary file it must read. On failure
the file tools fall back to in-process reads with `risk.credentialRead`
as the boundary, which is today's behaviour, and the note says so. A
degraded state that claimed the kernel was watching would be worse than
the matcher.

### 6. The criterion this leaves behind

**An entry is cheap; a rule is the smell.** Adding a path to
`internal/sandbox` is the mechanism working as designed. Adding a rule
to how paths are compared is the signal that the judgment sits in the
wrong domain. Three rule changes in a row is not a matcher needing a
fourth fix; it is a boundary in the wrong place.

## Consequences

- The credential matcher stops being load-bearing for reads. What
  remains of it: building the profile, the shell Block floor, and the
  write tools' Block.
- Deleted: `credentialTally`, the credential branches of `list_files`,
  `list_tree` and `search_files`, and ADR-0015 §2.
- Added: one profile function, one internal subcommand, one typed
  error, one retry branch, one startup probe.
- Each covered call costs about 19 ms more, against a local-model round
  measured in seconds.
- `search_files` becomes more useful: it reports the file it could not
  read instead of pretending it was not there.
- **Not covered: writes.** `write_file` to a credential path stays a
  Go-side Block. A write destroys a secret rather than publishing it,
  and every write is gated already. The symmetric move is available and
  is not taken here.
- gem-agent takes the same decision as its ADR-0086; the two runtimes
  stay one design, which is the point of ADR-0001.

## Alternatives considered

- **A long-lived helper process.** Rejected: it saves 19 ms per call
  and costs a protocol, a lifecycle, a restart path coupled to
  `/clear`'s work-directory rotation, and a new failure class where the
  helper dies mid-session.
- **Sandboxing the agent process itself.** Not possible: the profile
  would have to allow everything the agent legitimately does, and
  `sandbox_init` is irreversible and deprecated.
- **Kernel-enforced project confinement too.** Rejected: the child must
  read its own executable and the system libraries, so a global read
  deny becomes a list of system re-allows, which is a new unbounded
  list. `os.Root` already does confinement correctly.
- **Keeping the matcher as the boundary with the kernel as a second
  layer.** Rejected: two boundaries for one rule is what "one list, N
  enforcers" exists to avoid, and it leaves the matcher load-bearing,
  which is the complaint.
- **Patching the walks' root resolution, as gem-agent v0.78.0 did.**
  Correct for the shipped design and superseded by this one: the code
  that fix repairs is deleted here.
