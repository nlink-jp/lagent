# ADR-0024: text from outside the runtime is made inert before the terminal sees it

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-09-29) — implemented |
| Date | 2026-09-29 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | gem-agent ADR-0093: the model's text reached the terminal with its escape sequences intact, and an independent pre-release review of gem-agent found it. AGENTS.md: a defect in a mechanism both runtimes have is fixed in both, and this runtime's `internal/tui` has the same path — `case TextDelta` appends the raw chunk — so it had the same defect. The operator approved the port |
| Relates to | gem-agent ADR-0093 (the decision and its two review passes, ported here), RFP §2 "Input / Output" (**amended here**), [ADR-0002](0002-features-not-reproduced.md) (no diagrams, so no box-art hold), [ADR-0020](0020-inline-images-declare-their-height.md) (the declared-row account a stray image escape breaks) |

## Context

gem-agent ADR-0093 holds the analysis, the alternatives and the two review
passes; this record does not repeat them. What is this runtime's own is the
measurement and the differences.

### Measured here, before the change

gem-agent's `tools/escprobe` was copied into this tree for the run and not
kept (this runtime carries no probes). It drives this runtime's real model
under the real inline program in a private tmux 3.7c server with
`set-clipboard on`, on the same five channels — a streamed reply live and
flushed, a thought, a tool event, an approval dialog — and reads the terminal
back. Against v0.10.3, **61 of 95 deliveries acted** — the same pattern as gem-agent's: the title set and
the clipboard buffer written on every channel, the screen erased, the cursor
moved, a CR hiding the start of a line and an unterminated OSC the rest of it
(in the approval dialog, the command being approved), and the character
references `&#27;`, `&#x1b;`, `&#13;` decoded into controls by the Markdown
renderer on the flushed reply.

### The other entrances

The plain REPL and `-p` write the model's deltas to stdout and tool events,
approval prompts and the ask dialog to stderr, as gem-agent does; the RFP
says stdout carries model text only.

## Decision

The mechanism is gem-agent ADR-0093's, ported from gem-agent v0.85.1
(`8d7c780`) with the provenance line ADR-0001 asks for:

1. **Control characters are removed, not sequence bodies** — C0 other than
   tab and newline, DEL, C1, the bidi embeddings, overrides and isolates;
   invalid UTF-8 becomes U+FFFD (`internal/inert`). Removing a whole
   unterminated sequence would hide the rest of an approval command exactly
   as the terminal does.
2. **Once, at the TUI's ingress**: every string of every message declared in
   `internal/tui` is rewritten by reflection at the top of `Update`, and every
   `Options` callback and text field that is shown is wrapped in `New`. The
   operator's argv first message is not rewritten.
3. **The Markdown renderer's output is held to what it writes itself**: SGR
   for the dark and light styles, closed with a reset at the reply's end, and
   nothing for the plain style — because goldmark decodes character
   references into real controls after the ingress has seen only ASCII.
4. **The plain REPL and `-p`**: stdout and stderr are wrapped once, at the top
   of `runREPL`, when — and only when — the stream is a terminal (`ls -q` /
   `ls -w`). A pipe or a file still gets every byte.
5. **Held by tests**: the message types read off `msgs.go`, every string
   filled hostile and driven through `Update`, the flush and `View`; the
   walker's kinds; the `Options` fields; character references through the
   real renderer in every theme; a renderer that leaves a style open; the
   importers of `internal/inert` pinned by import path; the plain streams
   wired before the first print.

### What differs from gem-agent

- **No box-art hold.** This runtime draws no diagrams (ADR-0002), so the one
  transform is glamour; `inertArt` is not carried.
- **No `Output` message and no riskbook, compaction, picture or cell-aspect
  callbacks** — they are gem-agent features ADR-0002 does not reproduce, so
  the tests' message and `Options` registries are this runtime's own lists.

### What stays outside, and why that is accepted

gem-agent ADR-0093 §4, unchanged: `-p … | cat` hands raw bytes to a terminal
by the operator's choice; an SGR decoded from `&#27;[…m` can colour or
conceal text inside the model's own reply and is closed at its end;
`LAGENT_MCP_STDERR=1`, an opt-in debug switch, hands a server's stderr to the
terminal; this runtime has no telemetry exporter, so gem-agent's residual
there has no counterpart; what the operator types is not rewritten; an MCP
server writing to `/dev/tty` itself is code execution, not text shown. The
transcript stays verbatim.

## Consequences

- The measured table goes to zero: 0 of 95 deliveries act, on every channel
  probed and for every case, the character references included (the same
  temporary copy of the probe, run on this change).
- **The RFP's "stdout carries model text only" is amended**: to a pipe or a
  file, verbatim; to a terminal, with control characters removed.
- `internal/inert` and the TUI ingress join the mechanisms both runtimes
  share (AGENTS.md, both repositories).

## References

- gem-agent ADR-0093 and its measurements (`make escprobe` there).
- The knowledge base, security: "Making text inert at the ingress is not
  enough when a transform downstream decodes".
