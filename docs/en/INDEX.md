# Documentation Index

Entry point for lagent's maintainer-facing documentation. For
user-facing material see [`README.md`](../../README.md).

Japanese mirror: [`INDEX.ja.md`](../ja/INDEX.ja.md). `scripts/docs-mirror-check.sh`
enforces the structural half in `make check` — every `docs/en` file has its
`docs/ja` counterpart and back, the ADR catalogue is complete and ordered in
both, and the identifiers of each pair agree. Prose parity is the author's job.

## Specification

- [`lagent-rfp.md`](lagent-rfp.md) — the canonical spec: problem
  statement, functional surface, scope boundaries, phase plan. Features
  outside it need an ADR.

## Reference

Current behaviour, updated in place as the code changes.

- [`reference/configuration.md`](reference/configuration.md) — install,
  the config file, precedence, the command and flag tables
- [`reference/architecture.md`](reference/architecture.md) — package
  layout, the backend, the turn loop, approval, the round ladder,
  persistence, and what is not here

Feature references (interface, tools, approval, sessions, integration)
follow as the Phase 1 measurements settle the surface.

## ADRs

Point-in-time design decisions. Immutable once accepted; a changed
decision gets a new ADR that supersedes the old one (typo and link fixes
excepted).

- [`ADR-0001`](adr/0001-porting-sources-pinned.md) — gem-agent and
  llm-cli are porting sources, pinned by commit: a new repository, not a
  fork; packages come over one at a time with their source recorded, and
  nothing tracks the sources afterwards
- [`ADR-0002`](adr/0002-features-not-reproduced.md) — the gem-agent
  features lagent does not reproduce, why each is bound to Vertex AI or
  Google Cloud, and what lagent does instead
