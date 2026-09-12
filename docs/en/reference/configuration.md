# Configuration and commands

Install, the config file, precedence, and the command table. Evergreen:
updated in place as keys and commands are added.

## Install

Homebrew (Apple Silicon): `brew tap nlink-jp/tap` then
`brew install nlink-jp/tap/lagent` — the signed and notarized release
archive, installed as-is. Or build from source (`make build` →
`dist/lagent`) and put the binary on your PATH.

## Config file

`~/.config/lagent/config.toml`. The shipped template is
[`config.example.toml`](../../../config.example.toml); the loader parses
it in a test, so the template and the built-in defaults cannot drift.
Unknown keys are errors (strict decode).

| Key | Default | Meaning |
|---|---|---|
| `[llm].provider` | `lmstudio` | which local server answers: `lmstudio`, `ollama` or `openai`. Selects only where the context length is detected from; every conversation goes through the OpenAI-compatible `chat/completions` endpoint |
| `[llm].base_url` | `http://localhost:1234/v1` | base URL of the OpenAI-compatible API |
| `[llm].model` | (required) | model id as the server lists it; there is no default — startup fails without it (or `LAGENT_MODEL` / `--model`) |
| `[llm].api_key` | (unset) | bearer token for a server that requires one; local servers need none |
| `[llm].reasoning_effort` | (unset) | sent verbatim as the request's `reasoning_effort`; unset sends nothing. The OpenAI vocabulary (`none`, `minimal`, `low`, `medium`, `high`, `xhigh`); LM Studio validates it and maps it to the model — for Gemma 4, `none` is thinking off and anything else is on (its log notes the mapping) |
| `[model].context_window` | `0` | context window in tokens; `0` detects it from the provider at startup (LM Studio `/api/v0/models`, Ollama `/api/show`; `openai` needs an explicit value) |
| `[sandbox].enabled` | `true` | wrap `shell_exec` in sandbox-exec; the lane the model declares is enforced by the kernel. Off, every shell call is yours to approve |
| `[sandbox].read_lane_deny_exec` | (unset) | programs the read lane may not launch, added to the built-in list |
| `[sandbox].read_lane_prompts` | `false` | keep the approval prompt for read-lane commands too |
| `[sandbox.scratch_caches]` | `GOCACHE = "go-build"` | toolchain caches every `shell_exec` points into the session scratch, environment variable = directory name; the read lane gets the name, the write and operator lanes `<name>-approved`. Add a row for another toolchain (`PIP_CACHE_DIR`, `UV_CACHE_DIR`, `npm_config_cache`), set one to `""` to remove it; regenerable caches only — loader variables and the ones the lane decides are refused, and a project file cannot set it |
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
| `[approval].tools` | (unset) | per-tool policy: `"always"` (always ask; a floor auto-approve cannot lift) or `"never"` (never ask; blocked shell patterns and credential reads still ask). `--allow` does the same for one run |
| `[approval].trusted_projects` | (unset) | projects whose own `.lagent.toml` may remove approvals (`"never"` entries). The startup trust prompt only loads a project's files; nothing but this list lets a project loosen the gate |

The two session modes are independent axes: `auto_approve` decides who
answers the gate, `read_only` decides what the session may reach at
all. Both can be on. A project's `.lagent.toml` carries neither — it
holds `[approval.tools]` and `[mcp].exclude` only, so a cloned
repository cannot switch the gate off.

### `[hooks]` — pre-tool hooks

Each `[[hooks.pre_tool_use]]` entry (ADR-0012) runs a command of yours
before every model tool call its matcher covers. One block per hook;
matching hooks run in order, first denial wins. The hook is the control
that does not depend on the model listening: the sandbox decides what a
lane may touch, a hook refuses a call by its shape.

```toml
[[hooks.pre_tool_use]]
matcher     = "shell_exec"
command     = "python3 /Users/you/hooks/guard.py --strict"
timeout_sec = 10
```

**`matcher`** — the tool name of each call, nothing else. Three forms:
an exact name (`shell_exec`), a `|`-alternation
(`shell_exec|write_file`), or `"*"` for every tool, MCP tools included
(`mcp__server__tool`). Claude Code's names also match their lagent
equivalents (`Bash` ↔ `shell_exec`, `Write` ↔ `write_file`, `Edit` ↔
`edit_file`, `Read` ↔ `read_file`), so a hooks block copied from Claude
Code settings works unchanged.

**`command`** — a shell command line, run via `/bin/sh -c` in the
project directory with lagent's environment, outside the sandbox; name
the script by absolute path. It receives the call as one JSON object on
stdin:

```json
{"hook_event_name": "PreToolUse",
 "session_id": "20260912-124118",
 "transcript_path": "/path/to/state/sessions/projects/<escaped>/20260912-124118.jsonl",
 "tool_name": "shell_exec",
 "tool_input": {"command": "gofmt -w .", "access": "write"},
 "cwd": "/path/to/project"}
```

