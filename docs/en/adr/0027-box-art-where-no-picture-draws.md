# ADR-0027: box art where no picture draws

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-09-30) — implemented |
| Date | 2026-09-30 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | mermaid-render RFP phase 2e (the operator's decisions of 2026-09-30): an in-house text-art renderer, "lagent gets it too" |
| Relates to | gem-agent ADR-0095 (the same engine replacing mermaid-ascii there), [ADR-0025](0025-mermaid-fences-render-as-pictures.md) (**§1's "No box-art lane is added" and A1 are superseded here**; the picture path is unchanged), [ADR-0024](0024-outside-text-is-made-inert-for-the-terminal.md) (**"No box-art hold" is amended here: `inertArt` is carried**), [ADR-0020](0020-inline-images-declare-their-height.md) (A1's "diagrams, if they come, join the lane this creates": what happens here) |

## Context

ADR-0025 drew mermaid fences as pictures where the terminal draws images and
left them as source everywhere else. It declined a box-art lane for two
reasons: mermaid-ascii is a community module this runtime does not carry,
and a renderer that draws wrong in text is what the pictures replaced.

Both reasons are gone. mermaid-render now draws flowchart / graph,
sequenceDiagram and erDiagram as text art (`raster.RenderText`) from the
same parse as its pictures, and checks every render on its grid, refusing a
fault. It is organization code, already a dependency here. All 43 real blocks
draw, and the operator passed 15 of them over four review rounds (gem-agent
ADR-0095).

## Decision

1. **Where the TUI draws no images, a mermaid fence of those three types is
   box art.** No protocol, a multiplexer under `images = "auto"`: the fence
   goes, as written, to `raster.RenderText`, with `TextOptions.Width` the
   TUI's own cell measure (`ansi.StringWidth` per rune). Where images draw,
   the picture path of ADR-0025 is unchanged; nothing falls back from a
   picture to art. The plain REPL and `-p` stay source.
2. **Art is its own segment.** `diagram.Split` without a Picture now
   partitions the reply: a drawn fence is a `Segment{Text: art, Art: true}`.
   The TUI writes it as it is, past glamour, which would wrap its lines at
   spaces and shear the boxes (gem-agent ADR-0063). It declares no rows, as a
   text segment declares none.
3. **Art is held to no escapes** (`inertArt`, `inert.String`): the engine
   writes none, so any escape in the art came from the text it drew. This is
   the box-art hold ADR-0024 did not need until now.
4. **Outcomes as for pictures** (ADR-0025 §4):
   - An unsupported type passes through silently as source.
   - Any other error, or an engine panic, shows the source with the one-line
     note.

## Consequences

- A terminal without images (Terminal.app, a multiplexer) shows these three
  types as diagrams instead of source.
- A reply is now partitioned on every TUI, not only where pictures draw.
- `internal/diagram` gains an art path, and `internal/inert` a caller in the
  art hold. The ADR-0024 tests pin the new call.
- A terminal set to draw East Asian ambiguous characters double-width
  misaligns the box drawing. This cannot be detected without a query, and is
  accepted as in gem-agent.

## Alternatives considered

- **Keep source where no picture draws.** Rejected: that was a stopgap while
  the only renderer was community code that drew wrong. The operator asked
  for the lane.
- **Draw art also for `-p` and the plain REPL.** Rejected, as in gem-agent
  (ADR-0042 §4 there): those outputs are for pipes and copying, and stay the
  model's text.
