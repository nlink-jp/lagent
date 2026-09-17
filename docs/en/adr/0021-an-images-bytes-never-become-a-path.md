# ADR-0021: an image's bytes reach the screen without ever becoming a path

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-09-17) — implemented |
| Date | 2026-09-17 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | [ADR-0020](0020-inline-images-declare-their-height.md) §5 deferring the source: it settled how many rows an image costs and refused to name what may be drawn, because the question had been answered three times and refuted three times |
| Relates to | ADR-0020 (the declared box and the accounting), [ADR-0005](0005-images-reach-the-model.md) (how an image reaches the model here), [ADR-0015](0015-credential-reads-are-operator-only.md) / [ADR-0016](0016-the-kernel-reads-the-file.md) (who may open a file); gem-agent ADR-0090 (the same decision on the other side) |

## Context

ADR-0020 built the lane and left the source open, keeping one constraint
that survived all three refuted drafts: **the view layer opens no file.** A
read the view layer performs is not a tool call, so it reaches neither the
agent's decision nor ADR-0016's sandboxed child, and the path-judging list
— `pathJudgedTools`, keyed on tool name
([risk.go:323](../../../internal/risk/risk.go)) — cannot see it.

That rules out the obvious design. The MCP intake already writes an image
into the session work directory and hands the model
`[image saved at <path> … use view_image on that path]`
([mcpresult.go:224](../../../cmd/mcpresult.go)), so a path is sitting
there — but `write` short-circuits on `os.Stat`
([mcpresult.go:247](../../../cmd/mcpresult.go)) while every call hands the
server the work directory as `_meta[workdir.MetaKey]`
([client.go:608](../../../internal/mcp/client.go)). A local server child
knows its own name, its tool name, the bytes it will return and the
directory, so it can plant a symlink at the content-addressed name before
answering; the runtime then writes nothing and the path resolves where the
server chose. Reaching that file through `view_image` is contained, because
the agent resolves symlinks before the enforcers judge the real path. A
view layer that opened it would not be.

### The plumbing, on this side

The third refuted draft said the view layer would be handed "the bytes the
intake already holds". It cannot: `render` returns a `string`
([mcpresult.go:61](../../../cmd/mcpresult.go)), `mcpIntake` keeps only a
work-directory getter, a byte cap and a preview length
([mcpresult.go:54](../../../cmd/mcpresult.go)), and the tool contract is
`Run func(ctx, args) (string, error)`
([tools.go:67](../../../internal/tools/tools.go)).

The channel this needs already exists here too, and it is the same one:
the agent loop talks to the UI **during** a tool call —
`prog.Send(tui.ToolCall{…})` at [root.go:809](../../../cmd/root.go). Nothing
about the string contract has to move.

### The cost

Measured on gem-agent, against the counter both runtimes share
(`physicalRows` is byte-identical in the two trees): about **3.6 ms per
MiB** of decoded image, linear from 0.35 ms at 64 KiB to 29 ms at 8 MiB,
and the payload string is held in three places at once — the emit path,
Bubble Tea's queue, and the terminal's scrollback. Nothing bounds it today
but the JSON-RPC frame cap (`scannerMax = 10 MiB`,
[client.go:29](../../../internal/mcp/client.go)), which allows roughly
7.5 MiB decoded per block. The measurement is not repeated here because it
measures shared code, not a runtime.

## Decision

Identical to gem-agent ADR-0090, because the intake, the tool contract and
the UI channel are the same mechanism in both trees. Stated in full rather
than by reference, so this record is readable cold.

### 1. The bytes travel out-of-band, on the channel the UI already has

`mcpIntake` gains one optional sink beside `workDir`, called with the
decoded bytes and their MIME type as the block is taken in, and `cmd` wires
it to `prog.Send`. The tool result stays a `string`; the transcript, resume
and error paths are untouched. The sink is nil in every entrance that is
not an interactive TUI, so one-shot `-p` and the plain REPL pass no bytes
anywhere rather than deciding not to draw them.

### 2. An image is drawn if and only if the intake saved and described it

A block whose note does not fit the response budget is already neither
saved nor described individually — the guard sizes `binaryNote` before
anything is written ([mcpresult.go:113](../../../cmd/mcpresult.go)) — and
such a block is not drawn. Drawing one would put a picture on the
operator's screen that appears nowhere in the session's record. One rule,
not a second budget.

### 3. The box comes from the picture, the clamp comes from the terminal

`image.DecodeConfig` gives the pixel dimensions without decoding the
picture; the rows follow from the aspect ratio against a ceiling, the
columns follow the rows, and `termimg.Fit` clamps the width below the
terminal's (ADR-0020 §2). `DecodeConfig` is also the validator: a block
whose MIME says `image/*` but whose bytes do not decode is not an image and
is not drawn. The payload is base64, whose alphabet holds no `ESC` and no
`BEL`, so bytes reaching the view layer cannot break out of the escape that
wraps them.

### 4. Two megabytes, decoded

A block whose decoded bytes exceed 2 MiB is saved and described as usual
and not drawn. The bound is on the decoded bytes because that is what the
measured cost tracks, and it is checked before any payload is built.

### 5. Nothing here changes what the model sees

The note, the path and the `view_image` route are as ADR-0005 left them.
The model's access still goes through a tool whose verdict the enforcers
take, with symlinks resolved first. This adds a second audience — the
operator — and gives it a channel that opens no file.

## Consequences

- An MCP server's screenshot appears on screen as it is taken; the model
  still has to ask.
- The symlink pre-plant is neither repaired nor worsened. What changes is
  that nothing new opens that path.
- A server can now put a picture on the operator's screen. It could
  already put bytes in the transcript, and the picture lands in a box this
  runtime declares, but the surface is new and is named here.
- The 2 MiB ceiling refuses some images silently: a warning per oversized
  block is a report rather than a control, and the picture stays reachable
  through `view_image`.
- **Implemented here after gem-agent**, which had the lane already. The
  decision was taken in both records at the same time so that neither
  runtime holds it alone, and the port carries the erase gem-agent's first
  real-terminal run made necessary (ADR-0020 decision 3).

## Alternatives considered

**A1. Widen `Tool.Run` to return structured content.** Rejected: the
string result is what the transcript stores, what resume replays and what
`RemoteError` carries — three subsystems changed for a fourth's
convenience, which is the unenumerated reach ADR-0020 §5 was refuted for
three times.

**A2. Let the view layer read the saved path.** Rejected — it is the
constraint ADR-0020 §5 earned, and the `os.Stat` short-circuit makes the
path a thing a server can choose.

**A3. Draw every image block, budget or not.** Rejected: it puts a picture
on screen that the session's record does not contain.

**A4. Bound by pixels rather than bytes.** Rejected as the primary bound:
the measured cost tracks bytes. The dimensions are read anyway for
decision 3, so a pixel bound can be added later on evidence.

**A5. Warn when an image is refused.** Rejected: a line per oversized
block is a report, not a control, and the operator's next action does not
change because of it.

**A6. Wait for gem-agent to implement and then port.** Accepted for the
code, rejected for the decision — which runtime writes it first is
scheduling, and a decision held on one side only is the asymmetry both
runtimes have been repaired for before.

## References

- ADR-0020 §5 (the three refuted drafts and the constraint), §2 (the
  declared box and the clamp); ADR-0005 (how an image reaches the model)
- gem-agent ADR-0090 — the same decision, and the cost measurement
- ADR-0015 / ADR-0016 — who may open a file
