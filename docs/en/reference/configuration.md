# Configuration and commands

Install, the config file, precedence, and the command table. Evergreen:
updated in place as keys and commands are added.

## Install

Build from source (`make build` → `dist/lagent`) and put the binary on
your PATH. Release archives and a Homebrew formula come with the first
release (RFP Phase 3).

## Config file

`~/.config/lagent/config.toml`. The shipped template is
[`config.example.toml`](../../../config.example.toml); the loader parses
it in a test, so the template and the built-in defaults cannot drift.
Unknown keys are errors (strict decode).

| Key | Default | Meaning |
|---|---|---|
| `[llm].provider` | `lmstudio` | which local server answers: `lmstudio`, `ollama` or `openai`. Selects only where the context length is detected from; every conversation goes through the OpenAI-compatible `chat/completions` endpoint |
| `[llm].base_url` | `http://localhost:1234/v1` | base URL of the OpenAI-compatible API |
| `[llm].model` | `google/gemma-4-26b-a4b-qat` | model id as the server lists it |
| `[llm].api_key` | (unset) | bearer token for a server that requires one; local servers need none |
| `[model].context_window` | `0` | context window in tokens; `0` detects it from the provider at startup (LM Studio `/api/v0/models`, Ollama `/api/show`; `openai` needs an explicit value) |

## Precedence

flags > `LAGENT_*` environment > config file > built-in defaults.

| Environment variable | Key |
|---|---|
| `LAGENT_MODEL` | `[llm].model` |
| `LAGENT_BASE_URL` | `[llm].base_url` |
| `LAGENT_API_KEY` | `[llm].api_key` |

## Commands

| Command | Meaning |
|---|---|
| `lagent` | start the interactive session (RFP Phase 1; the scaffold exits with an error until the loop lands) |
| `lagent version` | print the version — the same line as `--version` |

| Flag | Meaning |
|---|---|
| `--version` | print the version and exit |
