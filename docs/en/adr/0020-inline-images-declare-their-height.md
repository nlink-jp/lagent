# ADR-0020: inline images declare their box — the counter is told, never measures

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-09-17) — implemented; decision 3 gained the erase after gem-agent's first run on a real terminal |
| Date | 2026-09-16 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator: when the terminal supports graphics, can a reply draw them inline — here and in gem-agent? |
| Rewritten because | An independent verification pass returned findings in three classes against both sides of this decision: claims about adjacent code asserted without reading it, "measured" claims wider than the instrument, and a lane opened without enumerating the dimensions it opens. One finding was specific to this side and is corrected below |
| Relates to | [ADR-0005](0005-images-reach-the-model.md) (images reach the model — this is the other direction), [ADR-0001](0001-porting-sources-pinned.md) (the TUI came from gem-agent at a pinned commit), [ADR-0015](0015-credential-reads-are-operator-only.md) / [ADR-0016](0016-the-kernel-reads-the-file.md) (who may open a file); gem-agent ADR-0089 (the same decision on the other side) |

## Context

ADR-0005 settled how an image reaches the **model**: a dropped path attaches,
`view_image` returned, and a reference that failed to attach is said.
Nothing has settled how one reaches the **operator**. A screenshot an MCP
server saved is, on this surface, a path in a line of text.

### What the counter can and cannot see

`emit` ([model.go:818](../../../internal/tui/model.go)) prints one line into
scrollback and counts its physical rows; the bottom pinning rests on that
count. Measured against `charmbracelet/x/ansi` v0.11.6 — the version
`go.mod:11` pins, the same one gem-agent pins — `ansi.StringWidth` returns
**0** and `ansi.Strip` the empty string for an iTerm2 `OSC 1337 File=`, a
kitty `APC _G` and a sixel `DCS q` alike, and `ansi.Hardwrap` leaves all
three byte-identical, so `wrapForScrollback`
([model.go:1071](../../../internal/tui/model.go)) shears nothing.

The counter is not blind, though. `physicalRows`
([model.go:1084](../../../internal/tui/model.go)) starts at `rows, cells :=
1, 0`, so an image line is credited with exactly **one** row while the
terminal advances N. The shortfall is `N-1`, not `N`.

### What a shortfall costs, measured with a control and in the right regime

The measurements live in gem-agent (`tools/rowprobe`, `tools/pinprobe`,
`tools/imgpayload`) and are **not ported**: they measure a terminal, not a
runtime, and this repository has no `tools/` directory. `pinprobe` drives
that runtime's real model through its real emit path — which is this
runtime's emit path too, function for function ([model.go:818, :1071,
:1084](../../../internal/tui/model.go) against gem-agent's :872, :1130,
:1143; `diff` returns nothing on the last two).

The regime is arranged, not assumed. The pin's padding is
`height − printed − view − 1` and the positive branch is labelled "screen
not full" ([model.go:1723](../../../internal/tui/model.go)). A filler count
chosen for a 30-row pane left an 80-row window on the other side of it, and
an earlier draft of both records reported those runs under the wrong label.

| terminal | payload | regime | stranded | control at same fill |
|---|---|---|---|---|
| iTerm2 3.7.2, 180×80 | OSC 1337 `height=12` ×3 | full | **3** | PLAIN ×3, clean |
| tmux 3.7c, 120×30 | sixel ×3, at 36 / 72 / 144px | full | **3** each | PLAIN ×3, clean |
| tmux 3.7c, 120×30 | OSC 1337 and kitty ×3 | full | 0 | same run: swallowed there, so controls |
| both | every case | not full | 0 | identical |

A terminal that draws what the counter cannot see strands one frame per
image, once the screen is full — on two terminals and two protocols, each
against a control at the same fill. Nothing happens while the screen is not
yet full: the pad absorbs it.

`rowprobe` separately established that a **declared box is reserved exactly,
in both dimensions, whatever the picture does inside it** — a 16:9 image in
a 40×12 box draws about ten rows and occupies twelve — and that the cursor
is left on the image's last row, past the last cell written there. That is
what makes a declaration usable as a count.

### What is different on this side

gem-agent partitions a reply before rendering: `diagram.Split` hands art
segments to the terminal verbatim, a lane its ADR-0063 built for mermaid.
**This runtime has no such lane.** `newGlamourRenderer`
([model.go:448](../../../internal/tui/model.go), the render at `:461`)
passes the whole reply through glamour in one piece — no `Split`, no segment type, no verbatim
path. So this decision does not join a lane here; it creates one, and an
image is its first and only member. That is the larger part of the work.

