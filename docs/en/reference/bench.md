# The task bench

How to measure the runtime on the fixed task set (ADR-0006). Evergreen:
updated in place as tasks and columns change.

## Run

```bash
make build bench-build                       # dist/lagent, dist/bench-mcp
go run ./bench run --bin dist/lagent --configs baseline --reps 3
go run ./bench report bench/results/<timestamp>
```

The local model server must be up at the `base_url` the configuration
names. A run of six tasks, three repetitions and one configuration is
eighteen one-shot sessions; budget minutes, not seconds (a cold prefix
is about two minutes on the reference machine).

| Flag | Meaning |
|---|---|
| `--bin` | the runtime binary to measure (required) |
| `--configs` | comma-separated configurations: `baseline` resolves to `bench/configs/<runtime>/baseline.toml`; `name=/path/config.toml` uses your own file |
| `--runtime` | `lagent` (default) or `gem-agent`: which state-root variable and config directory the binary honours |
| `--tasks` | task names to run (default: every directory under `bench/tasks`) |
| `--reps` | repetitions per task and configuration (default 3) |
| `--timeout` | deadline per run (default 10m); a run past it is recorded as timed out and the bench moves on |
| `--out` | results directory (default `bench/results/<timestamp>`, ignored by git) |
| `--mcp-bin` | the fixture MCP server for the `mcp-lookup` task (default `dist/bench-mcp`) |

Order: cases outside, configurations inside — for each task and
repetition, every configuration runs back to back, so the server's
state (cache, load) shifts hit all configurations alike. One line prints
per run as it lands.

## Isolation

Every run gets its own `HOME` (the configuration as `config.toml`, a
global `mcp.json` pointing at `bench-mcp` when the task needs it), its
own state root (`LAGENT_STATE_DIR` / `GEMAGENT_STATE_DIR`), and a fresh
copy of the task's `testdata/` as the project, placed under that `HOME`
so no instruction file above the results directory is read. Your
configuration, sessions and projects are never touched. Runs use `-p`
with `--auto`; the baseline configuration carries a `"never"` policy for
the fixture server so the MCP task needs no `--allow` (a grant would
preload the server and hide whether the model loads it).

## Tasks

| Task | Class | Completed when |
|---|---|---|
| `search-answer` | search then answer | the answer names `store.go:SaveConfig`; at least one tool call |
| `read-edit` | read then edit | `pager.go` has `i < n` and no `i <= n`; at least two tool calls |
| `multi-file-rename` | a change across files | three files carry `MaxEntries` and none `MaxItems`; at least three tool calls |
| `shell-count` | a shell command for a number | the answer contains `57`; at least one tool call |
| `mcp-lookup` | MCP lookup | the answer contains `Iceland`; at least two tool calls (`mcp_load`, then the lookup) |
| `view-image` | an image to look at | the answer contains `red`; at least one tool call |

A task is `bench/tasks/<name>/task.toml` (prompt, expectations, `mcp =
true` when it needs the fixture server) plus `testdata/`. Expectations
are `min_tool_calls`, `[[expect.file]]` (`path` with `contains` /
`not_contains`) and `[[expect.answer]]` (`regex`). A task the model
could answer without the files does not belong here.

## What is measured

Per run, read from the session transcript with the runtime's own
scanner: rounds (assistant messages carrying tool calls), tool calls by
name, prompt / output / cached tokens summed over the main-loop usage
records, wall time, exit code, the final answer, and the expectations
that failed. `runs.jsonl` holds one line per run; each run's directory
keeps its `stdout.txt`, `stderr.txt`, transcript, home and project.

`bench report` prints one Markdown row per task and configuration:
runs, completed, no-tool answers (the model replied without calling a
tool), and the medians of rounds, tool calls, prompt tokens and wall
seconds.

## Measurements

Baseline, 2026-09-12: lagent v0.1.0 (`acd6c4c`), LM Studio,
`google/gemma-4-26b-a4b-qat`, three repetitions, one configuration.

