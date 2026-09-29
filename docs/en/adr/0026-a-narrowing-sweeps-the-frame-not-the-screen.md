# ADR-0026: a narrowing sweeps the frame's rows, not the screen

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-09-29) — ported from gem-agent ADR-0094; implemented |
| Date | 2026-09-29 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | gem-agent ADR-0094, accepted by the operator: the TUI's screen clear on a width shrink lost the pictures on the screen and piled empty screens into the scrollback. This runtime's resize handling is the same, and the TUI's scrollback accounting is a mechanism both runtimes share, so the fix is ported in the same piece of work |
| Relates to | gem-agent ADR-0094 (the decision, its measurements and the operator's decisions), [ADR-0025](0025-mermaid-fences-render-as-pictures.md) (**its known limitation is resolved here**), [ADR-0020](0020-inline-images-declare-their-height.md) (the declared box and its accounting are not touched), [ADR-0001](0001-porting-sources-pinned.md) (porting provenance) |

## Context

On a width shrink the TUI returned `tea.ClearScreen` and reset its row
counter, to sweep the stale copies of the input box that the terminal's
re-wrap leaves above where Bubble Tea resumes. gem-agent measured what that
costs on iTerm2 and kitty, by hand, driven and by mouse drag (ADR-0094):
iTerm2 moves the screen into the scrollback on every clear, so a drag piled
hundreds of empty rows and a frame copy per report into it; kitty erases in
place and lost every line and picture on the screen. The same ADR found why
stale copies appear at all: the input box was drawn padded to the full
width, and a repaint during a live resize — the cursor blink alone — landed
wider than the screen, because the terminal runs ahead of the width it has
reported (iTerm2 reports, and updates the kernel's window size, only about
every 200 ms while it moves its screen continuously).

ADR-0025 recorded the limitation for this runtime and said it would be fixed
with gem-agent's.

## Decision

The three parts gem-agent adopted, as they are there:

- **C. A shrink erases the frame's rows, never the screen.** The program
  writes through `tui.SweepWriter`; on a shrink the model computes K, the
  rows the drawn frame gained by re-wrapping, and the writer extends the
  renderer's next flush upward by K and erases below it. The counter is set
  to the rows erased. Without a writer nothing is erased.
- **D. Frame rows are as short as their text.** Every row of the managed view
  ends at its last visible cell (`shortRows`); padding drawn on a background
  becomes an erase to the edge under it, so the input line's highlight still
  spans the window. The input box's cursor, a reversed blank, is kept.
- **E. The frame is drawn narrow while a resize is underway.** From a size
  report that changes the width until none has come for 400 ms every frame
  row is clipped to `minWidth − 1` cells; the settling tick draws it in full.
  A report that changes only the height starts none of this.

`internal/tui/sweep.go` and `shortrows.go` are ported from gem-agent at the
commit that adopted them, and the `Update`/`View` changes are the same.

What is not ported: the writer's measurement surface — the trace, the record
of each arm, `Inject` and `DrawnCells` — which only gem-agent's
`resizeprobe` reads. This runtime has no probes — no `tools/` — and a method
no one calls is a surface no one checks. `Stats` stays: the tests read it.

## Consequences

- The screen is no longer cleared after the first frame. ADR-0025's known
  limitation is resolved; the CHANGELOG says so.
- ADR-0020's accounting is unchanged: `emitSegments`, the declared rows and
  `physicalRows` are not modified; C only sets the counter to the rows it
  erased, and D shortens the managed view, not what is printed.
- While the window's width is changing, every row of the frame — the input
  box, the footer, a dialog, the live tail — is drawn cut to 19 cells, for
  400 ms after the last report that changed the width.
- The residue and scope gem-agent ADR-0094 records apply here unchanged:
  measured on iTerm2, kitty and tmux only; a terminal that counts erased
  cells with a background as content, or one that truncates instead of
  re-wrapping, can leave a stale row or blank rows of history; microsecond
  races can give one shrink a wrong K.
- The same tests hold it here as in gem-agent: the real renderer's flush
  begins with the cursor-up the sweep rewrites, K is the frame's wrap growth,
  a later arm replaces an earlier one, the frame is narrow until the last
  report settles, the cursor cell and the highlight survive the shortening,
  and no size report after the first clears the screen.
- Measured in gem-agent, not re-measured here: the mechanism and the frame it
  draws are the same code. Checked by hand by the operator with this
  runtime's built binary (`v0.11.0-5-g14b81c0`, 2026-09-29) on iTerm2 and
  kitty, a `/show` picture on the screen: the picture kept, no stale input
  box, no empty rows in the scrollback, widening back fine. A mermaid picture
  was not part of that check.

## Alternatives considered

- **Keep the clear until lagent measures on its own.** Rejected: the
  measurement is of the terminals and of code that is identical here; the
  clear's costs were measured on both terminals.
- **Port the probe-support methods too.** Rejected above: nothing here calls
  them.

## References

- gem-agent ADR-0094 and `tools/resizeprobe`
- gem-agent `internal/tui/sweep.go`, `internal/tui/shortrows.go`
