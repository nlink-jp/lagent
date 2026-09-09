# ADR-0005: Images reach the model — dropped paths attach, view_image returns, and a missing file is said

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-10 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | A screenshot's path pasted into the input (the shape a drop on the terminal produces) attached nothing; the model said it had received the image and then described one from thin air |

## Context

The operator dropped a screenshot on the terminal. macOS pastes the
file's absolute path with every space backslash-escaped; the operator
sent it in backticks. Nothing attached: the `@` grammar is the only
attachment route, and even an `@` would have stopped at the first
escaped space. The model then answered as if it had the image, and on
the next question described a Python error it had never seen.

The same input to gem-agent (Gemini 3.8 flash) ended in a correct
description, reached by seven tool calls: `ls` on the path, a
`view_image` that failed outside the project, a write-lane `cp` of the
operator's file into the project, `view_image` on the copy, and `rm`.
The model worked around the missing attachment because it had a tool
to look at images with. lagent's Phase 1 scope had dropped
`view_image`, so Gemma had no route and no signal that the image was
absent.

Three gaps, then: the drop convention is not read; the model has no
way to look at an image in the project or the work directory (an MCP
screenshot tool's output, for one); and a reference that failed to
attach is reported to the operator but not to the model.

## Decision

1. **A bare image path attaches.** `mention.Refs` takes, beside `@`
   references, any absolute or `~` path ending in an image extension,
   written without an `@`: at the start of the text or after
   whitespace or an opener (quote, backtick, bracket), ending at
   whitespace or a stopper, with backslash-escaped spaces read as
   spaces. `@` references accept the escapes too. Only images are taken
   bare — a text file's path in prose is a mention; the drop of an
   image is the operator's intent, and images already have the
   operator-typed exception that lets them come from outside the
   project.
2. **`view_image` returns.** The built-in from the porting source, as
   it was: read-only, project- and work-directory-confined, sniffed
   bytes; the agent attaches the pixels to the tool result, and the
   backend sends them as the image part that follows it. The system
   prompt names it, and the MCP intake's note points at it again for
   the images a server saved.
3. **A reference that did not attach is told to the model.** The
   agent adds an attachment of kind `missing` for every problem
   `mention.Expand` reported; at send time it renders outside the
   nonce tag as `[not attached: <ref> — <reason>]` — lagent's words,
   about a file, never the file's content. The operator's warning is
   unchanged.

## Consequences

- The dropped screenshot is in the first request, as one attachment,
  with no tool call, no copy and no delete of the operator's file —
  on either runtime, once the source adopts the first point.
- The Phase 1 tool roster is nine built-ins. The RFP's list of eight
  is amended by this record; ADR-0002 is unaffected (view_image is not
  a provider feature).
- A model that is told a file is absent can say so. Whether Gemma 4
  does is the next measurement; the runtime's part is to make the
  absence a fact in the conversation rather than a silence.
- A text file's bare path still attaches nothing; `@` remains the
  route for those.

## Alternatives considered

- **Attach any bare path** — rejected: paths in prose are common, and
  a directory or a large file attached by accident is a cost the
  operator did not ask for. Images are the case with a drop
  convention behind them.
- **Leave `view_image` out and rely on attachments** — rejected: the
  model has no way to attach, so an image a tool saved during the
  session (an MCP screenshot) would be unreachable.
- **Rewrite the operator's text to add the `@`** — rejected: the
  typed text is recorded as typed and echoed as typed; the reference
  is found, not inserted.

## References

- ADR-0004 — the MCP intake and its saved images
- gem-agent ADR-0012 (`view_image`, image attachments) at the pinned
  commit — the design the second point restores
