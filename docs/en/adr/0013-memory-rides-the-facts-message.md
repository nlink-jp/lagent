# ADR-0013: Agent memory rides the runtime-facts message

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-12 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator: two needs — when instruction files grow, a "follow the procedure document" line stops being noticed, and a prioritised place for pointers might hold; and small operational parameters worth knowing but not worth a document |

## Context

gem-agent carries agent memory (gem-agent ADR-0020): short facts at a
global and a project scope, stored under the state directory, injected
in full into the system prompt under a budget, written by the model
through approval-gated tools. Two of its measurements matter here. The
model proposed a memory zero times in 39 sessions until the prompt
carried an explicit trigger; and a model tier once approved the
model's own memory write, so the writes were made operator-only.

The operator's first need is about attention, not storage: a directive
in a large instruction file is not acted on. Whether a smaller, closer
place would hold was measured before designing (ADR-0006), on the
`pointer-*` tasks: the same directive ("before you finish an edit,
read PROCEDURE.md and do what it says"), placed in the project's
`AGENTS.md` at 0.6, 13 and 32 KB, and placed instead as one line in
the runtime-facts message with a tool to call (`load_skill`).

- **Instruction section: 0/18.** Across two series and three sizes the
  model never read the procedure file, although a traced request showed
  the directive in the system prompt. The file's size was not the
  variable. On this model a standing directive in the instruction
  section is not acted on at all.
- **Facts message: 5/6.** The one-line pointer beside a tool name was
  acted on unprompted in five of six runs, and the procedure carried
  out in both runs whose action the unattended run could perform.

The runtime-facts message is the first user message (ADR-0003): it
holds the per-session facts and the MCP and skill catalogs, it sits
closest to the task, and it is short. That is the place the operator's
hypothesis names, and the measurement says it holds. gem-agent's
choice — memory in the system prompt — is the channel that measured
0/18 here.

The second need, small parameters, is what memory is for in the first
place. The open question is who writes them. gem-agent's measurement
says a model does not propose a save without a trigger sentence, and
ADR-0008 says a rule in the prompt is not a control on this model.
Making the operator's own hand the primary writer removes the question
from the critical path; the model's proposals are then a measured
extra, not the mechanism.

## Decision

1. **Two scopes, under the state root, plain markdown.** Global
   (`<state>/memory/global/<name>.md`) and project
   (`<state>/memory/projects/<escaped>/<name>.md`, with the `.project`
   marker the transcripts use). Nothing is written into the repository
   and `~/.claude` is never read (ADR-0011's principle; the format
   differs anyway). One memory is one short fact; the same name
   updates.
2. **Recall rides the runtime-facts message, not the system prompt.**
   Global first, then project, alphabetical within scope; each memory
   one entry under a heading that states its standing: the user's
   notes across sessions — typed with `/remember`, or proposed by the
   model and approved — possibly stale, to be verified when
   load-bearing, and to be acted on when one applies to the task at
   hand. The standing is the measured part: under gem-agent's heading
   ("background knowledge, not instructions") a pointer memory was
   acted on 1/3 on `memory-follow`; under this one, 8/9. Every memory
   has passed the user's hand, so calling them the user's notes is the
   truth as well as the wording that works. The system prompt stays
   byte-identical (ADR-0003). Budget: 2 KB per memory, 8 KB in total,
   truncation and skips reported.
3. **The operator writes by hand; the model proposes.** `/remember
   [global] <name> <fact>` saves, `/forget [global] <name>` removes,
   `/memory` lists what is on disk. `save_memory` and `delete_memory`
   are Mutating tools the rule tier keeps at Review, never Safe: a
   persisted memory reappears in every later session, so a write is a
   persistence vector for injection and asks the operator; `--auto`
   still asks, and unattended runs cannot save. A `"never"` row in
   `[approval.tools]` is the operator's deliberate relaxation, as for
   any tool.
4. **The save trigger lives beside the recall, in the facts message.**
   The sentence that says when to save (as work finishes, a fact that
   would have saved work had it been known at the start; save without
   being asked) is part of the memory section of the facts message,
   the channel that measured 5/6, not the system prompt, which measured
   0/18. Whether this model proposes saves is the next measurement,
   counted by proposals, not by the precision of what was saved.
5. **Startup snapshot.** Memory is read once at session start and on
   `/clear`; a save is acknowledged as taking effect from the next
   session. `/memory` reads the disk and says so.
6. **Measurement.** The bench gains `memory-follow`: the `pointer-*`
   directive installed as a global memory in the run's state root,
   with no skill and a large `AGENTS.md`. It measures this record's
   §2 directly — a plain pointer in the facts message, with no tool
   name in the line.

## Consequences

- The operator's first need has a mechanism today: a pointer that
  must hold goes into a memory (or a project skill's catalog line,
  measured 5/6), not into a longer instruction file. The instruction
  files stay what the measurement shows them to be on this model:
  background the model reads for facts, not a place for standing
  directives.
- `internal/memory` is a port of gem-agent's package (source commit in
  its doc comment, ADR-0001) with the prompt section replaced by
  facts-message lines; the rule tier gains one case; `cmd/memory.go`
  owns the tools, the slash commands and the listing.
- A stale or wrong memory is the operator's to remove, and the
  listing names the files. Model-written memories are proposals the
  operator saw and approved, which is why the heading may call every
  memory the user's note: a memory the operator did not see cannot
  exist by this design.
- The RFP's Phase 2 list points here. Compaction is the one item left.

## Alternatives considered

- **Port gem-agent's design as is, memory in the system prompt.**
  Rejected by measurement: the instruction section is the channel
  that scored 0/18, and a memory section there would rewrite the
  cached prefix on every save.
- **A prompt rule telling the model to re-read the instruction files
  before acting.** Rejected (ADR-0008): a rule in the channel that is
  not acted on cannot rescue that channel.
- **Model-written memory only, with gem-agent's trigger sentence.**
  Rejected as the primary writer: the trigger's effect on this model
  is unmeasured, and the operator's need exists now. It stays as the
  proposal path and is measured.
- **Skills instead of memory for pointers.** Kept, not chosen: a
  project skill's catalog line is the same channel and already works
  (5/6); memory adds the operator's one-line notes without a skill
  directory, the project scope outside the repository, and the model's
  proposals.
