# ADR-0025: mermaid fences render as pictures where the terminal can draw them

| Field | Value |
|-------|-------|
| Status | **Proposed** (2026-09-29) |
| Date | 2026-09-29 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator: "use images as a rendering of the transcript — a runtime feature, not a model tool", for gem-agent and lagent alike (mermaid-render RFP); gem-agent ADR-0092 shipped it in gem-agent v0.85.0 |
| Relates to | gem-agent ADR-0092 (the decision, its measurements and its reviews, ported here), [ADR-0020](0020-inline-images-declare-their-height.md) (the declared-box lane; **§3's "the reply is never partitioned" and A1 are amended here**), [ADR-0022](0022-showing-is-an-act-of-output.md), [ADR-0024](0024-outside-text-is-made-inert-for-the-terminal.md) (the inert ingress and the renderer's hold), [ADR-0001](0001-porting-sources-pinned.md) (porting provenance) |

## Context

gem-agent ADR-0092 holds the analysis, the alternatives, the operator's
decisions and the measurements on iTerm2 and kitty; this record does not
repeat them. What is this runtime's own is the difference in what it starts
from.

### What is different on this side

- **There is no box-art lane.** gem-agent drew mermaid fences as box art with
  mermaid-ascii (its ADR-0042/0063) before pictures, and keeps that lane where
  no picture can be drawn. This runtime never had it: ADR-0020 A1 declined to
  port `internal/diagram` as a precondition — "diagrams, if they come, join
  the lane this creates" — and ADR-0024 records that it draws no diagrams. A
  fence in a reply is shown as the model wrote it, as Markdown code.
- **Model text is made inert at the TUI's ingress, and the Markdown
  renderer's output is held to its own styling** (ADR-0024 §2–§3). A picture
  payload is an escape sequence this runtime writes itself; it must reach the
  terminal without passing through that hold, as a tool image already does.
- **The reply is never partitioned** (ADR-0020 §3): `newGlamourRenderer`
  renders it in one piece and the segment lane carries only tool images.

The engine is the same organization library, `github.com/nlink-jp/mermaid-render`
v0.1.0, and the terminals are the same two, measured by the same operator.

## Decision

Ported from gem-agent v0.85.1 (`8d7c780`), ADR-0001.

1. **Where the TUI draws images (iTerm2 or kitty, ADR-0020 §7), a mermaid
   fence in a reply is a picture**, drawn by mermaid-render from the source
   as written. Everywhere else — no protocol, a multiplexer under
   `images = "auto"`, the plain REPL, `-p` — the fence is shown as source,
   exactly as today. **No box-art lane is added**: mermaid-ascii is a
   community module this runtime does not carry, and a renderer that draws
   wrong in text is what gem-agent's pictures replaced (gem-agent ADR-0092
   B1/B4).
2. **The reply is partitioned** (`diagram.Split`, picture-only here): the
   reply renderer returns segments with declared rows, `takeLive` and its four
   callers pass them through as one write, and a picture's segments carry
   their rows to the counter (ADR-0020 §1). This amends ADR-0020 §3's "the
   reply is never partitioned": a reply now is, and an image may now arrive
   from inside it.
3. **The box, the bands and the limits are gem-agent ADR-0092 §4's, as
   measured**: one em of diagram text per terminal line (28 px per em,
   mermaid-render Scale 2), the width from the cell aspect read with
   `TIOCGWINSZ` (an ioctl, never a query; 2.25 when the terminal reports no
   pixels); a picture wider than the terminal less one column shrinks; a
   taller one is not shrunk and scrolls — on kitty in bands of at most half
   the screen, since kitty clips a picture taller than the screen, and whole
   on iTerm2, where bands showed seams. The encoded payloads together may not
   pass `termimg.MaxBytes`.
4. **Every failure shows the source with a one-line note, never nothing**
   (gem-agent ADR-0092 §5): a syntax error, an unsupported construct, a
   character no font has, a limit, a layout that breaks the engine's own
   checks, a payload that cannot be built, a panic in the engine. An
   unsupported diagram type (gantt, class, …) is the source, silently.
5. **The font is read once, by the cmd layer, only where images draw**
   (`[tui.diagram]`: `font`, `font_name`, `bold_font`, `bold_font_name`;
   default Hiragino Sans W3 / W6). A setting that does not load is one banner
   warning and the default font, never a refusal to start (the operator's
   decision in gem-agent ADR-0092 §6); without the default font either,
   fences stay source. User config only.
6. **Inert, as ADR-0024 decided**: the fence text the engine reads has passed
   the ingress; a note is Markdown and goes through the renderer and its hold;
   a picture payload is built by `internal/tui` from the engine's pixels and
   goes to `emitSegments` beside the renderer, never through it, as a tool
   image does. `termimg.Payload` stays callable from `internal/tui` only.
7. **The runtime still says nothing about diagrams**: no tool, no prompt
   paragraph.

## Consequences

- On iTerm2 and kitty a reply's diagrams are readable, CJK labels included.
- **Known limitation, measured in gem-agent** (ADR-0092 §4): narrowing the
  window loses the pictures on the screen at that moment and leaves black
  space in the scrollback, because the TUI clears the screen on a shrink —
  this runtime's resize handling is the same. It is revisited with gem-agent's.
- lagent gains a dependency on `github.com/nlink-jp/mermaid-render` (an
  organization module) and, through it, `golang.org/x/image`.
- ADR-0024's "this runtime draws no diagrams" becomes "draws them only as
  pictures"; its conclusion — no box-art hold — stands, since there is still
  no box art.

## Alternatives considered

**A1. Port the box-art lane too, as gem-agent has it.** Rejected: the art was
measured unused in gem-agent (its ADR-0092 context), mermaid-ascii is a
community module, and a terminal without images shows the fence as source,
which is correct if plain.

**A2. Wait for gem-agent's shrink-clear rework before porting.** Rejected: the
limitation is the same on both sides and is fixed in both when it is fixed;
the pictures are useful now.

## References

- gem-agent ADR-0092 (§1–§8, the measurements in §4, the known limitation).
- mermaid-render RFP (lib-series).
