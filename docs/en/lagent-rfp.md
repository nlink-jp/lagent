# RFP: lagent

> Generated: 2026-09-10
> Status: Draft

## 1. Problem Statement

There is no coding-agent runtime that runs on a local LLM alone, with no
cloud API. gem-agent is bound to Vertex AI Gemini, so it cannot serve
offline environments, confidential projects, or work where API spend is
unwanted. lagent provides the same auditable minimal loop as gem-agent
(read / edit / shell / MCP / approval) on a local LLM server with an
OpenAI-compatible interface (target: LM Studio, model Gemma 4 26B A4B QAT).

**Purpose:** measure how far a local LLM can carry an agent runtime, in
cost (tokens, wall-clock time, turns) and effectiveness (task completion),
on the same scale as gem-agent.

**Target user:** the developer, for now. Experimental; distribution is not
assumed. The operator runs LM Studio or Ollama on macOS themselves.

**Positioning:** a separate product line from gem-agent. gem-agent's
technology (design, pure packages) is the porting source, but this is not
a fork. Not reproducing Vertex-specific features is accepted as the spec.

## 2. Functional Specification

### Commands / API Surface

gem-agent's surface, narrowed to what the experiment needs.

| Command / flag | Meaning |
|---|---|
| `lagent` | start the interactive TUI |
| `lagent "<first turn>"` | send the positional argument as turn one, then converse |
| `lagent -p "<prompt>"` | one-shot mode; model text only on stdout |
| `--continue` | resume from the previous session's transcript |
| `--auto` | auto-approve calls the rule tier classifies as Safe |
| `--version` | version (from `git describe`) |

As built, Phase 1 also has `--resume`, `--allow`, `--read-only` /
`--writable`, `--model`, `--mcp`, `--config`, `--no-sandbox` and the
`sessions` / `trust` / `workdirs` / `version` subcommands;
`reference/configuration.md` is the current table.

Phase 1 built-in tools: read_file / write_file / edit_file / list_files /
list_tree / search_files / file_info / shell_exec / ask_user. MCP server
tools are added by connecting from `.mcp.json`. Amended by ADR-0005:
`view_image` joins the list, and ADR-0004 adds `mcp_load`.

### Input / Output

- stdout carries model text only; banner, tool events and approval
  prompts go to stderr.
- `-p` reads a non-terminal stdin to EOF (gem-agent's contract).
- Sessions are JSONL transcripts. Record kinds keep gem-agent's names;
  `--continue` reads them.
- Usage records are written in the format gem-usage-lens reads (the four
  buckets `prompt` / `output` / `tool_prompt` / `total`; locally
  `tool_prompt` is always 0). As built, the record carries the lens's
  six fields — `thoughts` and `cached` beside those four — as
  `reference/architecture.md` lists. Cost is zero, so the comparison axes are
  tokens, wall-clock time, turns, and cache hits (inferred from time to
  first token).

### Configuration

`~/.config/lagent/config.toml`, strict decode (unknown keys are errors).
Precedence: flags > `LAGENT_*` environment > file > defaults.

```toml
[llm]
provider = "lmstudio"        # lmstudio | ollama | openai
base_url = "http://localhost:1234/v1"
model    = "google/gemma-4-26b-a4b-qat"
api_key  = ""                # optional; local servers need none

[model]
context_window = 0           # 0 = detect from the provider
```

`provider` only selects where the context length is detected (LM Studio:
`/api/v0/models`; Ollama: `/api/show`). Every conversation goes through the
OpenAI-compatible `chat/completions`. An explicit `context_window` removes
the provider dependency. As built, the file also carries `[sandbox]`,
`[agent]`, `[mcp]`, `[tui]` and `[approval]`; `reference/configuration.md`
lists every key.

### External Dependencies

- A local LLM server (LM Studio or Ollama). No credentials.
- macOS `sandbox-exec` (the same confinement as gem-agent).
- Go dependencies: stdlib + cobra + BurntSushi/toml + nlk. No OpenAI SDK.
  As built, the inline TUI ported under ADR-0001 brought Bubble Tea,
  bubbles, lipgloss and glamour with it.

## 3. Design Decisions

### Language and dependencies

Go. The porting sources, gem-agent and llm-cli, are Go, so sandbox / risk /
bounded / approve / session / mcp / tools / tui can be brought over
function by function. The OpenAI-compatible client is llm-cli's direct REST
approach (stdlib `net/http`, hand-written SSE). Per the organization's
supply-chain policy, community SDKs and wrappers are not adopted.

### Relationship to existing tools

- **gem-agent:** the control. The same task runs on both and the usage
  records are compared side by side.
- **llm-cli:** porting source for the OpenAI-compatible client.
- **gem-usage-lens:** reads both runtimes' usage records on one scale.

### Shape of the port

A new repository, not a fork. gem-agent's pure packages are brought in
individually as porting sources, with the source commit recorded. There is
no obligation to track gem-agent's later changes. The earlier fork
(`_wip/local-agent`, discarded 2026-09-09) failed in two classes: seams left
where upstream features were plugged with nil injection points, and
reference docs describing gem-agent's behaviour as current. lagent never
carries code or documents for features it does not ship.

### Prompt

Amended by ADR-0003, ADR-0004 and ADR-0005: the prompt now differs from
gem-agent's in the points those records list (the per-session facts
moved to the runtime's opening message, the MCP catalog sentence,
`view_image`); the measurement below predates them.

gem-agent's system prompt goes in unchanged as the first measurement
point. Measured: gem-agent's own prompt (about 1.2k tokens) + a 56KB
AGENTS.md + 15 built-in tools = 18,397 tokens, 7% of the 262k window.
Without the same prompt on both sides, a difference cannot be attributed
to the model or to the instructions. A local-oriented rewrite is decided by
ADR after the measurements.

