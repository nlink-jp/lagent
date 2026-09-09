# lagent

Sandboxed CLI agent runtime on a **local LLM**, served over an
OpenAI-compatible API (LM Studio, Ollama). File read/write, sandboxed
shell commands and MCP servers, with mutating calls gated by the
operator.

lagent is a separate product line from
[gem-agent](https://github.com/nlink-jp/gem-agent), built on the same
design: the same auditable minimal loop (read / edit / shell / MCP /
approval), the same drop-in reading of a project's AGENTS.md / CLAUDE.md /
.mcp.json, the same session records. It exists to measure, on the same
scale as gem-agent, how far a local model carries an agent runtime in cost
(tokens, wall-clock time, turns) and effectiveness.

> **Status: experimental (lab-series).** Not released. The scaffold
> answers `--version`; the agent loop is development Phase 1 of the
> [RFP](docs/en/lagent-rfp.md).

Japanese: [README.ja.md](README.ja.md)

## Requirements

- macOS on Apple silicon (isolation is built on `sandbox-exec`)
- A local LLM server with an OpenAI-compatible API:
  [LM Studio](https://lmstudio.ai/) (the tested backend, model
  `google/gemma-4-26b-a4b-qat`) or Ollama
- No credentials

## Configuration

`~/.config/lagent/config.toml` — see
[config.example.toml](config.example.toml). Precedence: flags >
`LAGENT_*` environment > file > defaults; unknown keys are errors.

```toml
[llm]
provider = "lmstudio"        # lmstudio | ollama | openai
base_url = "http://localhost:1234/v1"
model    = "google/gemma-4-26b-a4b-qat"

[model]
context_window = 0           # 0 = detect from the provider
```

## Build

```bash
make build      # → dist/lagent
make test
make check      # vet + lint + test + docs mirror + release gate + build
```

## Documentation

- [docs/en/INDEX.md](docs/en/INDEX.md) — specification (RFP), reference,
  ADRs

## License

MIT
