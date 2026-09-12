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

Errors from ingest are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Anything ingest reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

The ingest goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Gotchas:

- `ingest` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- `ingest` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.

## journal

Metrics from journal are counters and histograms with fixed label sets;
a label whose values are unbounded (a tenant id, a path) is a
cardinality bug and is rejected in review.

Metrics from journal are counters and histograms with fixed label sets;
a label whose values are unbounded (a tenant id, a path) is a
cardinality bug and is rejected in review.

Configuration for journal comes from the [journal] section of
config.toml; unknown keys are startup errors. Defaults live in one
place, and the example config is parsed in a test so the template cannot
drift.

Gotchas:

- `journal` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- `journal` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.

## index

Anything index reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Metrics from index are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Configuration for index comes from the [index] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Gotchas:

- A `index` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.
- `index` caches file handles per tenant; a test that opens many tenants
  must call Close or the descriptor limit trips on the CI runner.

## snapshot

The snapshot goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Logs from snapshot are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

When snapshot changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Gotchas:

- The `snapshot` fixture files are gzip-compressed; edit the source
  under fixtures/src and run `make fixtures`, never the .gz directly.
- `snapshot.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

## replay

Errors from replay are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

When replay changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Anything replay reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- `replay` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- A `replay` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.

## audit

Logs from audit are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Errors from audit are typed: a caller switches on the kind, never on the
message text. A new failure mode gets a new kind and a test that pins
the kind, and the message is for the operator's eyes only.

When audit changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Gotchas:

- A `audit` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.
- The `audit` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## export

Metrics from export are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

The export goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Logs from export are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Gotchas:

- `export.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- `export` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.

## quota

Configuration for quota comes from the [quota] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Anything quota reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

The quota package owns one responsibility and exposes it through a small
interface; callers never reach into its internals. Its tests are table-
driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Gotchas:

- The `quota` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- `quota` caches file handles per tenant; a test that opens many tenants
  must call Close or the descriptor limit trips on the CI runner.

## tenant

Logs from tenant are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Errors from tenant are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Anything tenant reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- `tenant` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- The `tenant` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## webhook

The webhook package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Errors from webhook are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

The webhook package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Gotchas:

- `webhook` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- The `webhook` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## retention

Metrics from retention are counters and histograms with fixed label
sets; a label whose values are unbounded (a tenant id, a path) is a
cardinality bug and is rejected in review.

The retention goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Errors from retention are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Gotchas:

- The `retention` fixture files are gzip-compressed; edit the source
  under fixtures/src and run `make fixtures`, never the .gz directly.
- `retention` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.

## search

Metrics from search are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Errors from search are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Logs from search are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Gotchas:

- `search.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- A `search` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.

## metrics

Anything metrics reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Anything metrics reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

The metrics goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Gotchas:

- The `metrics` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- A `metrics` retry without jitter synchronised twelve workers once;
  keep the jitter, and keep the test that measures spread.

## gc

The gc goroutines are started from one Run function and stopped through
the context; nothing detaches. A leak test counts goroutines before and
after, and it has caught two regressions.

Configuration for gc comes from the [gc] section of config.toml; unknown
keys are startup errors. Defaults live in one place, and the example
config is parsed in a test so the template cannot drift.

The gc goroutines are started from one Run function and stopped through
the context; nothing detaches. A leak test counts goroutines before and
after, and it has caught two regressions.

Gotchas:

- A `gc` retry without jitter synchronised twelve workers once; keep the
  jitter, and keep the test that measures spread.
- The `gc` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## schema

The schema goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Anything schema reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

The schema goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Gotchas:

- `schema` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- A `schema` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.

## migrate

The migrate goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Metrics from migrate are counters and histograms with fixed label sets;
a label whose values are unbounded (a tenant id, a path) is a
cardinality bug and is rejected in review.

Anything migrate reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- A `migrate` retry without jitter synchronised twelve workers once;
  keep the jitter, and keep the test that measures spread.
- `migrate` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.

## auth

Logs from auth are one line per event with key=value fields; the message
is stable and the fields carry the detail. Grep-ability wins over prose,
and a log line that appears on every request is a bug.

The auth goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Metrics from auth are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Gotchas:

- The `auth` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- `auth.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

## Procedure

Before you finish any edit in this project, read PROCEDURE.md in the
project root and do what it says. It applies to every edit, however
small.

## ratelimit

The ratelimit goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

The ratelimit goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Configuration for ratelimit comes from the [ratelimit] section of
config.toml; unknown keys are startup errors. Defaults live in one
place, and the example config is parsed in a test so the template cannot
drift.

Gotchas:

- `ratelimit` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- A `ratelimit` retry without jitter synchronised twelve workers once;
  keep the jitter, and keep the test that measures spread.

## backup

Configuration for backup comes from the [backup] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Errors from backup are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

Logs from backup are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Gotchas:

- A `backup` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.
- The `backup` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## verify

