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
| `LAGENT_PROVIDER` | `[llm].provider` |
| `LAGENT_BASE_URL` | `[llm].base_url` |
| `LAGENT_MODEL` | `[llm].model` |
| `LAGENT_API_KEY` | `[llm].api_key` |

## Commands

| Command | Meaning |
|---|---|
| `lagent` | start the interactive session in the current directory (the TUI on a terminal, a plain REPL on pipes) |
| `lagent "<first message>"` | send the argument as the first turn, then converse |
| `lagent sessions` | list this project's sessions (id, when, preview) |
| `lagent trust` | show or change the project's trust and its pins |
| `lagent workdirs` | list earlier sessions' work directories; `workdirs clean` removes them |
| `lagent version` | print the version — the same line as `--version` |

| Flag | Meaning |
|---|---|
| `--version` | print the version and exit |
| `-p`, `--prompt` | one-shot: run this prompt and exit; mutating tools are denied unless listed in `--allow` or `--auto` is set |
| `--auto` | start in auto-approve mode: rule-tier Safe calls run unasked |
| `--allow` | tools that never ask this run: names or `mcp__server__*` prefixes |
| `--read-only` / `--writable` | cap the session at the read lane, or state that it is not capped |
| `-c`, `--continue` | resume this project's most recent session |
| `--resume` | resume a specific session id |
| `--model` | override `[llm].model` for this run |
| `--mcp on|off` | override `[mcp].enabled` for this run |
| `--no-sandbox` | disable the sandbox-exec wrapper (debugging only, unsafe) |
| `--config` | config file path |
