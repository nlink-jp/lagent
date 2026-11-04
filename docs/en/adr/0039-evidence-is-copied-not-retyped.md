# ADR-0039: every MCP result is saved, and evidence is copied from the saved file — never re-typed

| Field | Value |
|-------|-------|
| Status | **Proposed** (2026-11-04) |
| Date | 2026-11-04 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The operator's report (2026-11-04), made for both runtimes: DWH query results kept as evidence lost rows, and evidence appeared that the database never returned |
| Ports | gem-agent ADR-0105 (Accepted 2026-11-04, released in gem-agent v0.98.0), at gem-agent `edd8e60` |
| Relates to | ADR-0016 (the kernel reads the file; credential retry), ADR-0015 (credential reads are operator-only), ADR-0034 (one round is bounded by the window), ADR-0008 (routes, not rules), ADR-0006 (measurement bench), ADR-0001 (porting sources pinned) |

## Context

### What was reported, and what lagent does

The operator collects data from databases and the DWH through MCP servers and keeps what was returned as evidence. lagent's MCP intake (`cmd/mcpresult.go`) is gem-agent's as it was before gem-agent ADR-0105: a text block that fits the response budget is handed to the model inline and written nowhere; a larger one is saved in the session work directory and previewed by head and tail. Under ADR-0034 the budget is the round's allotment, which is often smaller than gem-agent's 20,000 bytes, so more results are spilled — but the inline ones still exist only in context.

To keep a result as evidence the model has two routes, and they are approved differently:

- `write_file` inside the project or the work directory is **Safe**, approved at once.
- A write-lane `shell_exec` (`cp`) goes through ADR-0032's model tier and, failing that, to the operator. In `-p` with nobody to ask, it is refused.

### What was measured (2026-11-04)

gem-agent's `tools/evidenceprobe` runs the real binary one-shot against a stub that presents as the fleet's bigquery-mcp and really executes the queries against a seeded table (see gem-agent ADR-0105 for the method). lagent, `ddalcu/Qwen3.6-35B-A3B-MLX-Serve-4bit` on mlx-serve:

