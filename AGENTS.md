# AGENTS.md — lagent

Sandboxed CLI agent runtime on a local LLM served over an
OpenAI-compatible API (LM Studio, Ollama). A separate product line from
gem-agent built on the same design; gem-agent is the porting source
(ADR-0001), and the features it has that lagent does not reproduce are
listed in ADR-0002. Experimental, lab-series, unreleased. The scaffold
answers `--version` and `version`; the agent loop is RFP Phase 1.

- **Module:** `github.com/nlink-jp/lagent`
- **Series:** lab-series (target; developed in `_wip/lagent` until integration)
- **Spec:** `docs/en/lagent-rfp.md` / `docs/ja/lagent-rfp.ja.md` (canonical)
- **Docs entry point:** `docs/en/INDEX.md` / `docs/ja/INDEX.ja.md`

## Build / test

| Task | Command |
|------|---------|
| Build | `make build` → `dist/lagent` (never `go build` directly) |
| Test | `make test` (or `go test ./...`) |
| Lint | `make lint` (golangci-lint, org config in `.golangci.yml`) |
| Vet + lint + test + docs mirror + release gate + build | `make check` |
| Docs mirror only | `make docs-check` |
| Release gate's own test | `make gate-check` |
| Release binary | `make build-all` (darwin/arm64 only; signed by `make package`) |
| Release archive | `make package` → `dist/lagent-vX.Y.Z-darwin-arm64.zip`, notarized |
| Release gate | `make verify-release` — refuses a zip with no notarisation marker, one rebuilt after its marker, one that does not unpack, or one whose binary does not run or reports another tag's version |

Version is injected via `-X main.version` from `git describe` — never edit the
`version` var default.

## Structure

```
config.example.toml  shipped config template ([llm] provider/base_url/model/api_key, [model] context_window)
main.go            entry point (package main, calls cmd.Execute(version))
cmd/               cobra root command; `version` subcommand mirrors --version
internal/          (empty at scaffold) Phase 1 lands the packages here, each
                   ported one names its gem-agent source commit in its doc comment
scripts/           codesign-darwin.sh / notarize-darwin.sh (org templates, verbatim),
                   docs-mirror-check.sh, verify-release-selftest.sh (make check)
docs/en/, docs/ja/ INDEX + reference/ + adr/ + the RFP (en: no suffix; ja: .ja.md)
```

## Gotchas

- **macOS-only by design** — isolation is built on sandbox-exec, as in
  gem-agent. Do not add linux/windows targets to the Makefile.
- **gem-agent is a source, not an upstream.** Bring a package over with
  its commit recorded; do not add a feature because gem-agent has it.
  Anything in ADR-0002's list stays out — code, config keys, error text
  and docs alike. The earlier fork failed exactly by keeping those seams.
- **`--version` must always answer** and `version` must print the same
  line (pinned by `cmd/root_test.go`) — a Homebrew formula's `brew test`
  runs it.
- **Prompt processing on a local model is the cost to design around**:
  measured about 580 tokens/s on the reference machine, so a cache miss
  on a 20k-token prefix is 30+ seconds of silence before the first token,
  while an identical prefix re-sent hits LM Studio's KV cache in about a
  second. Anything that rewrites history (compaction) discards the cache.
- **Tool-call arguments arrive as one chunk** in streaming; there is no
  liveness signal while a large `write_file` argument is generated. Do
  not add a stall timeout on that path.
- **`finish_reason=length` is a partial result** — surface it, never
  drop the turn (a fork-era defect).
- **A new config key means updating `config.example.toml`** — strict
  decode makes a stale template a startup error, so the loader tests parse
  the shipped template against the built-in defaults.
- **Docs are checked mechanically** — `make docs-check` fails on a missing
  en/ja mirror, an ADR absent from either INDEX, an INDEX link that does
  not resolve, or a backticked identifier (tool name, flag, config key)
  present in one language only.