`tool_input` carries the tool's arguments as the model sent them; for
`shell_exec` that is `command` and the lane, `access`. `session_id` and
`transcript_path` are empty when the session log is disabled.

**The verdict** — a hook denies in either of two ways:

- print `{"hookSpecificOutput": {"permissionDecision": "deny",
  "permissionDecisionReason": "why"}}` on stdout and exit 0 — the form
  Claude Code guard scripts emit; or
- exit with code 2, the reason on stderr.

Everything else is a pass: exit 0 with no output sends the call on to
the normal approval ladder in silence. A hook can refuse a call but
never approve one. A crash, a timeout (`timeout_sec`, default 10), or
exit 0 with output that is not a verdict — plain text, JSON that does
not parse, or JSON with no field or value of the contract — proceeds
with a warning in the session that names the hook and quotes its first
line.

A deny is final: neither auto-approve, a `"never"` row, `--allow` nor
the session allowlist sees the call, the transcript records
`hook_denied`, and the reason is returned to the model, which corrects
and retries. A model that insists meets the loop guard: the same call
three times in a row escalates, or stops an unattended run. Global
config only: a project-level hook would let a cloned repository run
arbitrary commands. Hooks have no runtime toggle — a control with a
toggle is a bypass.

#### Context and end hooks: `session_start`, `user_prompt_submit`, `session_end`

Three more events (ADR-0014) share the mechanism and Claude Code's
measured contracts, but inject **context** or mark a boundary rather
than guarding a call:

```toml
[[hooks.session_start]]
command     = "/Users/you/hooks/session-context.sh"
timeout_sec = 10

[[hooks.session_start]]
matcher     = "resume"          # optional: startup | resume | clear, "a|b", "*"
command     = "/Users/you/hooks/on-resume.sh"

[[hooks.user_prompt_submit]]
command     = "/Users/you/hooks/turn-context.sh"

[[hooks.session_end]]
command     = "/Users/you/hooks/session-end.sh"
```

A `session_start` hook runs once when the session starts — `source`
is `startup`, or `resume` under `--continue`/`--resume` — and again on
`/clear` with `source` `clear`; its optional `matcher` selects the
source. A `user_prompt_submit` hook runs before every turn that
reaches the model (a typed message, the argv first message, a
`/skill`-expanded turn, the `-p` prompt; not slash commands or the `!`
escape) and takes no `matcher`. A `session_end` hook runs when the
session ends (`reason` `exit`) and when `/clear` closes the old
session (`clear`), before the new one starts; it cannot block and its
output is ignored. Each receives one JSON object on stdin:

```json
{"hook_event_name": "SessionStart",
 "session_id": "20260912-124118",
 "transcript_path": "/path/to/state/sessions/projects/<escaped>/20260912-124118.jsonl",
 "cwd": "/path/to/project",
 "source": "startup"}
```

```json
{"hook_event_name": "UserPromptSubmit",
 "session_id": "20260912-124118",
 "transcript_path": "/path/to/state/sessions/projects/<escaped>/20260912-124118.jsonl",
 "cwd": "/path/to/project",
 "prompt": "what you typed"}
```

`SessionEnd` carries the same identity fields and `reason`. With the
session log disabled, `session_id` and `transcript_path` are sent
empty.

