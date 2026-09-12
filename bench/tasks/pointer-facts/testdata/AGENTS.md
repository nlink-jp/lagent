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

Anything ingest reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

The ingest package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Anything ingest reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- `ingest` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- A `ingest` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.

## journal

When journal changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Anything journal reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Anything journal reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- `journal` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- `journal.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

## index

Configuration for index comes from the [index] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Logs from index are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Errors from index are typed: a caller switches on the kind, never on the
message text. A new failure mode gets a new kind and a test that pins
the kind, and the message is for the operator's eyes only.

Gotchas:

- A `index` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.
- The `index` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## snapshot

Configuration for snapshot comes from the [snapshot] section of
config.toml; unknown keys are startup errors. Defaults live in one
place, and the example config is parsed in a test so the template cannot
drift.

The snapshot package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

The snapshot goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Gotchas:

- A `snapshot` retry without jitter synchronised twelve workers once;
  keep the jitter, and keep the test that measures spread.
- `snapshot.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

## replay

The replay package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Logs from replay are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Metrics from replay are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Gotchas:

- `replay.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- A `replay` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.

## audit

Errors from audit are typed: a caller switches on the kind, never on the
message text. A new failure mode gets a new kind and a test that pins
the kind, and the message is for the operator's eyes only.

The audit package owns one responsibility and exposes it through a small
interface; callers never reach into its internals. Its tests are table-
driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

The audit goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Gotchas:

- The `audit` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- `audit` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.

## export

The export goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Metrics from export are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Anything export reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- `export.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- A `export` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.

## quota

The quota package owns one responsibility and exposes it through a small
interface; callers never reach into its internals. Its tests are table-
driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

The quota goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Logs from quota are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Gotchas:

- `quota` caches file handles per tenant; a test that opens many tenants
  must call Close or the descriptor limit trips on the CI runner.
- The `quota` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## tenant

Configuration for tenant comes from the [tenant] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

When tenant changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Errors from tenant are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Gotchas:

- `tenant` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- The `tenant` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## webhook

Errors from webhook are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Errors from webhook are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Logs from webhook are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Gotchas:

- `webhook` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- A `webhook` retry without jitter synchronised twelve workers once;
  keep the jitter, and keep the test that measures spread.

## retention

When retention changes its on-disk format, the version byte moves and
the reader keeps the old branch for two releases. Migration is forward-
only and idempotent; a half-applied migration must be safe to re-run.

Configuration for retention comes from the [retention] section of
config.toml; unknown keys are startup errors. Defaults live in one
place, and the example config is parsed in a test so the template cannot
drift.

Metrics from retention are counters and histograms with fixed label
sets; a label whose values are unbounded (a tenant id, a path) is a
cardinality bug and is rejected in review.

Gotchas:

- `retention` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- The `retention` fixture files are gzip-compressed; edit the source
  under fixtures/src and run `make fixtures`, never the .gz directly.

## search

Configuration for search comes from the [search] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Metrics from search are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Anything search reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- A `search` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.
- The `search` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## metrics

Metrics from metrics are counters and histograms with fixed label sets;
a label whose values are unbounded (a tenant id, a path) is a
cardinality bug and is rejected in review.

Logs from metrics are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

The metrics goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Gotchas:

- `metrics` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- The `metrics` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## gc

Logs from gc are one line per event with key=value fields; the message
is stable and the fields carry the detail. Grep-ability wins over prose,
and a log line that appears on every request is a bug.

Anything gc reads from disk goes through a bounded reader with the cap
stated in one constant; the cut is reported, never silent. A new read
path there needs the same treatment and a line in the architecture
test's allowlist.

The gc goroutines are started from one Run function and stopped through
the context; nothing detaches. A leak test counts goroutines before and
after, and it has caught two regressions.

Gotchas:

- A `gc` retry without jitter synchronised twelve workers once; keep the
  jitter, and keep the test that measures spread.
- The `gc` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## schema

The schema package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

The schema goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Errors from schema are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Gotchas:

- The `schema` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- `schema` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.

## migrate

