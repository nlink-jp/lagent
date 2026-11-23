# ADR-0055: the search opens each directory once — `search_files` reads its files side by side, and its output does not change

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-23) — implemented (see the addendum) |
| Date | 2026-11-23 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The operator, after gem-agent's `search_files` was made faster (gem-agent ADR-0125): "lagent needs this too" — measured first, and it does |
| Relates to | gem-agent ADR-0125 (the design this ports, its design review and its measurements), ADR-0001 (gem-agent is a source), ADR-0016 (the file-read child), ADR-0029 (every file not searched is counted; port of gem-agent ADR-0096), ADR-0034 (the round's allotment, applied to the output), ADR-0053 §1 (the four answers), gem-agent ADR-0065 (cancellation in process), CONVENTIONS ADR-029 (design before implementation) |

## Context

### What was measured (v0.32.1, 2026-11-23, Apple M2 Max)

lagent's `search_files` is gem-agent's before its ADR-0125, ported under ADR-0001. Its walk, its helpers (`openRead`, `openRegular`, `readDirIn`, `readFileCapped`, `readAllCapped`, `gitignoreReader`, `rootFor`, `rootAndLevel`) and `internal/ignore` are the same code. The differences are ADR numbers in comments and one in the output: lagent appends its footers after the output cap (the review after ADR-0015) and applies the round's allotment (ADR-0034).

The shipped binary, run as the parent runs it: `__file-read` under lagent's own file-read profile (`sandbox.FileReadProfile`), one request on stdin. Median of five warm runs.

| Tree | Literal pattern | `func \w+Registry` | `(?i)thought signature` |
|---|---|---|---|
| gem-agent (789 files scanned) | 148 ms | 147 ms | 294 ms |
| cli-series | 306 ms | 314 ms | 588 ms |

A `file_info` call through the same child takes 37 ms: the fixed cost of one call. These are gem-agent v0.108.2's figures to within the run-to-run spread, for the same reason. Each open resolves the file's path one directory at a time from the session root, and in the child that takes nearly all the time past the fixed cost. gem-agent ADR-0125's Context has the measurements of the parts: per-file opens, workers, and the regular expression by pattern shape.

## Decision

The three changes of gem-agent ADR-0125 §1–§3, as written there, with lagent's names. Each is defined by "same output, less time". For any tree that does not change during the call, the text `search_files` returns is byte-for-byte what v0.32.1 returns.

1. **Each directory is opened once, and its files are opened by one name** (gem-agent ADR-0125 §1).
   - The walk holds an `os.Root` for the directory it is in, opened from the one above it with `OpenRoot(name)`. The listing, the `.gitignore` and each file are reached through that handle.
   - Every directory, the start included, is opened as `OpenRoot(name + "/.")`, which resolves the name as a directory and refuses anything else at once, a FIFO included. So a FIFO or a regular file named as `path` gets today's answer. *(Corrected by the review before release; see the addendum.)*
   - A directory swapped for a FIFO between the listing and its open is refused the same way. The first version of this ADR accepted it as a residual; the review before release found that, in process, an interrupt did not end such a call.
2. **Files are read and matched side by side, and used in walk order** (gem-agent ADR-0125 §2).
   - One goroutine walks as now and emits every step the loop counted.
   - Up to `min(GOMAXPROCS, 8)` workers read and match the files.
   - The calling goroutine applies every count and every line in walk order and stops at the cap.
   - The stream ends with the walk's reason for ending, so a cut is never silent.
   - The call stops and waits for its goroutines before it returns.
3. **A pattern with a literal prefix checks each block of 1024 lines before its lines** (gem-agent ADR-0125 §3).
   - A pattern with `OpBeginText` or `OpEndText` is matched line by line, and so is a pattern without a literal prefix.
   - The context is consulted between blocks.

**lagent's footer is not touched.** The calling goroutine fills the same counters the loop filled. The code after the walk assembles the footers after the cap, applies the allotment and adds the ignore note, exactly as now.

**The design review is gem-agent's.** gem-agent ADR-0125 had one independent pass, whose six findings are part of the text ported here, and gem-agent's implementation measured what the design predicted. This port adds no design of its own. The review before release covers the port: one pass over the whole difference, and one over the fixes it leads to.

### What is expected

The gains gem-agent measured after its ADR-0125, since the code and the per-call cost are the same. For the gem-agent tree, 148 ms → about 70 ms, and `(?i)` 294 ms → about 90 ms. For cli-series, about 306 ms → about 115 ms.

## Development plan

The plan of gem-agent ADR-0125, one commit per step:

1. **The judge first**, on v0.32.1. It is the same scenarios and generated trees as gem-agent's `searchjudge_test.go`, with references produced by v0.32.1 from runs that repeat. lagent's footer order differs from gem-agent's, so the references are lagent's own, not gem-agent's.
2. §1 alone; the judge passes.
3. §3 alone; the judge passes, and fails when the anchor exclusion is removed.
4. §2; the judge passes at 1 and at 8 workers, with the race detector on. It fails when counts continue past the cap, and when the walk order changes.
5. The tests of gem-agent ADR-0125:
   - the cap under varied timing;
   - a cancel mid-read and a cancel at a directory, each leaving no goroutine;
   - no descriptor left;
   - a root rotated during a walk;
   - a credential file among the workers' files, through the built child under the real profile.
6. Measured again, with the same script and trees, under lagent's real profile. A figure that misses the expected gain by more than half is reported and nothing more is tuned.

## Alternatives considered

- **Do nothing in lagent.** The two runtimes would differ in a tool they share by design, for no reason either has. lagent runs local models whose rounds are shorter than Vertex's, so 100 ms is a larger share of a turn there.
- **Port only §1.** It is the timing-free half and needs no goroutines. It leaves `(?i)` at about 200 ms in the child, and the two runtimes would keep different search code that must be ported apart from now on.

## Addendum: the implementation (2026-11-23)

Implemented in the order of the plan, one commit per step: the judge, §1, §3, §2. Each change came from gem-agent's commit for the same step: `ceabbbe`, `32c94b9` and `061afe7`. ADR citations were renumbered to lagent's: ADR-0016 for the child, gem-agent's own numbers for what lagent did not renumber. The new files record their source commit.

### Measured again

The same script, trees and profile as the Context. Each figure is the median of five warm runs, both binaries measured in the same sitting.

| Tree | Pattern | v0.32.1 | Now |
|---|---|---|---|
| gem-agent | literal, not found | 146 ms | 71 ms |
| gem-agent | `func New` | 145 ms | 70 ms |
| gem-agent | `func \w+Registry` | 149 ms | 70 ms |
| gem-agent | `(?i)thought signature` | 297 ms | 93 ms |
| cli-series | literal, not found | 319 ms | 111 ms |
| cli-series | `(?i)thought signature` | 595 ms | 152 ms |

The fixed cost is unchanged (35–38 ms). Every figure is at the expected gain.

### What the judge was shown to catch

The breaks gem-agent used, put in and taken out again:

- §3 without the anchor exclusion: three scenarios failed.
- §2 counting past the cap: the cap scenarios failed at both worker counts.
- The walk taking directories before files: ten scenarios failed.
- A worker that does not release its directory handle: the descriptor test failed.

The references are v0.32.1's own. Sorted line by line, every one of the 25 holds the same lines as gem-agent's: only the order of the footers differs, as the Context says.

## Addendum: the review before release (2026-11-23)

One independent pass over the whole difference, and one over gem-agent's change it ports. Every finding of either applies to both, and both runtimes took the same fixes: see gem-agent ADR-0125's addendum "the review before release" for the five findings and what was done. In short:

- A directory swapped for a FIFO held the call past an interrupt, in process. Directories, the start included, are now opened through `name/.` (corrected in §1 above).
- A result waiting in walk order held its whole file; a worker now keeps only the clipped lines it prints.
- The random equivalence test now crosses block edges.
- Accepted, as recorded: descriptors grow with depth, and a walk finishes on the root it started with.

lagent's footer assembly after the cap, and its allotment, were checked by the review and are unchanged.

The findings have one cause, recorded with the full list in gem-agent ADR-0125's addendum ("why the design missed these"). The design restated only the old loop's output. The loop also held other properties, and moving its work onto goroutines changed them silently. Every property was listed and checked in both runtimes. The one that can return with a later edit — an open that waits — is held by the same test of the code's shape here: `TestFileToolOpensDoNotWait`.

The verification of the fixes (one pass, bounded to that table) found one missing row (nothing is opened past the cap), one overstated row (memory) and five gaps in the open test. Both runtimes took the same fixes, recorded in gem-agent ADR-0125's table and its section "the verification of the fixes".