- **Small result, inline (s1).** Three runs (one pilot, two counted): every one re-typed the evidence with `write_file`. The values happened to be right.
- **Large result, saved by the runtime (s3).** 10 valid runs, the result spilled in every one: **0 copied**; 8 re-typed with `write_file`, 6 of them wrong (a value taken from another row, a row dropped, a SHA-256 lost); 2 saved nothing, one of them after its `cp` was refused (the gate decision's source was the operator, absent in `-p`).

A saved file alone produced no copy, and the one attempt to copy took the route that needed approval. The operator decided that re-typing is the problem itself, not its error rate: a file written out from context has no provenance even when every value is right.

gem-agent ADR-0105 fixed this by saving every result, recording what was saved where no server can write, and giving `write_file` a byte-for-byte copy at the same tier as writing. Measured there, 20 of 20 runs copied.

## Decision

lagent adopts gem-agent ADR-0105 §1–§4 as implemented at `edd8e60` (§8 there), in lagent's words. The pieces, and where lagent differs:

### 1. Every text block is saved, and recorded where no server can write

- Every text block of an MCP result, `isError` included, is saved in the work directory under its **full sha256**. The blocks a response hands over inline are saved together as one file (a single block as its own exact bytes); a block that does not fit is spilled with its preview sized before the write; the rest past the budget is saved joined, as today. A file already at a name is reused only when it is a regular file of the result's size whose own sha256 matches.
- The intake returns what it saved beside the text; the MCP adapter reports it to a recorder the executor puts in the call's context; the executor copies it into the tool-result message's new `saved` field (path, bytes, sha256, or the reason it was not saved). Nothing is parsed from the server's text.
- When the history is sent, lagent's own line is drawn from that field **outside the nonce tag**, beside the runtime note of gem-agent ADR-0075 §3 (as lagent already does for `RuntimeNote`):

  ```
  [lagent saved this result whole: <path> (<n> bytes, sha256 <64 hex>) — to keep it as evidence, write_file with from set to that path copies it byte for byte]
  ```

  With no work directory (the transcript is off) the line says there is no file to copy. A result saved as more than one file — the inline group, a spilled block, the rest past the budget, an image — is never called whole: each line reads `[lagent saved part <i> of <n> of this result: <path> (…) — to keep the whole result as evidence, copy every part, each with write_file from set to its path]` (§6). The wording is revised in §7.2.

**Under ADR-0034.** The note is not the server's text, so it is not paid from the response budget or the call's allotment, and the inline text is unchanged. It is bounded all the same: one line for the inline group, one per spilled piece (each of which had to fit the budget to exist) and one per saved image. It is bytes the model receives, so the round budget and the compaction estimate count it as sent: one function (`runtimeSuffix`) builds what the wrap appends — the runtime note and this note — and the same function is what `spend` and `estimateTokens` count (§6). ADR-0034's backstop (`CutUnkept`) applies to the result text as before; the note is appended after it, so a cut result still names its whole saved file.

### 2. `write_file` copies: `from` instead of `content`

- `from` names an existing regular file whose bytes become the new file; exactly one of `content` and `from` is given (an empty `content` beside `from` counts as absent). The destination is judged exactly as a write of content, so **a copy is never costlier than re-typing**. The shrink guard compares the source's size.
- The source is read **raw, whole and bounded** through the file-read child (ADR-0016) by a dedicated operation that is not a tool (`copy_source`), up to 32 MiB; above that, or when the read is interrupted, the copy is refused, never written partly. The copy is read back and must hash to what was read; the result line gives the sha256.
- **A credential source is Block** ("copies credential material"), never a credential read to approve. The child maps the kernel's refusal of a copy source to an error of its own, and the executor's in-process credential retry (ADR-0016 §2) runs only for the covered reads (`tools.ChildTool`) — in lagent those are exactly the tools that can return `ErrCredentialRead`. `from` joins `path` in the real-path resolution before classifying.
- The approval detail names the source and its size, from a stat only: it reaches the activity trace and the transcript, and a source's bytes must not.

### 3. The route is offered where it is needed

In the per-result note and in `write_file`'s description, worded as a route, not as a prohibition. The system prompt is not changed (it is byte-identical across sessions and changes only by ADR).

### 4. Verifying evidence stays outside the runtime

Whether an evidence file is a copy is a hash comparison against the transcript's `saved` records. lagent neither judges nor refuses a `write_file` whose content looks re-typed.

### 5. Measured after implementation, before acceptance

The probe's lagent cells are re-run against the implementation, pre-registered here before any run:

- **Cells.** lagent, s1 (inline) and s3 (spilled), prompts unchanged, the same model and server as the baseline above, n = 10 valid runs each. An s3 run counts only when its result was spilled.
- **What counts.** *Copied* is an evidence file byte-identical to a returned result, by `write_file` `from` or a shell copy (the scorer as amended for gem-agent ADR-0105). *Derived* and *regenerated* are reported beside it. A valid run with no evidence file is not copied.
- **Reading.** The route fires when at least 8 of 10 valid runs in each cell are copied. Against the baseline (s3 0 of 10, s1 0 of 3), that is the before-and-after gem-agent did not have.
- **Below the threshold.** The note and description are revised once and the cells measured once more; still below, the operator decides. §1 and §2 stand either way.
- **Cost.** A local server; nothing is billed. Failed or contaminated runs are replaced up to 2n.

### 5.1 First measurement (2026-11-04): below the threshold

Run at `8ed0853` as pre-registered, except `-jobs 2` (two runs at a time on the one server; the baseline ran one at a time). No run failed or was contaminated.

| Cell | Valid | Copied | Derived | Regenerated | No evidence |
|------|-------|--------|---------|-------------|-------------|
| s1 (inline) | 10 | **0** | 0 | 10 (exact 1, faithful 7, lossy 2) | 0 |
| s3 (spilled in all 10) | 10 | **2** | 2 | 3 | 3 |

The route did not fire in either cell. Read from the transcripts, the two cells fail for different reasons:

- **s1: the saved file is not taken up.** In 8 of 10 runs no call names it; the model re-types the result it has in context. One run copied it with `write_file` `from` and then overwrote the copy with a re-typed file; one tried `cp`, which `-p` refused.
- **s3: the saved file is taken up, but its path is mis-transcribed.** All 10 runs named the saved file. In 7 of them the first path given was wrong, and always in the same place. The file name, the 64-hex hash included, was copied right. What broke was the work directory's state-directory name, the project path with `/` replaced by `-` (`…-scratchpad-ep-work-w<N>-project`): the model wrote it back with slashes, as the project path that also stands in its context. `write_file` `from` was tried in 3 runs: 2 copied, and 1 failed on such a path. Of the 3 runs with no evidence, every one ended after shell routes (`mkdir`, `cp`) were refused.
- **Common to both.** In 18 of 20 runs the model tried `mkdir` or `cp` through write-lane `shell_exec`, refused in `-p`, although `write_file` creates parent directories.

The second cause is not in the wording: the note gives the path right, and the model's copy of it is wrong where the runtime's layout puts a name that looks like a mistyped copy of another path in context. Revising the wording, as this section pre-registers, does not reach it. Per the rule above the operator decides how to proceed; §1 and §2 stand.

## 6. Independent review (2026-11-04, before commit)

A sub-agent read the implementation against this record, gem-agent's and the code. Confirmed and taken:

1. **The note was invisible to the window accounting.** It is appended at send time, but `estimateTokens` (ADR-0033's compaction estimate) and the round budget's `spend` counted only the stored text — about 330–420 bytes per MCP result, retained for as long as the result is, so compaction could judge itself under the threshold and the next request be over it. Both now count `runtimeSuffix`, the function the wrap appends from.
2. **"saved this result whole" was false for a split result.** A response whose first block fits and whose next does not is two files, and each note claimed the whole; under lagent's allotment splits are common. Each line now names its part (§1). This is gem-agent's wording too.
3. **A multi-line description broke the one-line catalog** (ADR-0040): a `|` block kept its line breaks into the facts message. The description is joined into one line.
4. **`Saved` was trusted outside the tag but not pinned** by the provenance architecture test. It is now, counted only in an `llm.Message` literal (another struct has a `Saved` field).
5. Smaller: a carried-over test clause that could never fire, an error that printed `<nil>` when only the hash differed, and patch leftovers in the tree.

Noted, not changed: an image's saved file gets a note beside its own image note (bounded, and the copy route applies to an image too); after compaction the summary drops the notes, so the model loses older paths — the transcript keeps them.

## 7. Revision after the first measurement (2026-11-05)

§5.1 found two causes. The operator chose to fix the second in the runtime as well as revising the wording, and to spend §5's one further measurement on both together.

### 7.1 The work directory has a short name in the file tools

A path whose first segment is exactly `$LAGENT_WORK_DIR` or `${LAGENT_WORK_DIR}` is resolved under the session work directory by the file tools' one resolver (`resolvePath`), so `$LAGENT_WORK_DIR/<name>` means in `read_file`, `write_file` (`path` and `from`) and the rest what it already means in a shell command. It is the name the runtime-facts message already gives the directory, not a new one.

- **Containment is unchanged.** The expansion happens before the roots are checked, so `$LAGENT_WORK_DIR/../x` is refused like any other escape. With no work directory (the transcript is off) such a path is an error that says so, never a project path.
- **Judged as what it names.** The decision point resolves a judged path to its real path before classifying (`realPathArgs`), and that goes through the same resolver, so a write to `$LAGENT_WORK_DIR/x` is judged a work-directory write and a `from` naming credential material through it is still Block. The pin check of an approved write treats such a path as outside the project.
- **What is not expanded.** Only that first segment, only in the built-in file tools. An MCP tool argument is still passed as written, and the facts message keeps telling the model to give such a tool the directory in full. A project directory literally named `$LAGENT_WORK_DIR` is reached as `./$LAGENT_WORK_DIR`.

This removes the trap where it is: the model no longer has to reproduce the state-directory name, the project path with `/` replaced by `-`, which §5.1 found it rewrote as the project path. The file name it has to copy is the part it copied right in every run.

### 7.2 The note names the file that way, and gives the call

```
[lagent saved this result whole as $LAGENT_WORK_DIR/<name> (<n> bytes, sha256 <64 hex>). To keep it as evidence, copy the file instead of re-typing it: write_file {"path": "<destination>", "from": "$LAGENT_WORK_DIR/<name>"} copies it byte for byte and creates the parent directories — no shell command is needed]
```

A part of a split result reads `[lagent saved part <i> of <n> of this result as $LAGENT_WORK_DIR/<name> (…). To keep the whole result as evidence, copy every part: write_file {"path": "<destination>", "from": "$LAGENT_WORK_DIR/<name>"} for each]`. The absolute path stays in the transcript's `saved` record. The call is spelled out because in §5.1 the model reached for `mkdir` and `cp` first in 18 of 20 runs; the words name what makes them unnecessary rather than forbidding them. `write_file`'s `from` description and the facts message's work-directory line say the same.

### 7.3 The second measurement, pre-registered

§5 unchanged — cells, n = 10, at least 8 of 10 copied in each cell, failed or contaminated runs replaced up to 2n — at the commit that implements this section, `-jobs 2` as in §5.1. Still below the threshold in either cell, the operator decides; §1, §2 and §7.1 stand either way.

## Not decided here

- Built-in tool output (`shell_exec` already spools cut output, gem-agent ADR-0096 §3 as ported).
- The write-lane asymmetry itself (ADR-0032 stands).
- The work directory's lifetime: saved results go with it; evidence is copied into the project.
- Whether gem-agent takes §7.1: the trap is in its layout too, but it measured 20 of 20 copied without it. gem-agent's owner decides, with this section's measurement in hand.

## Consequences

- Every MCP result is on disk, not only the spilled ones, mode 0600 in the session work directory. Accepted: the operator keeps evidence of the same data in the project.
- About 300–400 bytes of note per saved file enter the context outside the allotment. Saved file names grow by 56 hex characters.
- `write_file`'s schema grows one property, the child one raw-bytes operation, the transcript's tool-result entry one field. Older lagent builds resuming a transcript ignore it.
- Accepted residuals, as in gem-agent: a copy gets the operator's default mode, not 0600; without the child, the credential check and the open are two steps; after a resume a note may name a work directory that is gone, and the copy then fails with that reason.

## Alternatives considered

As in gem-agent ADR-0105: saving alone (measured here: 0 of 10); writing results into the project unasked; a Safe rule over shell `cp` text (an unbounded domain, the class ADR-0008 retired); a separate `copy_file` tool (a capability without a trigger: the measured model reached for `write_file` every time); detecting re-typing in content; verification inside the runtime.

## References

- gem-agent ADR-0105 and `edd8e60` (the porting source); `tools/evidenceprobe/` in gem-agent (the probe, pre-registrations in `main.go`)
- `cmd/mcpresult.go`, `cmd/mcp.go`, `cmd/filechild.go`, `internal/tools/copysource.go`, `internal/tools/saved.go`, `internal/agent/saved.go`, `internal/risk/risk.go`
