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

  With no work directory (the transcript is off) the line says there is no file to copy. A result saved as more than one file — the inline group, a spilled block, the rest past the budget, an image — is never called whole: each line reads `[lagent saved part <i> of <n> of this result: <path> (…) — to keep the whole result as evidence, copy every part, each with write_file from set to its path]` (§6).

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

## 7. A runtime workaround, considered and withdrawn (2026-11-05)

After §5.1 a revision was drafted and implemented: the file tools would resolve a path whose first segment is `$LAGENT_WORK_DIR` under the work directory, the note and the intake's own notes would name saved files that way, and the note would spell out the `write_file` call. An independent review found no route past containment or the credential rules in it. At the operator's decision it was withdrawn before it was measured, and its commit reverted (`ba336de`, reverted by `dbbcc7d`):

- **It compensates one model's weakness, not a false fact.** The note names the right path and the right route; what failed is the model's copy of a long path. gem-agent, with the same layout, measured 20 of 20 copies.
- **It widened what a path argument means.** Every reader of one had to know the variable, and the review found three that did not (`file_info`'s escaping-link report, the pin check, the intake's own spill notes), with three further residuals accepted.
- **It did not reach s1**, where the saved file is not taken up at all.
- **It would have split the two runtimes' file tools**, or carried into gem-agent a mechanism it does not need.

The wording revision §5 allows is not taken either. What §5.1 measured is a property of this model, recorded as such: that the runtime's facts are true and the route exists is this record's to decide; whether a given model takes the route is the model's. ADR-0039's status is decided by the operator on §5.1's result.

## Not decided here

- Built-in tool output (`shell_exec` already spools cut output, gem-agent ADR-0096 §3 as ported).
- The write-lane asymmetry itself (ADR-0032 stands).
- The work directory's lifetime: saved results go with it; evidence is copied into the project.

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
