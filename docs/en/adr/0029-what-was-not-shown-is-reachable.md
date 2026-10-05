# ADR-0029: what the runtime did not show is reachable and countable

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-10-05) — ported from gem-agent ADR-0096 Part A; implemented |
| Date | 2026-10-05 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | gem-agent ADR-0096, accepted by the operator: an operator's analysis-quality report traced partial results taken as whole to where the runtime puts what it kept. The spill intake, `read_file`, `shell_exec`'s output writer and the `search_files` walk are mechanisms both runtimes share, so the defects are this runtime's too |
| Relates to | gem-agent ADR-0096 (the report, the verification of its findings, the independent design review and the probe that decided Part B), gem-agent ADR-0058 (the spill), gem-agent ADR-0052 §2 (skips are reported) |

## Context

gem-agent checked the report's runtime findings against its code and
found four defects in mechanisms this runtime ported unchanged:

- A spilled MCP text block showed its **head only** (800 runes), so
  metadata a server appends — `"truncated": true`, a row total — was never
  in front of the model.
- The spill notice said `read_file <path>`, but `read_file` windows by
  line and stops at 200 KB: the tail of a spilled single-line result past
  200 KB was out of reach of the tool the notice named.
- `shell_exec` kept the **first** 20 KB of output and saved nothing; a
  script's closing totals were lost.
- `search_files` skipped files over 2 MB, binary and image files, and
  files and directories it could not read, **without counting them**.

The code here is the same (`cmd/mcpresult.go`, `internal/tools`,
`internal/bounded`), so the defects are the same.

## Decision

Part A of gem-agent ADR-0096, as it is there:

1. **The spill preview is head and tail** — the first 600 and last 200
   runes, both cut on rune boundaries — and the notice names the byte
   spans shown and the `read_file` offset/length route to the rest.
2. **`read_file` reads by bytes**: `offset` (negative from the end) and
   `length` (default `OutputCap`, at most `readCap`), exclusive with the
   line window, moved to rune boundaries, with a note naming the bytes
   returned. A plain read cut at `readCap` names `offset=N` to read on.
3. **`shell_exec` keeps head and tail and saves the whole**: past
   `OutputCap` the model gets the first three quarters and the last
   quarter, and the stream is saved to the session work directory (up to
   32 MiB) by the runtime, which holds the pipe — no lane's reach changes.
   The operator lane is not saved: it may read credentials, and a copy in
   the work directory would be readable without approval. Neither is any
   lane when the shell runs without the sandbox, where every lane can read
   credentials. The saved file is private (`0600`). The writer is
   `bounded.HeadTail`; the lane hint reads the command's own text, not the
   runtime's note after it.
4. **`search_files` counts every file it did not search**, by reason —
   over 2 MB (named, up to five), binary, image, unreadable, unlistable
   directory — and refusals past the five named.

`internal/tools/bytewindow.go`, `shellout.go`, `bounded.HeadTail` and
the changes to `mcpresult.go`, `tools.go` and `nav.go` are ported from
gem-agent at the commits that made them, with the same tests.

What is not ported: Part B (declaring partial views outside the nonce
tag). gem-agent measured it before deciding — real binaries against a
stub MCP server, three arms — and every arm, the head-only one included,
answered correctly; it was not taken there and there is nothing to port.
gem-agent's `tools/coverageprobe` is not ported either: this runtime has
no probes.

## Consequences

- The tail of a spilled result is in front of the model, and every byte
  of it is reachable by the tool the notice names.
- `shell_exec` output stops being lost past 20 KB, except in the operator
  lane, which says so; long outputs leave a file in the work directory.
- "No match" from `search_files` says what was not searched.
- The notes stay in the tool's text, in today's position.

## References

- gem-agent ADR-0096 — the decision, the verification and the measurement
- `cmd/mcpresult.go`, `internal/tools/bytewindow.go`, `internal/tools/shellout.go`,
  `internal/tools/nav.go`, `internal/bounded/bounded.go`
