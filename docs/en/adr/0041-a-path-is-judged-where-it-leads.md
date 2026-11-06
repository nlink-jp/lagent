# ADR-0041: a path is judged where it leads, and a probe that cannot remove itself says so

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-06) — implemented; port verification applied |
| Date | 2026-11-06 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator (2026-11-06), after gem-agent found both defects in its own code: "lagent should have the same problem". Measured here (below) |
| Ports | gem-agent ADR-0108 (a path is judged where it leads) and ADR-0107 §4 (a probe that cannot remove itself says so), both released in gem-agent v0.99.0, at gem-agent `e42f89e` |
| Amends | the file tools' confinement (README, "Confinement"): a path argument is normalised before the call is judged; ADR-0036 (the project is read wherever it lies: its probes run in a project anywhere) |
| Relates to | ADR-0001 (gem-agent is the porting source, and nothing here tracks it afterwards: gem-agent's added directories, its ADR-0099, are not in lagent, so gem-agent ADR-0107 §1–§3 do not apply), ADR-0036 (the project is read wherever it lies) |

## Context

**A path through a link outside the roots.** The roots — the project
and the session work directory — are real paths. `Registry.resolvePath`
checks that the typed path lies *lexically* inside a root before it
resolves links, so a path typed through a link that lies outside every
root is refused before it is resolved. Measured on this machine
(2026-11-06), with links in a directory outside both roots:

| Path typed | `RealPath` | Rule tier (`write_file`'s verdict; `read_file` is read-only and Safe) | `write_file` | `read_file` |
|---|---|---|---|---|
| link → the project, existing file | refused | Block, "outside the project and session work directories" | refused, "path escapes the project directory" | refused |
| link → the project, new file | refused | Block | refused | refused |
| link → the work directory | refused | Block | refused | refused |
| link → outside both | refused | Block | refused | refused |

A project in cloud storage is the case that meets it: the operator
opens `~/Library/CloudStorage/Dropbox/proj` (the real path, which the
runtime resolves at startup) and types, or the model writes, the full
path through the link, `/Users/<you>/Dropbox/proj/…` (a path written
with `~` is not expanded by the file tools, here or after this ADR). The shell lanes are not affected; the kernel
resolves the link.

**A probe that cannot remove its files.** `VerifyReadLane` and
`VerifyWriteLane` create probe files in the project at every start and
remove them ignoring a failure (read in `internal/sandbox/lane.go`; not
measured here). A project in a shared drive that lets the operator
create but not delete would keep the `.lagent-lane-probe-*` file and
probe directory and sync them to everyone who shares the drive, with
nothing said.

gem-agent decided both, with an independent design review, an
implementation review and a pre-release review; this ADR takes those
decisions as they were released, not the drafts the reviews replaced.

## Decision

### 1. A path argument is rewritten once, where the call comes in

When a tool call comes in — before it is shown, judged, hooked, run or
audited — each path argument of a built-in file tool is looked at once:
`path` of `read_file`, `write_file`, `edit_file`, `view_image`,
`show_image`, `file_info`, `list_files`, `list_tree` and `search_files`,
`write_file`'s `from`, and every entry of `file_info`'s `paths`. No other
tool is touched. An argument that is absolute, lexically outside both
roots, not credential material by its spelling, and not in a mount tree
has its **parent** resolved; when that real parent joined with the typed
last element lies in a root, the argument is replaced by it. A root
named by its own link becomes that root. Anything else is left as typed
and refused later exactly as today.

The resolution walks the path one element at a time onto a prefix that
is already real, and judges that prefix against the mount trees
(`/Volumes`, `/Network`, `/net`, `/home`, also spelled under
`/System/Volumes/Data`, case-folded) before reading into it, so no share
is touched before approval through any spelling or chain of links.

The rewritten call is the call from then on; the model's own message in
the history keeps what it sent. The reasons are gem-agent ADR-0108's: the
verdict and the run read one string, so no link is consulted between
them; the parent only, so a link at the last element is still judged on
its real name and written by replacement; spelling-based floors keep the
spelling; a failure says nothing about where a link leads.

`@` references follow the same rule for an absolute reference.

### 2. A probe leftover is reported

The lane probes — their files in the project, under the home and in
`/private/tmp` — report any probe file they could not remove, by
path, as a startup note (and at `/clear`, which rebuilds the lanes). It
is a note for the operator; lagent has no mid-session grant whose error
the model would read.

### 3. Ported code

`sandbox.IntoRoot`, its walk and its tests come from gem-agent
`internal/sandbox/intoroot.go` at `e42f89e`, with the added roots taken
out; the source commit is recorded in the file's doc comment (ADR-0001).

## Consequences

- The file tools reach the project and the work directory through a
  link the operator or the model knows them by.
- No new reach: the rewrite only ever produces a path inside a root,
  which the file tools then confine as any typed real path.
- Accepted residuals, as in gem-agent ADR-0108: the parent's metadata is
  read before approval, outside the roots (a protected folder may make
  macOS ask); the resolution runs before approval without the turn's
  cancellation; `~` is not expanded.
- A project inside a mount tree (`/Volumes/…`) is not reached through a
  link: the walk stops at the mount tree before reading into it. Its real
  path works as before.

## Alternatives considered

- **Port nothing, since lagent has no added directories.** Rejected:
  both defects are in code lagent already has, and a project in cloud
  storage meets the first today.
- **Resolve in `resolvePath`, at each use.** Rejected in gem-agent's
  design review: the verdict and the run would resolve separately.

## References

- gem-agent ADR-0108, ADR-0107 §4; gem-agent `internal/sandbox/intoroot.go`,
  `internal/agent/decision.go` (`intoRoots`) at `e42f89e`
- `internal/tools/tools.go` (`resolvePath`), `internal/agent/agent.go`,
  `internal/mention/mention.go`, `internal/sandbox/lane.go`
