# ADR-0025: mermaid fences render as pictures where the terminal can draw them

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-09-29) — implemented |
| Date | 2026-09-29 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator: "use images as a rendering of the transcript — a runtime feature, not a model tool", for gem-agent and lagent alike (mermaid-render RFP); gem-agent ADR-0092 shipped it in gem-agent v0.85.0 |
| Relates to | gem-agent ADR-0092 (the decision, its measurements, the operator's decisions and its reviews, ported here), [ADR-0020](0020-inline-images-declare-their-height.md) (the declared-box lane; **the Context's "a reply is never partitioned" and §3's "an image does not arrive inside a reply" are amended here, A3 is answered, and A1 is fulfilled**), [ADR-0022](0022-showing-is-an-act-of-output.md), [ADR-0024](0024-outside-text-is-made-inert-for-the-terminal.md) (the inert ingress and the renderer's hold; **its "no picture or cell-aspect callbacks" is amended here**), [ADR-0001](0001-porting-sources-pinned.md) (porting provenance) |

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
  the lane this creates" — and ADR-0024 records that there is no box art to
  hold. A fence in a reply is shown as the model wrote it, as Markdown code.
- **Model text is made inert at the TUI's ingress, and the Markdown
  renderer's output is held to its own styling** (ADR-0024 §2–§3). A picture
  payload is an escape sequence this runtime writes itself; it must reach the
  terminal without passing through either, as a tool image already does.
- **A reply is never partitioned** (ADR-0020's Context): `newGlamourRenderer`
  renders it in one piece, and the segment lane carries only tool images,
  which arrive from outside a reply (ADR-0020 §3).

The engine is the same organization library, `github.com/nlink-jp/mermaid-render`
v0.1.0, and the terminals are the same two, measured by the same operator.

## Decision

Ported from gem-agent v0.85.1 (`8d7c780`), ADR-0001. The picture code landed
in gem-agent v0.85.0 and is unchanged in v0.85.1; the pin is v0.85.1 so that
`renderReply` runs under the renderer hold this runtime already carries from
that commit (ADR-0024).

1. **Where the TUI draws images (iTerm2 or kitty, ADR-0020 §7), a mermaid
   fence in a reply is a picture**, drawn by mermaid-render from the source
   as written. Everywhere else — no protocol, a multiplexer under
   `images = "auto"`, the plain REPL, `-p` — the fence is shown as source,
   exactly as today: without a Picture the reply is not partitioned at all and
   renders whole, as before. **No box-art lane is added**: mermaid-ascii is a
   community module this runtime does not carry, and a renderer that draws
   wrong in text is what gem-agent's pictures replaced (gem-agent ADR-0092
   B1/B4). A forced `images = "iterm"` or `"kitty"` inside a multiplexer that
   swallows the payload loses the diagram; that setting is the operator's
   assertion that the terminal draws, and the residual is accepted, as in
   gem-agent.
2. **Where pictures draw, the reply is partitioned** (`diagram.Split`,
   picture-only here): the reply renderer returns segments with declared
   rows, `takeLive` and its four callers pass them through as one write, and a
   picture's segments carry their rows to the counter (ADR-0020 §1). The
   Markdown around a picture is rendered in separate passes, so a list or a
   quote that spans a fence is broken there. A fence that is not closed — a
   reply cut off by `finish_reason` `length`, by Ctrl+C, or flushed mid-fence
   at a tool call or the auto toggle — is text, and shows as source with no
   note; the draw was never attempted.
3. **The box, the bands and the limits are gem-agent ADR-0092 §4's, as
   measured**: one em of diagram text per terminal line (28 px per em,
   mermaid-render Scale 2), the width from the cell aspect read with
   `TIOCGWINSZ` (an ioctl, never a query; 2.25 when the terminal reports no
   pixels). This answers ADR-0020 A3 as gem-agent ADR-0092 answered its
   counterpart: both dimensions are still declared, the arithmetic only
   chooses them, and an ioctl read enters where a query does not. A picture
   wider than the terminal less one column shrinks; a taller one is not shrunk
   and scrolls — on kitty in bands of at most half the screen, since kitty
   clips a picture taller than the screen, and whole on iTerm2, where bands
   showed seams. The encoded payloads together may not pass
   `termimg.MaxBytes`.
4. **Every failure shows the source with a one-line note, never nothing**
   (gem-agent ADR-0092 §5): a syntax error, an unsupported construct, a
   character no font has, a limit, a layout that breaks the engine's own
   checks, a payload that cannot be built, a panic in the engine. An
   unsupported diagram type (gantt, class, …) is the source, silently.
5. **The font is read once, by the cmd layer, only where images draw**
   (`[tui.diagram]`: `font`, `font_name`, `bold_font`, `bold_font_name`;
   default Hiragino Sans W3 / W6; user config only, recorded with its source
   and shown in `/settings` as rows that apply at the next start). A value
   that does not load — a missing file, an unknown face, a name without its
   file — is one banner warning and the default font, never a refusal to
   start (the operator's decision in gem-agent ADR-0092 §6); without the
   default font either, fences stay source. An unknown key in the table is
   refused like any other, by the strict decode. The engine reads font files
   itself, outside `TestReadsAreBounded`'s reach; mermaid-render refuses
   anything but a regular file and stops at 256 MiB.
6. **Inert, as ADR-0024 decided**: the fence text the engine reads has passed
   the ingress; a note is Markdown and goes through the renderer and its hold;
   a picture payload is built by `internal/tui` from the engine's pixels,
   inside `Update`, and goes to `emitSegments` beside the renderer, never
   through it, as a tool image does. `termimg.Payload` stays callable from
   `internal/tui` only. **A payload never rides a `tea.Msg`**: the ingress
   would strip its ESC and BEL, and the base64 would print as text while the
   counter was told N rows.
7. **Rendering runs on the update path**, at the four places the live text is
   flushed, two of them keypresses while a reply streams (gem-agent ADR-0092
   §8: a real diagram renders in 1–9 ms and encodes in 4–49 ms, a dense one in
   about 0.3 s). If the stall is ever moved to a `tea.Cmd`, the message
   carries pixels or bytes and the payload is built on receipt, per §6.
8. **The runtime still says nothing about diagrams**: no tool, no prompt
   paragraph.

## Consequences

- On iTerm2 and kitty a reply's diagrams are readable, CJK labels included.
  Checked by the operator on both terminals with this runtime before release
  (2026-09-29): size, a tall diagram on kitty, and the order after a picture.
- **Known limitation, measured in gem-agent** (ADR-0092 §4): narrowing the
  window loses the pictures on the screen at that moment and leaves black
  space in the scrollback, because the TUI clears the screen on a shrink —
  this runtime's resize handling is the same. It is revisited with gem-agent's.
- lagent gains a dependency on `github.com/nlink-jp/mermaid-render` (an
  organization module) and, through it, `golang.org/x/image`; the module
  graph moves `golang.org/x/sys` from v0.47.0 to v0.48.0 and `golang.org/x/text`
  from v0.30.0 to v0.42.0, which mermaid-render requires. goldmark, the
  decoder ADR-0024 §3 measured, is not moved.
- Swept in the same change: ADR-0020 carries an "Amended by ADR-0025" note;
  ADR-0024's registry sentence is amended and its "no diagrams" citations
  point at ADR-0020 A1; the withdrawn-claims test's reason for "stops
  rendering the reply as one piece" is rewritten; `internal/tui`'s
  `Options` registry exempts `Picture` and `CellAspect` with their reasons.

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
