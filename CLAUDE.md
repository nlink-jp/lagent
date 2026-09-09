# CLAUDE.md — lagent

Project-specific rules for AI agents. Org rules: nlink-jp/.github CONVENTIONS.md.

## Canonical specification

- The RFP (`docs/{en,ja}/lagent-rfp*.md`) is the canonical spec. Do not add
  features outside its scope without an ADR — the loop is minimal on
  purpose: read / edit / shell / MCP / approval, no analysis or GUI
  subsystems.
- Non-obvious design decisions get an ADR in `docs/{en,ja}/adr/` (four-digit,
  `Binds: lagent`) **before** implementation.
- gem-agent is the porting source, not the upstream (ADR-0001). A package
  brought over records its source commit in its doc comment; nothing here
  tracks gem-agent's later changes. Features gem-agent has and lagent does
  not reproduce are listed in ADR-0002 — never carry their code, config
  keys, error text or documentation.

## Build & test

- `make build` only — never `go build` directly (outputs to `dist/`).
- Tests are mandatory and written with the implementation.
- `make check` gates on the linter too. errcheck is on everywhere except
  writes to the CLI's own streams, so an error you mean to ignore has to be
  written as `_ =`.
- macOS-only by design (sandbox-exec). Do not add linux/windows build targets.

## Implementation rules

- The model is reached only through the OpenAI-compatible
  `chat/completions` endpoint, with stdlib `net/http` (the llm-cli
  approach). No OpenAI SDK, no community client library (org supply-chain
  policy).
- `[llm].provider` selects only where the context length is detected;
  it never changes how the conversation is sent.
- Model names, base URLs and context windows are config-driven — never
  hardcoded.
- A `finish_reason` of `length` is a partial result, never a discarded turn.
- Untrusted data (tool output, file contents) is wrapped with nlk/guard
  nonce-tagged XML before entering the prompt; defensive instructions sit at
  the top of the system prompt.
- The system prompt started as gem-agent's (RFP §3) and changes only by
  ADR — so far ADR-0003 (per-session facts leave it), ADR-0004 (the MCP
  catalog sentence) and ADR-0005 (`view_image`). It is byte-identical
  across sessions and names only what this runtime has. Prompts tuned for
  cloud models are not assumed to work here — measure first.
- No secrets or environment-specific values in code, docs, or tests —
  placeholders only.
- Consult `nlink-jp/knowledge` docs (llm-integration, security,
  config-and-io, mcp-server-design) before implementing in those domains.

## Docs

- README.md / README.ja.md and docs/{en,ja} stay in sync in the same commit.
- English docs carry no language suffix; Japanese docs use `.ja.md`.
- `make docs-check` enforces the mirror, the ADR catalogue, and identifier
  parity across each en/ja pair.