| task | config | runs | completed | no-tool answers | rounds (med) | calls (med) | prompt tok (med) | wall s (med) |
|---|---|---|---|---|---|---|---|---|
| mcp-lookup | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 12578 | 12 |
| multi-file-rename | baseline | 3 | 3/3 | 0/3 | 11 | 11 | 57408 | 30 |
| read-edit | baseline | 3 | 1/3 | 0/3 | 3 | 3 | 16271 | 10 |
| search-answer | baseline | 3 | 3/3 | 0/3 | 3 | 3 | 16239 | 10 |
| shell-count | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 11992 | 10 |
| view-image | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 12039 | 10 |

What it says:

- No run answered without a tool call. Every class entered multi-step
  work under the baseline prompt, including the MCP lookup (the model
  called `mcp_load` on its own in 3/3) and the image (3/3 `view_image`).
  The single-round behaviour Phase 1 observed is not reproduced by
  these tasks; the operator's sessions that showed it were screenshot
  and read-lane cases, both since addressed. The boundary, if there is
  one, lies in tasks larger than these.
- The one failing class is not a behaviour of the model's plan but of
  its output: in 2/3 `read-edit` runs the model returned an empty
  completion (finish reason `stop`, 3 output tokens, no text, no tool
  call) right after reading `pager.go`, and the one-shot run ended
  there with an error. The third run edited the file after nine
  rounds.
- The multi-file rename costs about 57k prompt tokens over eleven
  rounds — every round replays the history, so the prefix cache is
  what keeps it at 30 s.

`read-edit`, eight repetitions with the raw stream traced
(`LAGENT_LLM_TRACE`): 5/8 completed. Two runs ended on the empty
completion, and the trace showed the same bytes both times — one delta
with `reasoning_content` `<tool_call|>`, an empty delta, `finish_reason:
stop`, four completion tokens: a broken tool-call opener the server
routed into the reasoning channel (ADR-0007). One run explained the fix
in prose with a code block and never called `edit_file` — the model
narrating instead of acting, the shape the prompt revision measures
against.

`read-edit` after ADR-0007 (`4df8c4f`), eight repetitions: 8/8
completed, median 6 rounds, 19 s. No empty completion occurred in these
eight, so the retry itself did not fire in the field; its behaviour is
pinned by the unit test, and the field rate of the fault is 2/8 before
and 0/8 after on a sample that small.

Reference runtime, 2026-09-12: gem-agent v0.76.0 on Vertex AI Gemini,
one repetition, the operator's configuration minus its hooks and
telemetry, `read_only_auto` off, the fixture server on a `"never"`
policy.

| task | config | runs | completed | no-tool answers | rounds (med) | calls (med) | prompt tok (med) | wall s (med) |
|---|---|---|---|---|---|---|---|---|
| mcp-lookup | bench | 1 | 1/1 | 0/1 | 2 | 2 | 17570 | 23 |
| multi-file-rename | bench | 1 | 1/1 | 0/1 | 23 | 23 | 193341 | 141 |
| read-edit | bench | 1 | 1/1 | 0/1 | 15 | 15 | 119359 | 128 |
| search-answer | bench | 1 | 1/1 | 0/1 | 2 | 2 | 17427 | 23 |
| shell-count | bench | 1 | 1/1 | 0/1 | 4 | 4 | 29993 | 55 |
| view-image | bench | 1 | 1/1 | 0/1 | 1 | 1 | 12369 | 11 |

Every task completed on both runtimes. The reference does more
verification per task (twenty-three rounds for the rename, fifteen for
the edit, running the program before and after) and pays for it in
prompt tokens and wall time; the local model reaches the same
completion in a third of the rounds. The rounds a run takes are a
property of each model's habit, not of the runtime, and are not a
quality score.

## Comparing

A prompt revision, a thinking toggle or a tool-description change is
one more configuration file, or one more `--bin`, on the same tasks. The
reference runtime runs the same tasks with `--runtime gem-agent --bin
$(which gem-agent) --configs mine=/path/to/your/gem-agent.toml`; its
credentials stay in your file, never in this repository. Its
Application Default Credentials under `~/.config/gcloud` would vanish
with the isolated `HOME`, so the profile passes the file through as
`GOOGLE_APPLICATION_CREDENTIALS` when it exists.
