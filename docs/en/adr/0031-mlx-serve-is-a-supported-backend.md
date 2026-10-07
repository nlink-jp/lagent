# ADR-0031: mlx-serve is a supported backend

| Field | Value |
|-------|-------|
| Status | **Proposed** (2026-10-07) — the provider is implemented; the measurements that decide adoption are in progress |
| Date | 2026-10-07 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The operator has run lagent against mlx-serve (`provider = "openai"`, a hand-set window) in daily use without trouble, faster than LM Studio by their account, and asked for it to be adopted as a backend |
| Relates to | [ADR-0006](0006-measurement-bench.md) (the task bench), [ADR-0007](0007-empty-completion-asked-again.md) (empty completions), [ADR-0009](0009-thinking-is-an-operator-key.md) (`reasoning_effort`) |

## Context

[mlx-serve](https://github.com/ddalcu/mlx-serve) is an MLX inference
server for Apple silicon, written in Zig against mlx-c, distributed as a
Developer ID–signed, notarized app (MIT). It speaks the OpenAI-compatible
`chat/completions` API at `http://localhost:11234/v1` by default. It is
written by one person outside this organization.

**Supply chain.** The organization does not adopt community code (org
ADR-024). lagent takes none: it reaches the server over the same HTTP
contract it uses for LM Studio, with its own stdlib client, and ships
nothing of mlx-serve's. The server is something the operator chooses to
run, as LM Studio is. What the operator takes on by running it — the
app, its update channel, the weights it serves — is recorded below as
residual risk rather than settled by lagent.

**What the wire carries (measured, mlx-serve 26.10.1, Qwen 3.6
35B-A3B):**

- Tool calls stream as one delta per call with the whole arguments,
  parallel calls as separate indices, then `finish_reason: tool_calls` —
  LM Studio's shape, which the accumulator already takes.
- Usage arrives on the stream (`stream_options.include_usage`) with
  `prompt_tokens_details.cached_tokens`; a `timings` object rides beside
  it. `completion_tokens_details.reasoning_tokens` is **not** sent, so a
  record's output tokens include the model's thinking.
- Thinking streams as `reasoning_content`. `reasoning_effort` is not
  validated: `none` turns thinking off and **every other value, `off`
  and unknown words included, leaves it on**. With no value sent, Qwen
  3.6 thinks.
- `max_tokens` ends a turn with `finish_reason: length`, during thinking
  too. An omitted `max_tokens` does not cap the answer (1,491 tokens
  measured), whatever the server's `--max-tokens` help text implies.
- **A model name the server does not have is answered by the model it
  has loaded**, with status 200. A mistyped `[llm].model` would run
  silently.
- `/v1/models` lists every model it can serve, with `context_length`.
- Images as `image_url` data URLs (PNG, JPEG) are read with this model.
  An upstream report (ddalcu/mlx-serve#753) says 26.10.1 broke vision for
  Gemma 4 and Qwen 3.5-VL.

## Decision (proposed)

1. **`[llm].provider = "mlxserve"`.** The context window is read from
   the model's entry in `/v1/models`. A model the list does not carry is
   an error naming the cause, since this lookup is the only place a
   mistyped id can surface (implemented, `fbb786a`).
2. **The default stays `lmstudio`.** Changing it would break every
   configuration that relies on the default, which the cli-series
   stability contract (ADR-0023) forbids outside the breaking-change
   process.
3. **Adoption is decided by measurement, not by the account of daily
   use:** the task bench (ADR-0006) and a server measurement
   (`bench serve`: cold prompt reading at 1k–69k, reuse on resend and on
   the next turn, decode speed, alternating conversations, two streams),
   on **the same weights under both servers** so the runtime difference
   is not confounded with the model, and on mlx-serve's own build of the
   model (with its MTP head) as daily use runs it.

## Verification

To be recorded here with the numbers.

## Residual risk

To be recorded here.

## References

- `internal/llm/openai.go` — `mlxServeWindow`
- `bench/serve.go` — the server measurement
