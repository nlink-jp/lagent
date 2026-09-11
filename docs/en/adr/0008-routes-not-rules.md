# ADR-0008: The runtime supplies routes, not rules — the local-oriented prompt revision

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-12 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | A bench run in which the model explained a fix in prose and never made it — and the transcript showed each step of the runtime's own hand in that |

## Context

The RFP's Phase 2 lists "a local-oriented prompt revision", decided
after measurement. The bench (ADR-0006) supplied the measurement: one
`read-edit` run in eight ended with the model describing the corrected
loop in a code block and no `edit_file` call. The transcript, read
backwards, is a chain the runtime built:

1. The prompt says "after making changes, verify them (run tests or
   the build via shell_exec with access: write)".
2. The model ran `go run .` in the read lane first. It failed: Go's
   build cache lives under `~/Library/Caches/go-build`, and the sandbox
   profile denies every write outside the project, the work directory
   and the scratch — in both lanes, on the operator's real machine
   (probed 2026-09-12: read and write lane alike). Building with a cold
   cache is impossible inside the sandbox today, and the write-lane
   note then points at the operator lane, which is not the answer
   either.
3. The model re-ran it in the write lane. The run is one-shot with
   `--auto`; a write-lane shell is Review, and with nobody to ask it
   was denied. The denial text is gem-agent's: "ask the user how to
   proceed instead".
4. There is no user in a one-shot run. The model did the nearest
   thing: it stopped acting and wrote what it would have done.

Every step follows a sentence the runtime wrote. The prompt asked for
a verification the sandbox cannot run; the sandbox failed it for a
reason unrelated to the task; the gate denied the escalation; the
denial told the model to consult someone absent. The fix is not a rule
telling the model to edit before it explains — the knowledge base
records what standing prose rules do to a local model (they are
skipped, and prohibitions breed third paths). The fix is to make each
of those sentences true.

Two smaller costs came out of the same transcripts. Every run opened
with `list_tree dirs_only=true`, was told "(empty directory)" for a
directory that had two files and no subdirectory, and spent a round on
`list_files` to learn otherwise. And the prompt's lane paragraph says
build and test tools need the write lane "because they write their
caches" — which was true only because the runtime gave them nowhere
else to write.

## Decision

Four changes, each replacing a sentence the model could not act on with
a fact it can, measured together on the bench.

1. **Toolchain caches ride the session scratch.** Every `shell_exec`,
   in every lane, runs with `GOCACHE` pointing at a `go-build` directory
   under the session's private scratch (the read lane's writable
   directory, which the write lane may write too). Builds, vets and
   tests then run in the read lane without approval, as inspection
   does; the cache is cold once per session and warm after. It is not
   the operator's shared cache on purpose: the read lane runs unasked,
   and a shared content-addressed cache is a place an unasked command
   could plant an object a later build outside the sandbox would trust.
   The list of cache variables is one function (`toolchainCacheEnv`)
   with Go alone in it; another toolchain joins when a measurement
   shows the same failure.
2. **An unattended denial names the route.** In a one-shot run the
   agent knows it is unattended (`Options.Unattended`), and a denied
   call's result says so: nothing that needs approval can run here;
   continue with what needs none — the file tools inside the project,
   the read-lane shell — or finish and state what remains undone. The
   interactive text keeps "ask the user".
3. **`list_tree` says what it saw.** With `dirs_only`, a directory that
   has files but no subdirectories lists those files (up to the
   per-directory cap) instead of reading as empty. A first cut reported
   the count and pointed at `list_files`; measured, the model followed
   the pointer every time and the round was spent anyway — a route the
   tool can walk itself is not a route to name.
4. **The prompt says what is now true and drops the rules the runtime
   replaced.** The lane paragraph: the read lane runs inspection *and*
   compiling, vetting and testing (the toolchain cache lives in the
   lane's scratch; a build that writes its binary into the project
   still needs the write lane — probed: `go build` of a single main
   package does); the write lane is for changing files, installing,
   committing and the network. The verification bullet names the read
   lane. The bullet "a denial is a decision, not an obstacle — ask how
   to proceed" goes: the denial result carries the route now. The
   `write_file` bullet is said once, affirmatively. `shell_exec`'s own
   description follows the same wording.

Nothing else in the prompt moves. The security framing stays first
and unchanged; the prompt stays byte-identical across sessions
(ADR-0003).

## Consequences

- Measured on the bench, three repetitions, before → after: recorded
  in `reference/bench.md` under this record's date. The expected
  effects, written before the run: `read-edit` completes 3/3 and its
  verification step succeeds in the read lane; every task loses the
  `list_files` round after `list_tree`; no run ends in prose after a
  denial.
- A Go build inside the sandbox compiles cold once per session. On a
  large project that is the price of building at all; the operator's
  own cache is untouched.
- Module downloads are not covered: `~/go/pkg/mod` is readable and not
  writable in any lane, so a project whose modules are not already
  cached cannot fetch them from inside a run. Recorded as a gap, not
  solved here.
- gem-agent's profile has the same cache failure (probed on the same
  machine). A proposal to the source is deferred, as the operator
  directed; this record is the evidence for it.

## Alternatives considered

- **Tell the model to edit before explaining, or to act without
  asking in one-shot.** Rejected: a standing prose rule against a
  behaviour the runtime itself provoked; the knowledge base's measured
  outcome for such rules is that they are skipped or over-generalized.
- **Allow writes to `~/Library/Caches` in the lanes.** Rejected for the
  read lane (an unasked command writing a shared content-addressed
  cache is a poisoning route) and therefore for the write lane too,
  since the point is that a build works in the read lane; one cache
  location for both lanes keeps the compile warm across a session.
- **Auto-approve write-lane builds in one-shot.** Rejected: the write
  lane reaches the network and the project; "build" is a word in a
  command line, not a lane, and the rule tier does not read command
  text for intent (gem-agent ADR-0073).
- **Make `list_files` the first tool instead.** Rejected: the prompt's
  orientation advice is fine; the tool's report was false.