### Auto-approval

Phase 1 is the rule tier plus human approval only. gem-agent's model tier
(risk review by a separate model call) becomes an extra call to the same
local model, costing seconds to tens of seconds each, so it is measured in
Phase 2 before adoption.

### Out of scope (explicit)

- web_search / web_fetch (Vertex grounding and URL context)
- GCS media upload, Cloud Logging telemetry
- thought signatures, safety settings
- Linux / Windows, GUI
- connecting several backends at once
- distribution and team use (experimental stage)

## 4. Development Plan

### Phase 1: Core

- OpenAI-compatible backend: streaming, assembling `tool_calls` deltas,
  `finish_reason=length` treated as a partial result (never discarded),
  resuming a transcript cut mid-turn
- agent loop, built-in tools, sandbox, approval gate, MCP client
- JSONL transcript with `--continue`, `-p`, TUI
- usage records (gem-usage-lens compatible)
- per-provider context-length detection (LM Studio / Ollama)
- tests for every item; independently reviewable.

**Exit criterion:** every E2E procedure already run on gem-agent (tool-call
round trip, approval flow, sandbox enforcement, a real MCP round trip,
AGENTS.md injection, both `-p` routes, TUI over a pty) passes on lagent,
and the usage records of the same task on both runtimes are laid side by
side in gem-usage-lens.

Measured outcome (2026-09-10): the E2E half passed. The side-by-side
half did not become a comparison: on the same instructions Gemma 4
answers in one round where Gemini works through a tool loop, so the
two transcripts record different work and their token counts cannot be
read against each other. The compatibility half of that criterion is
reduced to `gem-usage-lens verify --sessions-root <lagent sessions>`
reporting no checksum failure (11 transcripts, 37 records, 0 NG). The
finding itself — a local model that does not enter multi-step tool
work — is the effectiveness result Phase 2 measures against; the
cost comparison waits on it. `verify` reads transcripts without
opening the lens store; `ingest` does not, so lagent transcripts are
never pointed at the operator's live store.

### Phase 2: Features

Each item is adopted or rejected by ADR on the strength of Phase 1
measurements.

- history compaction (designed after measuring the KV-cache discard cost)
- the model tier of auto-approval
- skills, agent memory, hooks
- a local-oriented prompt revision
- Gemma 4's thinking toggle

### Phase 3: Release

- README.md / README.ja.md, CHANGELOG, AGENTS.md
- a health-check procedure (gem-agent's drill equivalent)
- signing and notarization, integration from `_wip` into lab-series

## 5. Required API Scopes / Permissions

None. No external services. LM Studio / Ollama are unauthenticated local
HTTP; `api_key` is an optional field for other OpenAI-compatible servers.
macOS sandbox-exec needs no extra entitlement.

## 6. Series Placement

Series: lab-series
Reason: experimental, single operator, purpose is measuring cost and
effectiveness. Follows the precedent of gem-agent, which started in
lab-series and was promoted to cli-series on production use. Promotion to
lite-series (local-first LLM tools) is reconsidered once practicality is
shown.

## 7. External Platform Constraints

Measured 2026-09-09 to 10 (LM Studio, Gemma 4 26B A4B QAT, MLX 4-bit,
Apple M2 Max 64GB).

- Prompt processing about 580 tok/s. A cache miss costs about 1.7 s per
  1k tokens to first token (36 s at 21k tokens). Generation about 50 tok/s.
- LM Studio's KV cache hits on an identical prefix (1 s) and is lost on
  history compaction or a parallel-slot switch. Design for an effective
  window of tens of thousands of tokens, not 262k.
- In streaming, tool-call arguments arrive as one chunk; silence while
  generating (the same property as Vertex's "one part").
- Verified working: parallel tool calls, the second round with role=tool,
  `response_format: json_schema`, base64 image input, tool selection with
  24 tool definitions and a 21k-token prompt, a 90-line write_file
  argument, exact-match edit_file strings.
- Unverified: Gemma 4's thinking toggle, behaviour at
  `finish_reason=length`, Ollama's tool calling and context detection,
  instruction-following of the nonce injection defence.

---

## Discussion Log

- **2026-09-09:** the idea of building a local-LLM runtime on gem-agent's
  technology. The API contract was measured live (LM Studio + Gemma 4 26B
  A4B QAT) and judged feasible. gem-agent's direct Vertex dependency is
  confined to four files; `llm.Backend` is one method.
- **2026-09-09:** the earlier fork `_wip/local-agent` was discarded for
  quality. Direction changed from a fork to a separate product line built
  new.
- **2026-09-10 item 1:** operator is the developer, experimental use.
  Motive: measuring cost and effectiveness.
- **Item 2:** Phase 1 limited to loop + built-in tools + sandbox + approval
  + MCP + transcript + `-p` + TUI. Auto-approval rule tier only. Usage
  records gem-usage-lens compatible (operator's decision).
- **Item 3:** Ollama selectable via config (operator's decision). Whether
  the prompt fits was measured first (operator's point): 18,397 tokens, 7%
  of the 262k window, so gem-agent's prompt goes in unchanged as the first
  measurement point. Tool name `lagent` (operator's decision).
- **Item 4:** Phase 1 exit criterion is passing all of gem-agent's E2E
  procedures plus a usage-record comparison.
- **Items 5 to 7:** no external services. lab-series (operator's
  decision). Constraints recorded from measurements.
- **Rejected alternatives:** adding multiple backends to gem-agent (touches
  a runtime under the cli-series stability contract), extracting a shared
  library (same), placement in lite-series (too early at the experimental
  stage).