The migrate goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Errors from migrate are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Configuration for migrate comes from the [migrate] section of
config.toml; unknown keys are startup errors. Defaults live in one
place, and the example config is parsed in a test so the template cannot
drift.

Gotchas:

- `migrate.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- The `migrate` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## auth

Configuration for auth comes from the [auth] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Errors from auth are typed: a caller switches on the kind, never on the
message text. A new failure mode gets a new kind and a test that pins
the kind, and the message is for the operator's eyes only.

Configuration for auth comes from the [auth] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Gotchas:

- `auth.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- `auth` caches file handles per tenant; a test that opens many tenants
  must call Close or the descriptor limit trips on the CI runner.

## ratelimit

Logs from ratelimit are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Errors from ratelimit are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

When ratelimit changes its on-disk format, the version byte moves and
the reader keeps the old branch for two releases. Migration is forward-
only and idempotent; a half-applied migration must be safe to re-run.

Gotchas:

- `ratelimit` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- The `ratelimit` fixture files are gzip-compressed; edit the source
  under fixtures/src and run `make fixtures`, never the .gz directly.

## backup

When backup changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Logs from backup are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Configuration for backup comes from the [backup] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Gotchas:

- `backup.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- A `backup` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.

## verify

Metrics from verify are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Anything verify reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Configuration for verify comes from the [verify] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Gotchas:

- The `verify` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- `verify.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

## ingest

Metrics from ingest are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

The ingest package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Configuration for ingest comes from the [ingest] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Gotchas:

- The `ingest` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- `ingest.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

## journal

When journal changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Anything journal reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

When journal changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Gotchas:

- The `journal` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- `journal` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.

## index

The index goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Anything index reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Metrics from index are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Gotchas:

- `index.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- The `index` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## snapshot

Metrics from snapshot are counters and histograms with fixed label sets;
a label whose values are unbounded (a tenant id, a path) is a
cardinality bug and is rejected in review.

Anything snapshot reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Errors from snapshot are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Gotchas:

- `snapshot` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- A `snapshot` retry without jitter synchronised twelve workers once;
  keep the jitter, and keep the test that measures spread.

## replay

Anything replay reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Errors from replay are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Metrics from replay are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Gotchas:

- `replay` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- A `replay` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.

## audit

Configuration for audit comes from the [audit] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Metrics from audit are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

The audit package owns one responsibility and exposes it through a small
interface; callers never reach into its internals. Its tests are table-
driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Gotchas:

- A `audit` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.
- The `audit` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## export

The export package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Errors from export are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

When export changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Gotchas:

- `export.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- `export` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.

## quota

The quota package owns one responsibility and exposes it through a small
interface; callers never reach into its internals. Its tests are table-
driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

When quota changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Metrics from quota are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Gotchas:

- `quota.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- The `quota` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## tenant

Logs from tenant are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Anything tenant reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

When tenant changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Gotchas:

- `tenant` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- A `tenant` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.

## webhook

The webhook goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

The webhook goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Errors from webhook are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Gotchas:

- `webhook` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- `webhook.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

## retention

The retention package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Errors from retention are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Anything retention reads from disk goes through a bounded reader with
the cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- The `retention` fixture files are gzip-compressed; edit the source
  under fixtures/src and run `make fixtures`, never the .gz directly.
- The `retention` fixture files are gzip-compressed; edit the source
  under fixtures/src and run `make fixtures`, never the .gz directly.

## search

The search package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Metrics from search are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Configuration for search comes from the [search] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Gotchas:

- The `search` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- `search` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.

## metrics

Errors from metrics are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Errors from metrics are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Anything metrics reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- A `metrics` retry without jitter synchronised twelve workers once;
  keep the jitter, and keep the test that measures spread.
- `metrics` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.

## gc

Errors from gc are typed: a caller switches on the kind, never on the
message text. A new failure mode gets a new kind and a test that pins
the kind, and the message is for the operator's eyes only.

Metrics from gc are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Anything gc reads from disk goes through a bounded reader with the cap
stated in one constant; the cut is reported, never silent. A new read
path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- `gc.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- `gc.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

