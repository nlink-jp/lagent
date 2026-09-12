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

## Procedure

Before you finish any edit in this project, read PROCEDURE.md in the
project root and do what it says. It applies to every edit, however
small.
