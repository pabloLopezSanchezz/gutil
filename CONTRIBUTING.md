# Contributing

## Local checks

Install Go and Git, then run:

```sh
test -z "$(gofmt -l .)"
go vet ./...
go test -race ./...
go test ./... -count=3
```

Integration tests use temporary local repositories and do not require network access.

## Adding a subcommand

Keep parsing and orchestration in a focused package under `internal/commands`. Put external process execution behind `internal/process.Runner`, and reusable Git operations in `internal/git`. Add a failing behavior test before production code, then add a real integration test for stateful Git behavior.

All code, terminal messages, errors, tests, commits, and documentation must be in English.