The verify package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Logs from verify are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Metrics from verify are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Gotchas:

- `verify` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- A `verify` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.

## ingest

The ingest goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

The ingest goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

The ingest package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Gotchas:

- A `ingest` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.
- The `ingest` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## journal

When journal changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

When journal changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

The journal package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Gotchas:

- `journal.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- `journal.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

## index

The index goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Anything index reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

The index package owns one responsibility and exposes it through a small
interface; callers never reach into its internals. Its tests are table-
driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Gotchas:

- `index` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- `index` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.

## snapshot

Anything snapshot reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

The snapshot goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

When snapshot changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Gotchas:

- A `snapshot` retry without jitter synchronised twelve workers once;
  keep the jitter, and keep the test that measures spread.
- `snapshot` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.

## replay

When replay changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Metrics from replay are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

Configuration for replay comes from the [replay] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Gotchas:

- A `replay` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.
- A `replay` retry without jitter synchronised twelve workers once; keep
  the jitter, and keep the test that measures spread.

## audit

Errors from audit are typed: a caller switches on the kind, never on the
message text. A new failure mode gets a new kind and a test that pins
the kind, and the message is for the operator's eyes only.

The audit goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

When audit changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Gotchas:

- The `audit` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.
- The `audit` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## export

Configuration for export comes from the [export] section of config.toml;
unknown keys are startup errors. Defaults live in one place, and the
example config is parsed in a test so the template cannot drift.

Errors from export are typed: a caller switches on the kind, never on
the message text. A new failure mode gets a new kind and a test that
pins the kind, and the message is for the operator's eyes only.

The export package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Gotchas:

- `export` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- `export.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

## quota

Metrics from quota are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

The quota goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

The quota package owns one responsibility and exposes it through a small
interface; callers never reach into its internals. Its tests are table-
driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Gotchas:

- `quota` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- `quota` caches file handles per tenant; a test that opens many tenants
  must call Close or the descriptor limit trips on the CI runner.

## tenant

Logs from tenant are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Metrics from tenant are counters and histograms with fixed label sets; a
label whose values are unbounded (a tenant id, a path) is a cardinality
bug and is rejected in review.

When tenant changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

Gotchas:

- `tenant` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- `tenant` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.

## webhook

Anything webhook reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

When webhook changes its on-disk format, the version byte moves and the
reader keeps the old branch for two releases. Migration is forward-only
and idempotent; a half-applied migration must be safe to re-run.

The webhook package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Gotchas:

- `webhook` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.
- `webhook` caches file handles per tenant; a test that opens many
  tenants must call Close or the descriptor limit trips on the CI
  runner.

## retention

Metrics from retention are counters and histograms with fixed label
sets; a label whose values are unbounded (a tenant id, a path) is a
cardinality bug and is rejected in review.

The retention goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

The retention package owns one responsibility and exposes it through a
small interface; callers never reach into its internals. Its tests are
table-driven and run without the network, and a fixture under testdata/
carries every shape the parser has met in production.

Gotchas:

- `retention.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.
- The `retention` fixture files are gzip-compressed; edit the source
  under fixtures/src and run `make fixtures`, never the .gz directly.

## search

The search goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Logs from search are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Anything search reads from disk goes through a bounded reader with the
cap stated in one constant; the cut is reported, never silent. A new
read path there needs the same treatment and a line in the architecture
test's allowlist.

Gotchas:

- `search` reads the clock through an injected Now; a test that uses
  time.Now directly is flaky under load and is rejected.
- `search.Run` refuses to start when the data directory is on a case-
  insensitive filesystem — the index relies on case; use a disk image
  locally.

## metrics

The metrics goroutines are started from one Run function and stopped
through the context; nothing detaches. A leak test counts goroutines
before and after, and it has caught two regressions.

Logs from metrics are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Logs from metrics are one line per event with key=value fields; the
message is stable and the fields carry the detail. Grep-ability wins
over prose, and a log line that appears on every request is a bug.

Gotchas:

- A `metrics` retry without jitter synchronised twelve workers once;
  keep the jitter, and keep the test that measures spread.
- The `metrics` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

## gc

The gc goroutines are started from one Run function and stopped through
the context; nothing detaches. A leak test counts goroutines before and
after, and it has caught two regressions.

Anything gc reads from disk goes through a bounded reader with the cap
stated in one constant; the cut is reported, never silent. A new read
path there needs the same treatment and a line in the architecture
test's allowlist.

Configuration for gc comes from the [gc] section of config.toml; unknown
keys are startup errors. Defaults live in one place, and the example
config is parsed in a test so the template cannot drift.

Gotchas:

- `gc` caches file handles per tenant; a test that opens many tenants
  must call Close or the descriptor limit trips on the CI runner.
- The `gc` fixture files are gzip-compressed; edit the source under
  fixtures/src and run `make fixtures`, never the .gz directly.

