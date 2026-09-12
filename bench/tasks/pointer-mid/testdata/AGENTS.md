# AGENTS.md — ledgerd

Append-only ledger service with a REST face and a CLI. Go, stdlib plus
the org's shared packages. This file is the map for agents working in
the repository: build and test commands, the layout, and the things
that bite.

## Build / test

| Task | Command |
|------|---------|
| Build | `make build` → `dist/ledgerd` (never `go build` directly) |
| Test | `make test` (or `go test ./...`) |
| Lint | `make lint` |
| Everything | `make check` |

## ingest

The ingest package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

When ingest changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Anything ingest reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- `ingest` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- `ingest` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.

## journal

Logs from journal are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Errors from journal are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Anything journal reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- `journal` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- A `journal` retry without jitter synchronised twelve workers once;
  keep the jitter, and keep the test that measures spread.

## index

Logs from index are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

The index package owns one responsibility and exposes it through a small
interface; callers never reach into its internals. Its tests are table-
driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Errors from index are typed: a caller switches on the kind, never on the
message text. A new failure mode gets a new kind and a test that pins
the kind, and the message is for the operator's eyes only.

Gotchas:

- `index.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- `index` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.

## snapshot

Logs from snapshot are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

The snapshot package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Logs from snapshot are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Gotchas:

- `snapshot` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- `snapshot` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.

## replay

Logs from replay are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Logs from replay are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Errors from replay are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Gotchas:

- The `replay` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- The `replay` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## audit

Configuration for audit comes from the [audit] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Errors from audit are typed: a caller switches on the kind, never on the
message text. A new failure mode gets a new kind and a test that pins
the kind, and the message is for the operator's eyes only.

Anything audit reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- The `audit` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- `audit.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

## Procedure

Before you finish any edit in this project, read PROCEDURE.md in the
project root and do what it says. It applies to every edit, however
small.

## export

Configuration for export comes from the [export] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Metrics from export are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Configuration for export comes from the [export] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Gotchas:

- `export` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- The `export` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## quota

Anything quota reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Logs from quota are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Errors from quota are typed: a caller switches on the kind, never on the
message text. A new failure mode gets a new kind and a test that pins
the kind, and the message is for the operator's eyes only.

Gotchas:

- `quota.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- `quota.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

## tenant

The tenant package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

The tenant package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Metrics from tenant are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Gotchas:

- The `tenant` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- A `tenant` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.

## webhook

Logs from webhook are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Errors from webhook are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Configuration for webhook comes from the [webhook] section of
config.toml; unknown keys are startup errors. Defaults live in one
place, and the example config is parsed in a test so the template cannot
drift.

Gotchas:

- The `webhook` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- The `webhook` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## retention

Errors from retention are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Anything retention reads from disk goes through a bounded reader with
the cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

When retention changes its on-disk format, the version byte moves and
the reader keeps the old branch for two releases. Migration is forward-
only and idempotent; a half-applied migration must be safe to re-run.

Gotchas:

- A `retention` retry without jitter synchronised twelve workers once;
  keep the jitter, and keep the test that measures spread.
- `retention` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.

## search

Errors from search are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Anything search reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Anything search reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- `search.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- The `search` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## metrics

Errors from metrics are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Anything metrics reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

When metrics changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Gotchas:

- `metrics` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- `metrics` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.

