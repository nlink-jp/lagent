# ADR-0055: the search opens each directory once — `search_files` reads its files side by side, and its output does not change

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-23) |
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
   - The start of the walk is first opened the way the file tools open, which does not wait. Only a directory is then opened as a root, so a FIFO or a regular file named as `path` gets today's answer.
   - The residual is a directory swapped for a FIFO between the listing and its `OpenRoot`. It is judged on ADR-0053 §1's four answers as gem-agent ADR-0125 §1 judges it: it needs a program written to race the runtime (origin D), and it gives that program a search that waits until the operator interrupts.
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