The other half of the material is already here: `mcp.Client` runs a tool
result's binary blocks through `base64.StdEncoding.DecodeString`
([client.go:638](../../../internal/mcp/client.go);
`:575` is the `Content` carrier an earlier draft cited instead),
and the intake **writes an image into the session work directory** and hands
the model `[image saved at <path> … use view_image on that path]`
([mcpresult.go:200](../../../cmd/mcpresult.go)). The first draft said such
blocks were "forwarded to the model"; they are not — the bytes never ride
back inline.

**A correction specific to this side.** The first draft said the hazard
behind decision 7 was "recorded in this code rather than inherited", citing
`newGlamourRenderer`'s note that `WithAutoStyle` is deliberately absent
because it queries the terminal and the reply arrives as phantom user input
once Bubble Tea owns stdin. The note is real and the hazard is real, but it
is **byte-identical to gem-agent's** — inherited under ADR-0001, not
recorded here. The provenance claim was wrong. A related divergence was recorded here and has since
been repaired rather than left recorded: gem-agent's `AGENTS.md` carried the
stdin-ownership fact behind that hazard and this repository's did not,
although both pin `bubbletea v1.3.10`. This repository's `AGENTS.md` carries
it as of 2026-09-17, and both now list `internal/tui`'s scrollback
accounting among the shared mechanisms — which neither did while these
records were arguing that it is shared.

### Not measured, and not asserted

Whether kitty or Ghostty honour `r=` (every measurement is iTerm2 or tmux);
whether Terminal.app implements any of the three; the per-image cost (iTerm2
answered the next cursor report 0.8–1.5 s after a 2.4 KB payload, which
bounds its parser, not its drawing); and a terminal that draws without
reserving rows, where the correction would credit one row too many.

## Decision

### 1. The emitter declares the box; the counter is told

`physicalRows` never measures an image. An image segment carries the row
count its payload declares — `height=N` for iTerm2, `r=N` for kitty — and
`emit` uses that number **in place of** the one row `physicalRows` floors
to, not in addition to it.

### 2. The declaration covers columns too

The emitter declares a column count as well and clamps it below `m.width`.
A declared box is a cage in two dimensions or it is not a cage.

The reason is the **row** count, not the renderer. An earlier draft argued
that `wrapForScrollback`'s invariant goes unenforced for an image line; a
verification pass refuted it, because Bubble Tea's renderer gates on
`ansi.StringWidth` too and so always gives a zero-width line its
`EraseLineRight`. What harms is an image wider than the terminal: the
terminal wraps the picture itself and adds rows the declared height never
claimed.

### 3. The renderer gains a segment lane, and an image is its only member

`newGlamourRenderer` stops rendering the reply as one piece: it partitions
into segments, renders ordinary ones through glamour as now, and emits image
segments verbatim with their declared box, each preceded by one erase. An
image occupies its own line, because text sharing a line with one is split
across its first and last rows. Porting `internal/diagram` is **not** part
of this (see A1).

**The erase is not decoration, and only a real terminal found it.** Bubble
Tea flushes a queued line from the top of its own frame and appends
`EraseLineRight`, which clears ONE row. An image then draws down N rows at
its declared width, so every cell of the old frame to the RIGHT of a
narrower picture survives on every row the picture covers. Measured on
gem-agent's iTerm2 with the declared count already correct: three images,
three stranded footers — the same damage this record was written to
prevent, from a second cause it had not enumerated. `ansi.EraseScreenBelow`
before the payload removes the frame the renderer is about to repaint below
us anyway, and touches nothing above the cursor. The emit path here is the
same one, function for function, so the fix is carried rather than
rediscovered — and four verification passes missed it, none of them having
a terminal.

### 4. Two protocols, and only the ones that can declare

**iTerm2 `OSC 1337 File=` and the kitty graphics protocol**, both of which
take the row count as a parameter — decision 1's precondition. **Sixel is not taken**: it cannot declare one, which settles it.

