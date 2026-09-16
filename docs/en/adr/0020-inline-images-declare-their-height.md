# ADR-0020: inline images declare their height — the counter is told, never measures

| Field | Value |
|-------|-------|
| Status | **Proposed** (2026-09-16) |
| Date | 2026-09-16 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator: when the terminal supports graphics, can a reply draw them inline — here and in gem-agent? |
| Relates to | [ADR-0005](0005-images-reach-the-model.md) (images reach the model — this decision is the other direction), [ADR-0001](0001-porting-sources-pinned.md) (the TUI came from gem-agent at a pinned commit); gem-agent ADR-0089 (the same decision on the other side — the two runtimes stay identical) |

## Context

ADR-0005 settled how an image reaches the **model**: a dropped path attaches,
`view_image` returned, and a reference that failed to attach is said. Nothing
has ever settled how an image reaches the **operator**. A screenshot an MCP
server saved is, on this surface, a file path in a line of text.

`emit` prints one line into scrollback and counts its physical rows, and the
bottom pinning rests on that count being exact. **An inline image defeats the
counter outright.** Measured against `charmbracelet/x/ansi` v0.11.6 — the
version this module pins, the same one gem-agent pins — `ansi.StringWidth`
returns **0** for an iTerm2 `OSC 1337 File=`, a kitty `APC _G` and a sixel
`DCS q` alike, and `ansi.Strip` returns the empty string, while the terminal
advances real rows. An image is not a wide line; it is a line the counter
cannot see at all. The same measurement shows `ansi.Hardwrap` leaves all three
payloads byte-identical, so `wrapForScrollback` shears nothing.

Bubble Tea runs **inline** here as it does there: completed output goes to the
terminal's native scrollback and is never repainted, so an image, once drawn,
is the terminal's to keep.

### What is different on this side

gem-agent partitions a reply before rendering — `diagram.Split` hands art
segments to the terminal verbatim, bypassing glamour, and gem-agent's
ADR-0063 built that lane for mermaid. **lagent has no such lane.**
`newGlamourRenderer` passes the whole reply through glamour in one piece;
there is no `Split`, no segment type, and no verbatim path. So this decision
does not join an existing lane here — it creates one, and an image is its
first and only member.

The runtime also already holds the other half of the material: `mcp.Client`
decodes a tool result's binary blocks to `Content{Data, MIME}`, and
`view_image` reads images from the project and the work directory. Both feed
the model today and neither reaches the screen.

One hazard is already recorded in this code rather than inherited.
`newGlamourRenderer` says it: `WithAutoStyle` is deliberately absent because
it queries the terminal and, once Bubble Tea owns stdin, the reply arrives as
phantom user input. Detecting a graphics capability is the same query and the
same hazard, and decision 4 answers it the same way.

### What the terminal does

Measured with gem-agent's `tools/rowprobe` on iTerm2 3.7.2, 180×80 cells, 16
of 16 cursor reports answered. This is a property of the **terminal**, not of
the runtime, so it transfers and the tool is not ported:

| case | declared | occupies | cursor Δ | end col |
|------|----------|----------|----------|---------|
| no size declared | (none) | 10 rows | 9 | 41 |
| `height=6` only | 6 | 6 rows | 5 | 25 |
| 40×6 box, aspect kept (wide image) | 6 | 6 rows | 5 | 41 |
| 40×6 box, aspect kept (tall image) | 6 | 6 rows | 5 | 41 |
| 40×6 box, stretched | 6 | 6 rows | 5 | 41 |
| `height=1` | 1 | 1 row | 0 | 5 |
| 40×12 box, aspect kept | 12 | 12 rows | 11 | 41 |
| text + image + text on one line | 6 | 6 rows | 5 | 38 |

1. **The declared box is reserved exactly, in both dimensions, whatever the
   picture does inside it.** The 40×12 case holds a 16:9 image that draws
   about ten rows and still occupies twelve; the 40×6 cases end at column 41
   though the drawing is twenty-four columns wide. No aspect-ratio derivation
   is needed — the declaration overrides the aspect ratio.
2. **The cursor is left on the image's last row**, at the column past the box,
   never at column 1. The raw delta is therefore one *less* than the
   occupancy; read as the count it makes a terminal honouring every
   declaration look like one honouring none, which is how the run was first
   read.
3. **An undeclared image takes its native size**, recoverable only from the
   cell pixel size. Declaring is the difference between a number we choose and
   one we have to go and ask for.
4. **Text on the same line lands badly**: the prefix on the image's first row,
   the suffix on its last.

Unresolved: after a 2.4 KB payload iTerm2 answered the next cursor report
0.8–1.5 s later. That bounds when its parser reached the image token and is
*not* established as draw latency.

macOS only, and **Terminal.app implements none of the three protocols**, so
the no-graphics path is a main road rather than an edge case.

## Decision

### 1. The emitter declares the height; the counter is told

`physicalRows` never measures an image and never learns to. An image segment
carries the row count its payload declares — `height=N` for iTerm2, `r=N` for
kitty — and `emit` adds **that** number. There is no second path to the
number, so nothing can disagree with it: the count is exact by construction
and the bottom pin stays exact. The cursor lands on the image's last row, so
the newline `emit` already appends opens the next row rather than adding one;
a segment declaring N rows costs exactly N.

### 2. The renderer gains a segment lane, and an image is its only member

`newGlamourRenderer` stops rendering the reply as one piece. It partitions
into segments, renders ordinary segments through glamour as now, and emits
image segments verbatim with their declared row count. This is the lane
gem-agent's ADR-0063 already has; here it arrives with one member and no
mermaid. Porting `internal/diagram` is **not** part of this decision (see
Alternatives).