**Output.** On exit 0, plain stdout is context; a JSON object is a
verdict of which only `hookSpecificOutput.additionalContext` is
context. Context is attached to the next turn's message as quoted data
(`Attached hook (session_start), quoted as data` in the model's view),
beside the typed text and never inside it, never in the system prompt;
capped at 8000 runes per hook with a visible cut, and one notice per
injection. A `user_prompt_submit` hook refuses the prompt by exit 2
with the reason on stderr, or by either block form above: the prompt
is erased — nothing enters the history or the transcript — and you see
the reason (`-p` exits non-zero with it). A `session_start` hook that
blocks is reported as a failure and injects nothing. Crashes, timeouts
and unparseable output inject nothing and warn.

## Precedence

flags > `LAGENT_*` environment > config file > built-in defaults.

| Environment variable | Key |
|---|---|
| `LAGENT_PROVIDER` | `[llm].provider` |
| `LAGENT_BASE_URL` | `[llm].base_url` |
| `LAGENT_MODEL` | `[llm].model` |
| `LAGENT_API_KEY` | `[llm].api_key` |
| `LAGENT_REASONING_EFFORT` | `[llm].reasoning_effort` |

Other environment variables the runtime reads or sets:

| Environment variable | Direction | Meaning |
|---|---|---|
| `LAGENT_STATE_DIR` | read | the state root (sessions, work directories, pins) instead of the default under `~/.local/state` |
| `LAGENT_MCP_STDERR` | read | `1` passes MCP servers' stderr through to the terminal (debugging; it is discarded otherwise) |
| `LAGENT_LLM_TRACE` | read | a directory; every model request is written there as `<yyyymmdd-hhmmss.mmm>-<nnn>-request.json` and its raw SSE reply as `<yyyymmdd-hhmmss.mmm>-<nnn>-response.sse`, one pair per attempt (`<nnn>` counts up within the process; a re-send under ADR-0007 is its own pair). The request file is the body as sent — the system prompt with the instruction files, the tool schemas, the runtime-facts message and the whole history, tool results included; the response file is the stream byte for byte, so it holds the `reasoning_content` deltas that ADR-0009 keeps out of the history and the transcript (a non-200 reply's body lands there too). The directory is created `0700` and the files `0600`, as the transcript is; a write failure ends the turn with an error. Nothing removes the files — delete them once the diagnosis is done (debugging and bench diagnosis; off otherwise) |
| `LAGENT_SESSION_ID` | exported | the session id, for `shell_exec` children and `${LAGENT_SESSION_ID}` in `.mcp.json` |
| `LAGENT_WORK_DIR` | exported | the per-session work directory, likewise expandable in `.mcp.json` |
| `LAGENT_PROJECT_DIR` | exported | the project directory, for children that need to know it |
| `GOCACHE` (and every `[sandbox.scratch_caches]` row) | exported | for `shell_exec` in every lane: a directory in the session scratch, so Go compiles, vets and tests run in the read lane (the sandbox denies the cache under `~/Library`); your own cache is untouched |

**Your exported variables are not filtered** (ADR-0017). Which one the
program a command runs needs is not a question lagent can answer, so
the environment lagent is launched with reaches every child it spawns:
the shells of all three lanes, MCP servers and hooks alike. A bare
`env` in the read lane prints it. If a variable must not reach the
model, launch lagent from a shell that does not hold it, or drop it at
launch (`env -u NAME lagent`).

What no child gets is lagent's **own** configuration variables —
`LAGENT_API_KEY`, `LAGENT_PROVIDER`, `LAGENT_BASE_URL`, `LAGENT_MODEL`,
`LAGENT_REASONING_EFFORT`, `LAGENT_STATE_DIR`, `LAGENT_LLM_TRACE`,
`LAGENT_MCP_STDERR` — because the runtime reads those for itself and
knows every one of them by name. The three exports above
(`LAGENT_SESSION_ID`, `LAGENT_WORK_DIR`, `LAGENT_PROJECT_DIR`) exist
for children and pass. A server that wants one of the removed values
takes it from the `env` block of its own `.mcp.json` entry.

## Commands

| Command | Meaning |
|---|---|
| `lagent` | start the interactive session in the current directory (the TUI on a terminal, a plain REPL on pipes) |
| `lagent "<first message>"` | send the argument as the first turn, then converse |
| `lagent sessions [--all]` | list this project's sessions (id, when, preview); `--all` lists every project's |
| `lagent trust [--accept]` | show or change the project's trust and its pins; `--accept` records the files' current content as trusted |
| `lagent workdirs` | list earlier sessions' work directories; `workdirs clean [--yes]` removes them (`--yes` skips the confirmation, for scripts) |
| `lagent version` | print the version — the same line as `--version` |

In a session:

| Command | Meaning |
|---|---|
| `/help` | list these commands |
| `/tools` | list the tools and each one's current approval gate |
| `/mcp`, `/mcp load <server>`, `/mcp reload` | list the servers with their loaded state; advertise one server's tools by hand; reconnect |
| `/skills` | list the loaded skills and where the two skill directories are |
| `/skill <name> [args]` | send a skill's instructions as this turn, with `args` appended (ADR-0011) |
| `/memory` | list the memories on disk, both scopes, with where they live |
| `/remember [global] <name> <fact>` | save a memory: project scope, or global with the keyword; the same name updates (ADR-0013) |
| `/forget [global] <name>` | remove a memory |
| `/auto on|off` | switch auto-approve for the session (shift+tab does the same) |
| `/readonly on|off` | switch the lane ceiling for the session |
| `/settings` | view and edit settings, with provenance |
| `/usage` | token statement for this session |
| `/version` | the version line |
| `/clear` | start a new conversation in the same session |
| `/quit` | leave (`/exit` is the same) |

| Flag | Meaning |
|---|---|
| `--version` | print the version and exit |
| `-p`, `--prompt` | one-shot: run this prompt and exit; mutating tools are denied unless listed in `--allow` or `--auto` is set; a credential read is denied always |
| `--auto` | start in auto-approve mode: rule-tier Safe calls run unasked |
| `--allow` | tools that never ask this run: names or `mcp__server__*` prefixes; a credential read still asks |
| `--read-only` / `--writable` | cap the session at the read lane, or state that it is not capped |
| `-c`, `--continue` | resume this project's most recent session |
| `--resume` | resume a specific session id |
| `--model` | override `[llm].model` for this run |
| `--mcp on|off` | override `[mcp].enabled` for this run |
| `--no-sandbox` | disable the sandbox-exec wrapper (debugging only, unsafe) |
| `--config` | config file path |