A second reason was tried twice and is **removed rather than adjudicated a
third time**. The organization's supply-chain rule was withdrawn here as
unsupported (wrong: it is in this repository's own RFP), restored as a
reason to refuse an encoder, then narrowed to "API clients". Its reach is
not settled here and decision 4 does not rest on it.

### 5. What may be drawn is NOT settled here — one constraint is

Written three times, refuted three times, each time the worst finding of its
round: a path the model names bypasses the enforcers, which are keyed on
tool name (`PathJudged`, [risk.go:323](../../../internal/risk/risk.go)); the
path the intake wrote can be pre-empted, because `write` short-circuits on
`os.Stat` ([mcpresult.go:223](../../../cmd/mcpresult.go)) and every call
hands the server the work directory in `_meta`
([client.go:608](../../../internal/mcp/client.go)); and the third draft's
decoded bytes have no carrier at all — `render` returns a `string`
([mcpresult.go:61](../../../cmd/mcpresult.go)) and `Tool.Run` is
`func(ctx, args) (string, error)` ([tools.go:67](../../../internal/tools/tools.go)).

Three drafts in one place is one mistake: **the source cannot be named until
the plumbing exists.** It is deferred to [ADR-0021](0021-an-images-bytes-never-become-a-path.md),
which inherits the one
constraint that held against all three: **the view layer opens no file.**
Whatever the source turns out to be, the bytes must arrive by a channel the
enforcers already govern, because a read the view layer performs is not a
tool call and nothing here can see it.

**ADR-0005's bare-path grammar is still not reused for this**, and now for
the older of the two reasons only: it reads the **operator's input** while
drawing reads the **model's output**. The second reason — that reusing it
would open the view-layer read — belongs to the deferred decision now.

### 6. Only the view layer emits an image escape

Bytes arriving from a tool are data. The implementation commit carries an
architecture test enumerating the sites that may emit an escape; without it
this sentence is "as of today". This does not close the existing surface:
`ansi.Strip` is called at one site in non-test code
([model.go:1089](../../../internal/tui/model.go)), inside `physicalRows`, to
*measure* — so raw escapes from shell output already reach the terminal.
Pre-existing, not widened here, not repaired here.

### 7. Drawing is TUI-only, and the capability is probed once

`tea.NewProgram` is constructed at one site
([root.go:1350](../../../cmd/root.go)); one-shot `-p` and the plain REPL
never build it and never draw. The probe runs **before** that construction
and is cached, for the reason `newGlamourRenderer` records about
`WithAutoStyle` — inherited from the porting source, and true here. It takes
seconds, drains before querying, and treats no reply as *no capability*,
because an abandoned cursor report is misfiled into the next query, not
lost.

`[tui] images = "auto"` selects it, beside `theme` and `language`
([config.go:180](../../../internal/config/config.go) and `:184`). Inside a multiplexer
the answer is **off** — because the one measured rendering a payload
stranded a frame for every image, not because passthrough is someone else's
configuration.

### 8. The runtime says nothing about images

No tool, no prompt paragraph. The first draft argued the model "already
produces" these sources; that is a firing-rate claim and neither runtime has
a denominator for it. And §5 defers the source, so there is no source for a prompt to steer
toward — an earlier draft argued from "decision 5's single source", which
was one of the refuted drafts. This also
keeps the decision clear of `CLAUDE.md`'s rule that a behavioural claim
about the local model is measured on the bench (ADR-0006) before it is
relied on — no such claim is made.

## Consequences

- The bottom pin survives images by construction, in both dimensions.
- This runtime gains a segment lane it has never had. That is the larger
  part of the work here and the part gem-agent does not have to do.
- An image a tool produced **is** drawn for the operator: [ADR-0021](0021-an-images-bytes-never-become-a-path.md)
  named the source §5 deferred — the MCP intake, for a block it both saved
  and described. ADR-0005 settled ingestion on this side; the screen is now
  settled on both, and gem-agent ADR-0089/0090 say the same.
- Terminal.app — and any terminal that does not draw — loses nothing.
- What is unmeasured stays unmeasured; `auto` should not be trusted in a
  streaming turn until the per-image cost is.
- gem-agent ADR-0089 is the same decision on the other side. Neither runtime
  may hold it alone.

## Alternatives considered

**A1. Port `internal/diagram` first and add images to that lane.** Rejected
as a precondition, not as an idea: the lane this needs is the partition and
the verbatim path, which is small, while the diagram package is the frozen
mermaid translation table and its faithfulness guards — a separate decision
with its own evidence (gem-agent ADR-0042/0063) and no measurement here.
Diagrams, if they come, join the lane this creates.

**A2. Measure the image instead of declaring it.** No measurement path
exists, and a cursor round-trip per line is impossible once Bubble Tea owns
stdin.

**A3. Derive the row count from pixels and the cell size.** Measured
unnecessary: the declared box overrides the aspect ratio.

**A4. Sixel, for breadth.** Decision 4, now with a measurement.

**A5. An alt-screen region that manages images.** Rejected: inline mode and
the native scrollback are what let an image survive being scrolled past.

**A6. Reuse ADR-0005's bare-path grammar to decide what to draw.** Decision
5 — wrong input, and it opens a read outside every enforcer.

**A7. Wait for gem-agent to implement first and port the result.** Rejected
for the decision, accepted as a possibility for the code: which runtime
writes the implementation first is scheduling, not design.

## References

- gem-agent ADR-0089 — the same decision, and `tools/rowprobe` /
  `tools/pinprobe` / `tools/imgpayload`, the measurements behind the tables
  above
- ADR-0005 — how an image reaches the model here; ADR-0015 / ADR-0016 — who
  may open a file
- gem-agent ADR-0063 §3 — art bypasses glamour, and why
