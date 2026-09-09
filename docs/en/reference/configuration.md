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
| `[sandbox].enabled` | `true` | wrap `shell_exec` in sandbox-exec; the lane the model declares is enforced by the kernel. Off, every shell call is yours to approve |
| `[sandbox].read_lane_deny_exec` | (unset) | programs the read lane may not launch, added to the built-in list |
| `[sandbox].read_lane_prompts` | `false` | keep the approval prompt for read-lane commands too |
| `[agent].max_turns` | `50` | round budget per turn; a checkpoint interactively, a stop in `-p`; a hard cap of 3x |
| `[agent].shell_timeout_sec` | `120` | per-command timeout for `shell_exec` |
| `[agent].auto_approve` | `false` | start with auto-approve on: rule-tier Safe calls run unasked, Review and Block ask. **Ignored in `-p`** — only `--auto` arms it there. `/auto on|off` and shift+tab change it for the session |
| `[agent].read_only` | `false` | start with the lane ceiling in force: nothing outside the session scratch may change. Applies in `-p` too; `--read-only` / `--writable` override it per run, `/readonly on|off` per session. Nothing in the runtime lowers it |
| `[mcp].enabled` | `true` | `false` disables every MCP server, global and project; `--mcp on|off` overrides per run |
| `[mcp].call_timeout_sec` | `60` | per-call timeout for an MCP tool |
| `[mcp].exclude` | (unset) | servers or single functions this session does not have; a project's `.lagent.toml` may add to it, never remove |
| `[mcp].advertise` | `deferred` | what the model is shown of the connected servers: `deferred` gives it a catalog in the runtime facts and advertises a server's tools once it calls `mcp_load` with the server name; `all` advertises every tool from the start (the measurement baseline) |
| `[mcp].preload` | (unset) | servers advertised from the start under `deferred`; a `--allow mcp__<server>__*` grant preloads that server for the run |
| `[tui].theme` | `auto` | `auto`, `dark`, `light`, or `plain` |
| `[tui].language` | `auto` | `auto` (from `LC_ALL` / `LC_MESSAGES` / `LANG`), `ja`, or `en` |
| `[tui].show_thoughts` | `true` | show the server's reasoning deltas in the live area; display-only |
| `[approval].pin_trusted_files` | `true` | trust is given to content: a trusted project's agent-facing files are pinned by digest and a changed one asks again |
| `[approval].tools` | (unset) | per-tool policy: `"always"` (always ask; a floor auto-approve cannot lift) or `"never"` (never ask; blocked shell patterns still ask). `--allow` does the same for one run |
| `[approval].trusted_projects` | (unset) | projects whose own `.lagent.toml` may remove approvals; the same decision as the startup trust prompt |

The two session modes are independent axes: `auto_approve` decides who
answers the gate, `read_only` decides what the session may reach at
all. Both can be on. A project's `.lagent.toml` carries neither — it
holds `[approval.tools]` and `[mcp].exclude` only, so a cloned
repository cannot switch the gate off.

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
| `/mcp`, `/mcp load <server>`, `/mcp reload` (in a session) | list the servers with their loaded state; advertise one server's tools by hand; reconnect |
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