An image occupies its own line, because text sharing the line with one is
split across its first and last rows.

### 3. Two protocols, and only the ones that can declare

**iTerm2 `OSC 1337 File=` and the kitty graphics protocol**, both of which
take the row count as a parameter — decision 1's precondition.

**Sixel is not taken.** It cannot declare a row count; the count would have to
be derived from the image's pixel height and the cell pixel size, reintroducing
the derivation decision 1 removes and resting on a second query that can fail.
It also needs an encoder, and the only ones available are community packages.
PNG and JPEG go to the two protocols above as bytes, with `image.DecodeConfig`
from the standard library the only decoding this runtime does.

### 4. The capability is probed once, before Bubble Tea owns stdin

`$TERM` cannot answer and is not asked. The terminal is queried — the kitty
`a=q` probe, `TERM_PROGRAM` plus `XTVERSION` for iTerm2 — **once, at startup,
before `tea.NewProgram`**, and cached for the session. Never during it, for
the reason `newGlamourRenderer` already gives about `WithAutoStyle`: a
terminal's reply to a query becomes phantom user input once Bubble Tea owns
stdin.

The probe takes a budget in seconds rather than milliseconds, drains the
stream before querying, and treats a reply that never arrives as *no
capability* rather than skipping it — a cursor report carries no tag, so an
abandoned reply is misfiled into the next query rather than lost (measured
while building `rowprobe`: 11 sent, 5 read, 6 landing on the shell prompt
after exit).

`[tui] images = "auto"` selects it, beside `theme` and `language`: `auto`
probes, `off` never draws, `iterm` / `kitty` force a protocol. Inside a
multiplexer the answer is **off** unless a protocol is forced — passthrough is
the multiplexer's configuration, not this runtime's to assume.

### 5. What may be drawn is a list, not a rule

Two entries:

- an **image content block in an MCP tool result** — already decoded to
  `Content{Data, MIME}` and today only forwarded to the model;
- a **local image file named by the reply**, as a Markdown image link, whose
  bytes decode as PNG or JPEG via `image.DecodeConfig`.

A third source is a new entry argued on its own, not a rule generalizing these
two.

Note what is deliberately *not* an entry: the bare image path ADR-0005 made
attachable. That grammar is about the **operator's input** reaching the model;
drawing is about the **model's output** reaching the screen. Reusing it would
make the runtime redraw the operator's own attachment back at them.

### 6. Only the view layer emits an image escape

Bytes arriving from a tool are data. The view layer decides a segment is an
image and writes the escape; nothing a tool returns is passed through as an
escape because it looks like one.

This does **not** close the existing surface: tool output is printed without
ANSI stripping today — `ansi.Strip` is called only to *measure* width — so raw
escapes from shell output already reach the terminal. Pre-existing, not
widened here, not repaired here. Written down so the next review finds it
named rather than missed.

### 7. The runtime says nothing about images

No tool, no prompt paragraph, no "this terminal can draw." The two sources in
decision 5 are things the model already produces for its own reasons, so there
is no capability waiting on a trigger that must be taught. A test pins the
absence.

## Consequences

- The bottom pin survives images by construction rather than by care.
- No aspect-ratio arithmetic and no cell-pixel-size query enter the runtime.
- Terminal.app loses nothing: the fallback is today's behaviour.
- **The per-image cost is unmeasured**, and the 0.8–1.5 s figure bounds the
  terminal's parser, not its drawing. Measuring it comes before `auto` is
  trusted in a streaming turn.
- This runtime gains a segment lane it has never had. That is the larger part
  of the work here and the part gem-agent does not have to do.
- An image a tool produced is drawn for the operator whether or not the model
  was given it. ADR-0005 settled ingestion; this settles the screen. They are
  separate surfaces and a tool's image can now reach both.

## Alternatives considered

**A1. Port `internal/diagram` from gem-agent first, and add images to that
lane.** Rejected as a precondition, not as an idea. The lane this needs is the
partition and the verbatim path, which is small; the diagram package is the
frozen mermaid translation table and the faithfulness guards, which are a
separate decision with their own evidence (gem-agent ADR-0042/0063) and no
measurement on this side. Bringing it along would make an image lane wait on a
mermaid argument nobody has had here yet. If diagrams come later they join the
lane this creates.

**A2. Measure the image instead of declaring it.** There is no measurement
path — the payload is zero cells wide to every surface the TUI has — and
asking the terminal per line means a cursor round-trip inside the loop,
impossible once Bubble Tea owns stdin.

**A3. Derive the row count from the image's pixels and the cell size.**
Measured unnecessary: the declared box overrides the aspect ratio, so the
derivation would compute a number the terminal ignores, and it would add an
`ESC[16t` query that can go unanswered.

**A4. Sixel, for breadth.** See decision 3.

**A5. An alt-screen region that manages images.** Rejected: inline mode and
the native scrollback are what let an image survive being scrolled past.

**A6. Reuse ADR-0005's bare-path grammar to decide what to draw.** Rejected —
see decision 5: it reads the operator's input, and drawing reads the model's
output.

**A7. Wait for gem-agent to implement first and port the result.** Rejected
for the decision, accepted as a possibility for the code. The two runtimes
have been repaired before for holding the same rule in one and not the other;
a decision taken on one side and left open on the other is that same class.
Which runtime writes the implementation first is a scheduling question, not a
design one.

## References

- gem-agent ADR-0089 — the same decision, and `tools/rowprobe`, the
  measurement behind the table above
- ADR-0005 — how an image reaches the model on this runtime
- gem-agent ADR-0063 §3 — art bypasses glamour, and why
