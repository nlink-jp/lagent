# ADR-0031: mlx-serve is a supported backend

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-10-07) — implemented; adopted on the measurements below |
| Date | 2026-10-07 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | The operator has run lagent against mlx-serve (`provider = "openai"`, a hand-set window) in daily use without trouble, faster than LM Studio by their account, and asked for it to be adopted as a backend |
| Relates to | [ADR-0006](0006-measurement-bench.md) (the task bench), [ADR-0007](0007-empty-completion-asked-again.md) (empty completions; amended here), [ADR-0009](0009-thinking-is-an-operator-key.md) (`reasoning_effort`), [ADR-0023](0023-promotion-to-cli-series.md) (the stability contract) |

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

## Decision

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
4. **A completion of only whitespace is empty** (amends ADR-0007).
   Found on the LM Studio side of the comparison: Qwen 3.6 answers there
   begin with the blank lines after its thinking, and one run's whole
   final answer was `\n\n`, which the empty check passed through as the
   answer.

## Verification

Measured 2026-10-07 on an M2 Max (64 GB), mlx-serve 26.10.1 and LM
Studio, Qwen 3.6 35B-A3B 4-bit. "Same weights" is literal: LM Studio's
`qwen/qwen3.6-35b-a3b` and mlx-serve's
`lmstudio-community/Qwen3.6-35B-A3B-MLX-4bit` are the same directory on
disk. "MTP" is mlx-serve's own build (`ddalcu/Qwen3.6-35B-A3B-MLX-Serve-4bit`)
with its multi-token-prediction head, the configuration daily use runs.
mlx-serve ran with the operator's settings (prefill partly offloaded to
the Neural Engine, prefix cache 2 GB in memory plus disk); LM Studio with
262k context and 4 parallel slots. One server was loaded at a time.

**The task bench** (seven tasks — the six of ADR-0006 and `skill-follow`
— three repetitions, thinking on via `reasoning_effort = "medium"`,
`bench/configs/lagent/{mlxserve-qwen36-mtp,mlxserve-qwen36,lmstudio-qwen36}.toml`):

| configuration | completed | wall time, 21 runs | the failures |
|---|---|---|---|
| mlx-serve, MTP | 19/21 | 294 s | `read-edit` 1 (fix described, not made), `view-image` 1 (answered with the untrusted-data tag name) |
| mlx-serve, same weights | 19/21 | 337 s | `read-edit` 1, `multi-file-rename` 1 (both: change described, not made) |
| LM Studio, same weights | 14/21 | 324 s | `skill-follow` 3 (the right line after two blank lines — the format check is anchored), `read-edit` 2, `multi-file-rename` 2 (one answer was only `\n\n`) |

Repeated on the MTP configuration, `read-edit` completed 5/6 and
`view-image` 6/6: the tag-name answer is 1 in 9 and has not recurred.
"Described, not made" is the class ADR-0008 measured on Gemma 4; it is a
model behaviour, seen under both servers. No run on mlx-serve returned
an empty completion, a malformed tool call or an argument error.

**The server** (`bench serve`, medians of two; prompt tokens as
reported — the size classes 1k/12k/35k/69k came to 966, 11,185, 31,694
and 60,333):

| | mlx-serve, MTP | mlx-serve, same weights | LM Studio, same weights |
|---|---|---|---|
| cold read, 11k | 11.5 s | 11.4 s | 15.5 s |
| cold read, 32k | 46.0 s | 45.4 s | 49.0 s |
| cold read, 60k | 124.4 s | 122.9 s | 110.6 s |
| resend, 60k | 0.9 s | 0.8 s | 1.5 s |
| next turn, 60k | 1.3 s | 1.2 s | 1.5 s |
| decode, 1k | 158 tok/s | 106 tok/s | 85 tok/s |
| decode, 11k | 110 tok/s | 93 tok/s | 81 tok/s |
| decode, 60k | 65 tok/s | 61 tok/s | 57 tok/s |
| two streams at once, 1k (sum) | 168 tok/s | 115 tok/s | 125 tok/s |
| back to the first of five 32k conversations | 27.7 s (16k reused) | 27.6 s (16k reused) | 1.1 s |

- **The runtime itself is not where the speed comes from.** Under the same
  weights, mlx-serve reads prompts up to ~32k faster and LM Studio reads
  60k faster; generation is 7–25 % faster under mlx-serve. The large gap
  is the MTP head of mlx-serve's own build: 1.9× LM Studio's generation
  at 1k, 1.4× at 11k, converging at 60k (1.1×).
- **Long context holds.** A cold 122,019-token prompt completed on
  mlx-serve (414 s, decode 39 tok/s): the GPU-watchdog stop that ruled
  out Magnitude (2026-10-04) did not occur.
- **The memory cache is a setting, and its default is too small for
  lagent.** With the default 2 GB (the app's "Auto" — the larger budget it
  describes for long-context hybrid models applies to one architecture
  only), five 32k conversations do not fit and a 122k one does not fit
  once: a resend reused 81,920 of 122,019 tokens (207 s), the next turn
  98,304 (131 s), and going back to the first of five conversations took
  27.7 s. Qwen 3.6 keeps about 26 KB a token (attention KV on 10 of 40
  layers plus the linear-attention state), so 122k is about 3.1 GB. The
  same server with `--prefix-cache-mem 8GB` (the app's "Prefix cache
  memory cap") — one run, everything else the operator's — resent the
  122k prompt in 1.2 s, took the next turn in 2.1 s, and went back to
  each of the five conversations in 0.4–0.5 s, faster than LM Studio's
  1.1 s; the cache peaked at 5.0 GB.
- mlx-serve reports `cached_tokens`; LM Studio does not, so its records
  show 0 cached although it reuses the prefix.

## Residual risk

lagent cannot settle these; the operator who runs the server takes them on.

1. **The server is community software.** One author, weekly releases,
   and an open report of a vision regression in the current one (#753). The app checks for its
   own updates and for updates to the model packs it downloaded, and its
   own builds of models (`ddalcu/…`) are converted by the same author.
   The weights LM Studio serves here are converted by a third party too
   (`lmstudio-community`), so this is a difference of degree, not kind.
2. **26.10.1 listens on every interface with no key by default** (its
   help says a later version will default to `127.0.0.1`). The API
   includes `/v1/load-model` by absolute path and `/v1/unload-model`.
   The operator binds it to `127.0.0.1` or sets its API key; on the
   reference machine it was open to the LAN until this verification
   said so.
3. **The server rewrites tool-call arguments** to the types the schema
   declares (`toolAutocorrect`, on by default). It parses Qwen's
   tag-format calls into JSON as LM Studio's parser does; what lagent
   receives is the server's reading of the model, on either server.
4. **A record's output tokens include the thinking** (no
   `reasoning_tokens`), so the same work records more output under
   mlx-serve than under LM Studio.
5. **The prefix cache's default budget re-reads long sessions.** Below
   2 GB of cache — about 80k tokens of Qwen 3.6 — nothing changes; past
   it, every turn re-reads tens of thousands of tokens. The operator sets
   the memory cap (8 GB measured) where the RAM allows; lagent cannot.
6. **A mistyped model id runs against the loaded model.** The provider
   reports it at startup as a notice; the session still starts, as it
   does when any provider's window lookup fails.

## References

- `internal/llm/openai.go` — `mlxServeWindow`
- `bench/serve.go` — the server measurement; `bench/_results/serve-*`
  and `tasks-*` hold the raw logs (not in git)
- `internal/agent/agent.go` — the empty check (decision 4)
