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

> **Status: experimental (lab-series).** The Phase 1 core of the
> [RFP](docs/en/lagent-rfp.md) is in: the loop, the tools, the sandbox
> lanes, approval, MCP, sessions, the TUI. Releases carry a signed and
> notarized darwin/arm64 archive. The side-by-side measurement against
> gem-agent did not become a comparison (RFP §4): the local model answers
> in one round where Gemini works through a tool loop, and that finding
> is where Phase 2 starts.

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

## Quickstart

Start LM Studio, load the model, and run lagent in the project
directory:

```bash
cd ~/work/my-project
lagent
```

The first start asks whether to trust the project's own AGENTS.md /
CLAUDE.md / .mcp.json. Mutating tools ask before running; `--auto` lets
rule-tier Safe calls run unasked; `-p "…"` runs one prompt and exits.
`/help` lists the slash commands.

## What it does

- **Tools:** `list_files`, `list_tree`, `search_files`, `read_file`,
  `file_info`, `view_image`, `write_file`, `edit_file`, `shell_exec`,
  `ask_user`, and
  every tool of the MCP servers in `.mcp.json` — shown to the model as a
  catalog, and advertised per server once it calls `mcp_load` (a local
  model cannot afford 243 schemas on every turn; `[mcp].preload` and
  `[mcp].advertise = "all"` are the operator's levers).
- **Confinement:** file tools stay inside the project (and the session
  work directory), and a credential file — `.env`, a private key, a
  token store — is read only when you approve it, every time, never
  unattended: those reads run in a sandboxed child that cannot open
  credential material at all, so the list raises the question and the
  kernel is what refuses; `shell_exec` runs under `sandbox-exec` in the lane it
  declares — read runs unasked (inspection, and Go builds and tests,
  whose cache lives in the session scratch), write and operator ask.
- **Hooks:** `[[hooks.pre_tool_use]]` runs your guard script before a
  model tool call, on Claude Code's contract, so the same script that
  guards Claude Code guards this runtime; a deny is final, whatever the
  approval mode, and the reason goes back to the model (ADR-0012).
  `session_start`, `user_prompt_submit` and `session_end` hooks share
  the mechanism: their output reaches the model as quoted data, and a
  prompt hook can refuse a prompt (ADR-0014).
- **Memory:** `/remember <name> <fact>` keeps a short fact for every
  later session in this project (`global` for every project); the model
  can propose one with `save_memory`, which asks you. Memories are
  recalled in the runtime facts, the place this model acts on, so a
  memory that names a file or a command works as a pointer (ADR-0013).
- **Sessions:** a JSONL transcript per session; `--continue` and
  `--resume`; usage records in the shape
  [gem-usage-lens](https://github.com/nlink-jp/gem-usage-lens) reads for
  both runtimes.
- **Skills:** Claude Code's `SKILL.md` format, read as-is from
  `~/.config/lagent/skills/<name>/` and a trusted project's
  `.claude/skills/<name>/`. One line per skill tells the model what
  each is for; it loads one with `load_skill`, and you invoke one by
  hand with `/skill <name>` (ADR-0011).
- **Not here:** web search and fetch, media uploads, audit-log export,
  history compaction — see the RFP and
  [ADR-0002](docs/en/adr/0002-features-not-reproduced.md).

## Attachments

`@<path>` attaches a project file or directory; `@<image>` attaches an
image from anywhere (absolute or `~` paths), and an image path dropped
on the terminal attaches without the `@` — escaped spaces included.
A reference that cannot be read is reported to you and told to the
model.

## Install

Apple Silicon Mac, via the nlink-jp Homebrew tap (the signed and
notarized release archive, installed as-is):

```bash
brew tap nlink-jp/tap
brew install nlink-jp/tap/lagent
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
